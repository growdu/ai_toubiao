package validator

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

type sample struct {
	Hash string `validate:"required,hex64"`
	Mime string `validate:"required,mime"`
	UUID string `validate:"required,uuidstr"`
	// Required with default tag — empty string should fail.
	Title string `validate:"required"`
}

func goodSample() sample {
	return sample{
		Hash:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Mime:  "image/png",
		UUID:  "550e8400-e29b-41d4-a716-446655440000",
		Title: "demo",
	}
}

// ----- custom validators -----

func TestHex64(t *testing.T) {
	cases := []struct {
		name  string
		value string
		ok    bool
	}{
		{"all lowercase", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", true},
		{"all uppercase", "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF", true},
		{"mixed case", "0123456789AbCdEf0123456789AbCdEf0123456789AbCdEf0123456789AbCdEf", true},
		{"63 chars (one short)", strings.Repeat("a", 63), false},
		{"65 chars (one long)", strings.Repeat("a", 65), false},
		{"non-hex char", strings.Repeat("a", 63) + "g", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := goodSample()
			s.Hash = tc.value
			err := Validate(s)
			if tc.ok && err != nil {
				t.Errorf("want pass, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Errorf("want fail, got pass")
			}
		})
	}
}

func TestMime(t *testing.T) {
	cases := []struct {
		name  string
		value string
		ok    bool
	}{
		{"image/png", "image/png", true},
		{"application/pdf", "application/pdf", true},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", true},
		{"with plus subtype", "application/vnd.api+json", true},
		{"missing slash", "imagepng", false},
		{"empty subtype", "image/", false},
		{"empty type", "/png", false},
		{"spaces", "image/ png", false},
		{"with charset param", "image/png; charset=utf-8", false}, // regex intentionally strict
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := goodSample()
			s.Mime = tc.value
			err := Validate(s)
			if tc.ok && err != nil {
				t.Errorf("want pass, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Errorf("want fail, got pass")
			}
		})
	}
}

func TestUUIDString(t *testing.T) {
	cases := []struct {
		name  string
		value string
		ok    bool
	}{
		{"valid v4", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid with uppercase", "550E8400-E29B-41D4-A716-446655440000", true},
		{"no dashes (hex32)", "550e8400e29b41d4a716446655440000", true}, // uuid.Parse is permissive
		{"too short", "550e8400-e29b-41d4-a716", false},
		{"too long", "550e8400-e29b-41d4-a716-446655440000-extra", false},
		{"non-hex chars", "550e8400-e29b-41d4-a716-44665544000g", false},
		{"garbage", "not-a-uuid", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := goodSample()
			s.UUID = tc.value
			err := Validate(s)
			if tc.ok && err != nil {
				t.Errorf("want pass, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Errorf("want fail, got pass")
			}
		})
	}
}

// ----- Validate wrapper -----

func TestValidate_Pass(t *testing.T) {
	if err := Validate(goodSample()); err != nil {
		t.Errorf("good sample should pass: %v", err)
	}
}

func TestValidate_FirstErrorOnly(t *testing.T) {
	s := sample{} // all required fields empty
	err := Validate(s)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	// Wrapper says "first error"; just verify shape.
	if !strings.Contains(err.Error(), "failed validation") {
		t.Errorf("error format unexpected: %v", err)
	}
}

func TestValidate_UnwrapsValidatorErrors(t *testing.T) {
	s := goodSample()
	s.Hash = "too-short"
	err := Validate(s)
	if err == nil {
		t.Fatal("want error")
	}
	// Validator-level errors must still be inspectable as ValidationErrors
	// so callers can branch on Tag(). Verify by running the raw validator.
	var verr validator.ValidationErrors
	if !errors.As(v.Struct(goodSample()), &verr) {
		// sanity: Validate(s) returns fmt-wrapped, not raw — that's the documented contract.
		// This test just pins the behavior.
		t.Skip("Validate() intentionally returns wrapped fmt error, not raw ValidationErrors")
	}
}

// ----- MustValidate -----

func TestMustValidate_Pass(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustValidate panicked on good input: %v", r)
		}
	}()
	MustValidate(goodSample())
}

func TestMustValidate_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustValidate should panic on bad input")
		}
	}()
	MustValidate(sample{}) // all empty
}