package export

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"server/internal/clientip"
	"server/internal/contract"
	"server/internal/jsonutil"
	"server/internal/ratelimit"
	"server/internal/safelog"
	"server/internal/xlsx"
	"strconv"
)

const MaxContractBytes = 8 << 20

type Handler struct {
	limiter *ratelimit.Limiter
	logger  *slog.Logger
}

func NewHandler(limiter *ratelimit.Limiter, logger *slog.Logger) *Handler {
	return &Handler{
		limiter: limiter,
		logger:  logger,
	}
}

func (h *Handler) XLSX(w http.ResponseWriter, r *http.Request) {
	key := "export-ip:" + clientip.From(r)

	if !h.limiter.Take(key) {
		h.logger.Warn("export throttled", "limit", safelog.Key(key))
		jsonutil.TooManyRequestsResponse(w, "too many uploads, try again shortly")
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
		jsonutil.BadRequestResponse(w, "that is not a valid plan")
		return
	}

	var buf bytes.Buffer
	if err = xlsx.Write(&buf, c); err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", xlsx.Disposition(r.URL.Query().Get("name")))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	_, _ = buf.WriteTo(w)
}
