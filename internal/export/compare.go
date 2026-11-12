package export

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"server/internal/clientip"
	"server/internal/contract"
	"server/internal/diff"
	"server/internal/events"
	"server/internal/jsonutil"
	"server/internal/safelog"
)

const (
	MaxComparisonBytes   = 2*MaxContractBytes + 1024
	MsgTooLargeToCompare = "this plan is larger than the comparison supports"
	MsgNotAComparison    = "send the previous and the current plan"
)

type comparison struct {
	Previous json.RawMessage `json:"previous"`
	Current  json.RawMessage `json:"current"`
}

func (h *Handler) Compare(w http.ResponseWriter, r *http.Request) {
	key := "compare-ip:" + clientip.From(r)

	if !h.limiter.Take(key) {
		h.logger.Warn("compare throttled", "limit", safelog.Key(key))
		jsonutil.TooManyRequestsResponse(w, "too many comparisons, try again shortly")
		return
	}

	if r.ContentLength > MaxComparisonBytes {
		jsonutil.ContentTooLargeError(w)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxComparisonBytes))
	if err != nil {
		jsonutil.BadRequestResponse(w, "plans are too large or could not be read")
		return
	}

	var req comparison
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&req); err != nil || len(req.Previous) == 0 || len(req.Current) == 0 {
		jsonutil.BadRequestResponse(w, MsgNotAComparison)
		return
	}

	previous, ok := h.plan(w, req.Previous)
	if !ok {
		return
	}
	current, ok := h.plan(w, req.Current)
	if !ok {
		return
	}

	report := diff.Compare(previous, current)

	if err = jsonutil.WriteJSON(w, http.StatusOK, report, nil); err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	h.counts.Count(events.CompareRun)
}

func (h *Handler) plan(w http.ResponseWriter, raw json.RawMessage) (*contract.Contract, bool) {
	c, err := contract.Decode(raw)
	if err == nil {
		return c, true
	}

	h.logger.Warn("compare rejected", "err", err)
	if errors.Is(err, contract.ErrLimit) {
		jsonutil.BadRequestResponse(w, MsgTooLargeToCompare)
		return nil, false
	}
	jsonutil.BadRequestResponse(w, MsgNotAPlan)
	return nil, false
}
