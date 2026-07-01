package httperr

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteFormat(t *testing.T) {
	rr := httptest.NewRecorder()
	Write(rr, 404, "NOT_FOUND", "项目不存在", "rid-abc", map[string]any{"id": "x"})

	if rr.Code != 404 {
		t.Errorf("status: got %d want 404", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}

	var resp Response
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error.Code != "NOT_FOUND" {
		t.Errorf("code: got %q", resp.Error.Code)
	}
	if resp.Error.RequestID != "rid-abc" {
		t.Errorf("request_id: got %q", resp.Error.RequestID)
	}
	if resp.Error.Details["id"] != "x" {
		t.Errorf("details: got %v", resp.Error.Details)
	}
}

func TestUnauthorizedHelper(t *testing.T) {
	rr := httptest.NewRecorder()
	Unauthorized(rr, "rid-1")
	if rr.Code != 401 {
		t.Errorf("status: got %d want 401", rr.Code)
	}
}

func TestVersionConflictShape(t *testing.T) {
	rr := httptest.NewRecorder()
	Write(rr, 409, CodeVersionConflict, "版本冲突", "rid-2", nil)
	if rr.Code != 409 {
		t.Errorf("status: got %d want 409", rr.Code)
	}
}

func TestForbiddenHelper(t *testing.T) {
	rr := httptest.NewRecorder()
	Forbidden(rr, "rid-3", "无权访问")
	if rr.Code != 403 {
		t.Errorf("status: got %d want 403", rr.Code)
	}
	var resp Response
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error.Code != CodeForbidden {
		t.Errorf("code: got %q want %q", resp.Error.Code, CodeForbidden)
	}
	if resp.Error.Message != "无权访问" {
		t.Errorf("message: got %q", resp.Error.Message)
	}
}

func TestNotFoundHelper(t *testing.T) {
	rr := httptest.NewRecorder()
	NotFound(rr, "rid-4", "项目")
	if rr.Code != 404 {
		t.Errorf("status: got %d want 404", rr.Code)
	}
	var resp Response
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error.Code != CodeNotFound {
		t.Errorf("code: got %q", resp.Error.Code)
	}
	// NotFound appends " 不存在" to the supplied "what".
	if resp.Error.Message != "项目 不存在" {
		t.Errorf("message: got %q want %q", resp.Error.Message, "项目 不存在")
	}
}

func TestInvalidInputHelper(t *testing.T) {
	rr := httptest.NewRecorder()
	InvalidInput(rr, "rid-5", "字段缺失", map[string]any{"field": "name"})
	if rr.Code != 400 {
		t.Errorf("status: got %d want 400", rr.Code)
	}
	var resp Response
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error.Code != CodeInvalidInput {
		t.Errorf("code: got %q", resp.Error.Code)
	}
	if resp.Error.Details["field"] != "name" {
		t.Errorf("details: got %v", resp.Error.Details)
	}
}

func TestInternalErrorHelper(t *testing.T) {
	rr := httptest.NewRecorder()
	InternalError(rr, "rid-6")
	if rr.Code != 500 {
		t.Errorf("status: got %d want 500", rr.Code)
	}
	var resp Response
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error.Code != CodeInternalError {
		t.Errorf("code: got %q", resp.Error.Code)
	}
}

func TestWrite_NilDetailsOmitted(t *testing.T) {
	// Verify `details` is omitted from JSON when nil (per json:\"details,omitempty\").
	rr := httptest.NewRecorder()
	Write(rr, 500, "X", "y", "rid", nil)
	body := rr.Body.String()
	if !contains(body, `"request_id":"rid"`) {
		t.Errorf("request_id missing: %s", body)
	}
	// Should NOT contain a `"details"` key at all.
	if contains(body, `"details"`) {
		t.Errorf("details should be omitted, got: %s", body)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}