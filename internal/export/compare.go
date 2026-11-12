package export

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"server/internal/clientip"
	"server/internal/contract"
	"server/internal/diff"
	"server/internal/events"
	"server/internal/jsonutil"
	"server/internal/safelog"
	"server/internal/user"
	"server/internal/xlsx"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MaxComparisonBytes   = 2*MaxContractBytes + 1024
	MsgTooLargeToCompare = "this plan is larger than the comparison supports"
	MsgNotAComparison    = "send the previous and the current plan"
	MsgChangesArePro     = "the change report is part of Pro"
	maxFileNameRunes     = 255
)

type comparison struct {
	Previous json.RawMessage `json:"previous"`
	Current  json.RawMessage `json:"current"`
}

func (h *Handler) Compare(w http.ResponseWriter, r *http.Request) {
	previous, current, ok := h.readComparison(w, r, "compare-ip:")
	if !ok {
		return
	}

	report := diff.Compare(previous, current)

	if err := jsonutil.WriteJSON(w, http.StatusOK, report, nil); err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	h.counts.Count(events.CompareRun)
}

func (h *Handler) CompareXLSX(w http.ResponseWriter, r *http.Request) {
	if !user.GetUserContext(r).HasSubscription() {
		jsonutil.ForbiddenResponse(w, MsgChangesArePro)
		return
	}

	previous, current, ok := h.readComparison(w, r, "compare-xlsx-ip:")
	if !ok {
		return
	}

	previousName := fileName(r.URL.Query().Get("previous_name"))
	currentName := fileName(r.URL.Query().Get("current_name"))

	var buf bytes.Buffer
	if err := xlsx.WriteChanges(&buf, diff.Compare(previous, current), previousName, currentName); err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	stem := strings.TrimSuffix(currentName, filepath.Ext(currentName))
	if stem == "" {
		stem = "plan"
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", xlsx.Disposition(stem+" changes"))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	h.counts.Count(events.CompareExport)

	_, _ = buf.WriteTo(w)
}

func (h *Handler) readComparison(w http.ResponseWriter, r *http.Request, prefix string) (*contract.Contract, *contract.Contract, bool) {
	key := prefix + clientip.From(r)

	if !h.limiter.Take(key) {
		h.logger.Warn("compare throttled", "limit", safelog.Key(key))
		jsonutil.TooManyRequestsResponse(w, "too many comparisons, try again shortly")
		return nil, nil, false
	}

	if r.ContentLength > MaxComparisonBytes {
		jsonutil.ContentTooLargeError(w)
		return nil, nil, false
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxComparisonBytes))
	if err != nil {
		jsonutil.BadRequestResponse(w, "plans are too large or could not be read")
		return nil, nil, false
	}

	var req comparison
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&req); err != nil || len(req.Previous) == 0 || len(req.Current) == 0 {
		jsonutil.BadRequestResponse(w, MsgNotAComparison)
		return nil, nil, false
	}

	previous, ok := h.plan(w, req.Previous)
	if !ok {
		return nil, nil, false
	}
	current, ok := h.plan(w, req.Current)
	if !ok {
		return nil, nil, false
	}

	return previous, current, true
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

func fileName(raw string) string {
	name := strings.TrimSpace(raw)
	if utf8.RuneCountInString(name) <= maxFileNameRunes {
		return name
	}
	return string([]rune(name)[:maxFileNameRunes])
}
