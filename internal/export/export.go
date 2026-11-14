package export

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"server/internal/clientip"
	"server/internal/contract"
	"server/internal/events"
	"server/internal/jsonutil"
	"server/internal/lookahead"
	"server/internal/ratelimit"
	"server/internal/safelog"
	"server/internal/xlsx"
	"strconv"
)

const MaxContractBytes = contract.MaxBytes

const (
	MsgNotAPlan          = "that is not a valid plan"
	MsgTooLargeToConvert = "this plan is larger than the converter supports"
)

type counter interface {
	Count(name events.Name)
}

type Handler struct {
	limiter *ratelimit.Limiter
	counts  counter
	logger  *slog.Logger
}

func NewHandler(limiter *ratelimit.Limiter, counts counter, logger *slog.Logger) *Handler {
	return &Handler{
		limiter: limiter,
		counts:  counts,
		logger:  logger,
	}
}

func (h *Handler) XLSX(w http.ResponseWriter, r *http.Request) {
	key := "export-ip:" + clientip.From(r)

	if !h.limiter.Take(key) {
		h.logger.Warn("export throttled", "limit", safelog.Key(key))
		jsonutil.TooManyRequestsResponse(w, "too many exports, try again shortly")
		return
	}

	if r.ContentLength > MaxContractBytes {
		jsonutil.ContentTooLargeError(w)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxContractBytes))
	if err != nil {
		jsonutil.BadRequestResponse(w, "contract is too large or could not be read")
		return
	}

	c, err := contract.Decode(body)
	if err != nil {
		h.logger.Warn("export rejected", "err", err)
		if errors.Is(err, contract.ErrLimit) {
			h.counts.Count(events.ExportTooLarge)
			jsonutil.BadRequestResponse(w, MsgTooLargeToConvert)
			return
		}
		jsonutil.BadRequestResponse(w, MsgNotAPlan)
		return
	}

	query := r.URL.Query()
	view := events.ExportXLSX
	name := query.Get("name")

	var buf bytes.Buffer
	if query.Has("lookahead_from") || query.Has("lookahead_weeks") {
		window, err := lookahead.Parse(query.Get("lookahead_from"), query.Get("lookahead_weeks"))
		if err != nil {
			jsonutil.BadRequestResponse(w, err.Error())
			return
		}
		view = events.ExportLookahead
		name = window.FileName(name)
		err = xlsx.WriteLookahead(&buf, c, window)
		if err != nil {
			jsonutil.ServerErrorResponse(w, r, err, h.logger)
			return
		}
	} else if err = xlsx.Write(&buf, c); err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", xlsx.Disposition(name))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	h.counts.Count(view)

	_, _ = buf.WriteTo(w)
}
