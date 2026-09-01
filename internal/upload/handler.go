package upload

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"server/internal/clientip"
	"server/internal/contract"
	"server/internal/jsonutil"
	"server/internal/parser"
	"server/internal/ratelimit"
	"server/internal/safelog"
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
		jsonutil.TooManyRequestsResponse(w, "too many uploads, try again shortly")
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

	plan, err := h.client.Parse(r.Context(), r.Body, r.ContentLength)
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
		if !storable(plan, h.logger) {
			w.Header().Set("X-Save-Refused", "unreadable")
		} else {
			publicID, err := h.store.Save(r.Context(), u.ID, r.URL.Query().Get("name"), plan)
			switch {
			case errors.Is(err, user.ErrSaveLimit):
				w.Header().Set("X-Save-Refused", "limit")
			case err != nil:
				jsonutil.ServerErrorResponse(w, r, err, h.logger)
				return
			default:
				w.Header().Set("X-Project-Id", publicID)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(plan)
}

func storable(plan []byte, logger *slog.Logger) bool {
	if _, err := contract.Decode(plan); err != nil {
		logger.Error("parser produced a contract we cannot store", "err", err)
		return false
	}

	return true
}

func (h *Handler) allow(r *http.Request, u *user.User) bool {
	keys := []string{"upload-ip:" + clientip.From(r)}

	if !u.IsAnonymous() {
		keys = append(keys, "upload-user:"+strconv.FormatInt(u.ID, 10))
	}

	key, allowed := h.limiter.TakeAll(keys)
	if !allowed {
		h.logger.Warn("upload throttled", "limit", safelog.Key(key))
		return false
	}

	return true
}
