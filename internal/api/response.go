package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
)

type errorEnvelope struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type collectionEnvelope struct {
	Items  any   `json:"items"`
	Total  int   `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	var code string
	var status int

	switch {
	case errors.Is(err, ErrNotFound):
		status, code = http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, ErrInvalidInput):
		status, code = http.StatusUnprocessableEntity, "INVALID_INPUT"
	default:
		status, code = http.StatusInternalServerError, "INTERNAL_ERROR"
	}

	writeJSON(w, status, errorEnvelope{Error: err.Error(), Code: code})
}

func writeCollection(w http.ResponseWriter, items any, total, limit, offset int) {
	writeJSON(w, http.StatusOK, collectionEnvelope{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}
