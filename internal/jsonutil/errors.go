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
	errorResponse(w, http.StatusInternalServerError, "the server encountered a problem and could not process your request")
}

func BadRequestResponse(w http.ResponseWriter, err any) {
	errorResponse(w, http.StatusBadRequest, err)
}

func LengthRequiredResponse(w http.ResponseWriter) {
	errorResponse(w, http.StatusLengthRequired, "request must declare its size")
}

func ContentTooLargeError(w http.ResponseWriter) {
	errorResponse(w, http.StatusRequestEntityTooLarge, "content too large")
}

func SaveLimitResponse(w http.ResponseWriter, message string) {
	errorResponse(w, http.StatusConflict, message)
}

func TooManyRequestsResponse(w http.ResponseWriter, message string) {
	errorResponse(w, http.StatusTooManyRequests, message)
}

func ForbiddenResponse(w http.ResponseWriter, message string) {
	errorResponse(w, http.StatusForbidden, message)
}

func UnauthorizedResponse(w http.ResponseWriter) {
	errorResponse(w, http.StatusUnauthorized, "you must be signed in to do that")
}

func NotFoundResponse(w http.ResponseWriter) {
	errorResponse(w, http.StatusNotFound, "not found")
}
