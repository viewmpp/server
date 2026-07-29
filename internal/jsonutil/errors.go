package jsonutil

import (
	"log/slog"
	"net/http"
)

func errorResponse(w http.ResponseWriter, status int, message any) {
	_ = WriteJSON(w, status, Envelope{"error": message}, nil)
}

func ServerErrorResponse(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	logger.Error("error occurs", "err", err, "method", r.Method, "uri", r.RequestURI)
	message := "the server encountered a problem and could not process your request"
	errorResponse(w, http.StatusInternalServerError, message)
}
