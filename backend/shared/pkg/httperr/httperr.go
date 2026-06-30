// Package httperr provides uniform HTTP error responses matching
// the API contract documented in docs/api/errors.md.
package httperr

import (
	"encoding/json"
	"net/http"
)

// Error is the wire format. Matches docs/api/errors.md.
type Error struct {
	Code            string         `json:"code"`
	Message         string         `json:"message"`
	Details         map[string]any `json:"details,omitempty"`
	RequestID       string         `json:"request_id,omitempty"`
	DocumentationURL string        `json:"documentation_url,omitempty"`
}

// Response wraps Error in the standard envelope.
type Response struct {
	Error Error `json:"error"`
}

// Common error codes (extend as needed). These match the API contract.
const (
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeTokenExpired       = "TOKEN_EXPIRED"
	CodeForbidden          = "FORBIDDEN"
	CodeTenantMismatch     = "TENANT_MISMATCH"
	CodeInvalidInput       = "INVALID_INPUT"
	CodeMissingField       = "MISSING_FIELD"
	CodeNotFound           = "NOT_FOUND"
	CodeAlreadyExists      = "ALREADY_EXISTS"
	CodeVersionConflict    = "VERSION_CONFLICT"
	CodeRateLimited        = "RATE_LIMITED"
	CodeQuotaExceeded      = "QUOTA_EXCEEDED"
	CodeInternalError      = "INTERNAL_ERROR"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// Write writes a uniform JSON error response.
func Write(w http.ResponseWriter, status int, code, message, requestID string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{
		Error: Error{
			Code:      code,
			Message:   message,
			Details:   details,
			RequestID: requestID,
		},
	})
}

// Common helpers.

func Unauthorized(w http.ResponseWriter, reqID string) {
	Write(w, http.StatusUnauthorized, CodeUnauthorized, "缺少或无效的 token", reqID, nil)
}

func Forbidden(w http.ResponseWriter, reqID, message string) {
	Write(w, http.StatusForbidden, CodeForbidden, message, reqID, nil)
}

func NotFound(w http.ResponseWriter, reqID, what string) {
	Write(w, http.StatusNotFound, CodeNotFound, what+" 不存在", reqID, nil)
}

func InvalidInput(w http.ResponseWriter, reqID, message string, details map[string]any) {
	Write(w, http.StatusBadRequest, CodeInvalidInput, message, reqID, details)
}

func InternalError(w http.ResponseWriter, reqID string) {
	Write(w, http.StatusInternalServerError, CodeInternalError, "服务器内部错误", reqID, nil)
}