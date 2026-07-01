package validator

import "testing"

// BenchmarkValidate measures the cost of running all custom validators
// (hex64 + mime + uuidstr) plus the built-in `required` on a typical
// request struct. Hot path: every incoming API request that has a
// `validate:"..."` struct tag goes through this.
func BenchmarkValidate(b *testing.B) {
	s := goodSample()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(s)
	}
}

// BenchmarkHex64Only exercises a struct with only the hex64 tag so the
// cost of one custom regex is visible in isolation.
func BenchmarkHex64Only(b *testing.B) {
	s := struct {
		Hash string `validate:"hex64"`
	}{Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(s)
	}
}

// BenchmarkMimeOnly exercises a single mime-type validation, which is
// the hot path for /upload endpoints.
func BenchmarkMimeOnly(b *testing.B) {
	s := struct {
		Mime string `validate:"mime"`
	}{Mime: "application/pdf"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(s)
	}
}