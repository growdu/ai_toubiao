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