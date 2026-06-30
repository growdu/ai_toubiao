// Package httpx provides tiny helpers around net/http.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// ReadJSON decodes the request body into v. Limits to 1 MB.
func ReadJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20)) // 1 MB
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}