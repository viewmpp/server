package upload

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"server/internal/jsonutil"
	"server/internal/parser"
	"server/internal/ratelimit"
	"server/internal/user"
	"strconv"
)

type Handler struct {
	store   uploadStore
	client  *parser.Client
	limiter *ratelimit.Limiter
	logger  *slog.Logger
}

type uploadStore interface {
	Save(ctx context.Context, userID int64, fileName string, contract []byte) (string, error)
}

func NewHandler(
	client *parser.Client,
	store uploadStore,
	limiter *ratelimit.Limiter,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		store:   store,
		client:  client,
		limiter: limiter,
		logger:  logger,
	}
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	u := user.GetUserContext(r)

	if !h.allow(r, u) {
		jsonutil.TooManyRequestsResponse(w)
		return
	}

	limit := u.MaxUploadBytes()

	if r.ContentLength < 0 {
		jsonutil.LengthRequiredResponse(w)
		return
	}

	if r.ContentLength > limit {
		jsonutil.ContentTooLargeError(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, limit)

	contract, err := h.client.Parse(r.Context(), r.Body, r.ContentLength)
	if err != nil {
		pe, exists := errors.AsType[*parser.ParseError](err)
		if exists && pe.Status < 500 {
			jsonutil.BadRequestResponse(w, pe.Message)
			return
		}
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if !u.IsAnonymous() {
		publicID, err := h.store.Save(r.Context(), u.ID, r.URL.Query().Get("name"), contract)
		if err != nil {
			jsonutil.ServerErrorResponse(w, r, err, h.logger)
			return
		}
		w.Header().Set("X-Project-Id", publicID)
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(contract)
}

func (h *Handler) allow(r *http.Request, u *user.User) bool {
	keys := []string{"upload-ip:" + h.limiter.ClientIP(r)}

	if !u.IsAnonymous() {
		keys = append(keys, "upload-user:"+strconv.FormatInt(u.ID, 10))
	}

	key, allowed := h.limiter.TakeAll(keys)
	if !allowed {
		h.logger.Warn("upload throttled", "key", key)
		return false
	}

	return true
}
