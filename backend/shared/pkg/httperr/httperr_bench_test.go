package httperr

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkWrite measures the cost of marshaling + writing a typical error
// envelope. Useful as a baseline: every error path in the system goes
// through this, so a regression here shows up everywhere.
func BenchmarkWrite(b *testing.B) {
	rr := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Write(rr, http.StatusNotFound, "NOT_FOUND", "项目不存在", "rid-abc", map[string]any{"id": "x"})
	}
}

// BenchmarkWriteNoDetails isolates the cost of writing without the
// optional Details map (the common case for 401/403/500).
func BenchmarkWriteNoDetails(b *testing.B) {
	rr := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Write(rr, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误", "rid", nil)
	}
}