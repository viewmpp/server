package jsonutil

import (
	"log/slog"
	"net/http"
	"server/internal/safelog"
)

func errorResponse(w http.ResponseWriter, status int, data any) {
	_ = WriteJSON(w, status, data, nil)
}

func ServerErrorResponse(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	logger.Error("error occurs", "err", err, "method", r.Method, "uri", safelog.URI(r.RequestURI))
	message := "the server encountered a problem and could not process your request"
	errorResponse(w, http.StatusInternalServerError, message)
}

func BadRequestResponse(w http.ResponseWriter, err any) {
	errorResponse(w, http.StatusBadRequest, err)
}

func LengthRequiredResponse(w http.ResponseWriter) {
	message := "request must declare its size"
	errorResponse(w, http.StatusLengthRequired, message)
}

func ContentTooLargeError(w http.ResponseWriter) {
	message := "content too large"
	errorResponse(w, http.StatusRequestEntityTooLarge, message)
}

func TooManyRequestsResponse(w http.ResponseWriter) {
	errorResponse(w, http.StatusTooManyRequests, "too many uploads, try again shortly")
}

func NotFoundResponse(w http.ResponseWriter) {
	message := "not found"
	errorResponse(w, http.StatusNotFound, message)
}
