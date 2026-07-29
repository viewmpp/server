package jsonutil

import (
	"encoding/json"
	"maps"
	"net/http"
)

type Envelope map[string]any

func WriteJSON(w http.ResponseWriter, status int, data any, headers http.Header) error {
	j, err := json.Marshal(data)
	if err != nil {
		return err
	}

	j = append(j, '\n')

	maps.Copy(w.Header(), headers)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(j)

	return err
}
