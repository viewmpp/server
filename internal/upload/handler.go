package upload

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"server/internal/jsonutil"
	"server/internal/parser"
	"server/internal/user"
	"strings"
	"unicode/utf8"
)

type Handler struct {
	logger *slog.Logger
	client *parser.Client
	store  uploadStore
}

type uploadStore interface {
	Save(ctx context.Context, id int64, fileName string, contract []byte) (string, error)
}

func NewHandler(logger *slog.Logger, client *parser.Client, store uploadStore) *Handler {
	return &Handler{
		logger: logger,
		client: client,
		store:  store,
	}
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	u := user.GetUserContext(r)

	limit := u.MaxUploadBytes()

	if r.ContentLength > limit {
		jsonutil.ContentTooLargeError(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, limit)

	contract, err := h.client.Parse(r.Context(), r.Body)
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
		filename := sanitizeFileName(r.URL.Query().Get("name"))
		publicID, err := h.store.Save(r.Context(), u.ID, filename, contract)
		if err != nil {
			jsonutil.ServerErrorResponse(w, r, err, h.logger)
			return
		}
		w.Header().Set("X-Project-Id", publicID)
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(contract)
}

func sanitizeFileName(name string) string {
	name = filepath.Base(name)
	if !utf8.ValidString(name) {
		return ""
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	for len(name) > 255 {
		_, size := utf8.DecodeLastRuneInString(name[:255])
		name = name[:255-size+size%1]
	}

	return name
}
