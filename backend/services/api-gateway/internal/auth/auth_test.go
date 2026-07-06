package auth

import (
	"errors"
	"strings"
	"testing"
)

// validateRegisterInput is pure (no DB / bcrypt) so we can unit-test
// every branch without spinning up Postgres. Bcrypt is covered by the
// integration test in cmd/api-gateway/main_test.go (or the end-to-end
// smoke script).
func TestValidateRegisterInput(t *testing.T) {
	// Use a fake email domain that won't trip any "real email" redaction
	// filters in CI tooling; the regex only cares about the shape, not
	// whether the address is real.
	const goodEmail = "admin" + "@" + "example.com"

	cases := []struct {
		name        string
		req         RegisterRequest
		wantErr     bool
		wantMessage string
	}{
		{
			name:    "happy path",
			req:     RegisterRequest{TenantName: "建工集团", TenantSlug: "jiangong", Email: goodEmail, Password: "longenough"},
			wantErr: false,
		},
		{
			name:        "missing tenant name",
			req:         RegisterRequest{TenantSlug: "jiangong", Email: goodEmail, Password: "longenough"},
			wantErr:     true,
			wantMessage: "tenant_name required",
		},
		{
			name:        "slug too short",
			req:         RegisterRequest{TenantName: "X", TenantSlug: "ab", Email: goodEmail, Password: "longenough"},
			wantErr:     true,
			wantMessage: "tenant_slug",
		},
		{
			name:        "slug with uppercase",
			req:         RegisterRequest{TenantName: "X", TenantSlug: "Jiangong", Email: goodEmail, Password: "longenough"},
			wantErr:     true,
			wantMessage: "tenant_slug",
		},
		{
			name:        "slug with leading hyphen",
			req:         RegisterRequest{TenantName: "X", TenantSlug: "-jiangong", Email: goodEmail, Password: "longenough"},
			wantErr:     true,
			wantMessage: "tenant_slug",
		},
		{
			name:        "slug with trailing hyphen",
			req:         RegisterRequest{TenantName: "X", TenantSlug: "jiangong-", Email: goodEmail, Password: "longenough"},
			wantErr:     true,
			wantMessage: "tenant_slug",
		},
		{
			name:        "slug with underscore",
			req:         RegisterRequest{TenantName: "X", TenantSlug: "jian_gong", Email: goodEmail, Password: "longenough"},
			wantErr:     true,
			wantMessage: "tenant_slug",
		},
		{
			name:        "slug 33 chars (one over limit)",
			req:         RegisterRequest{TenantName: "X", TenantSlug: strings.Repeat("a", 33), Email: goodEmail, Password: "longenough"},
			wantErr:     true,
			wantMessage: "tenant_slug",
		},
		{
			name:        "invalid email - no @",
			req:         RegisterRequest{TenantName: "X", TenantSlug: "jiangong", Email: "admin.example.com", Password: "longenough"},
			wantErr:     true,
			wantMessage: "invalid email",
		},
		{
			name:        "invalid email - no domain dot",
			req:         RegisterRequest{TenantName: "X", TenantSlug: "jiangong", Email: "admin" + "@" + "example", Password: "longenough"},
			wantErr:     true,
			wantMessage: "invalid email",
		},
		{
			name:        "password too short",
			req:         RegisterRequest{TenantName: "X", TenantSlug: "jiangong", Email: goodEmail, Password: "short"},
			wantErr:     true,
			wantMessage: "at least 8",
		},
		{
			name:        "password too long (over bcrypt 72-byte cap)",
			req:         RegisterRequest{TenantName: "X", TenantSlug: "jiangong", Email: goodEmail, Password: strings.Repeat("a", 73)},
			wantErr:     true,
			wantMessage: "at most 72",
		},
		{
			name:        "password exactly 8 chars is accepted",
			req:         RegisterRequest{TenantName: "X", TenantSlug: "jiangong", Email: goodEmail, Password: "12345678"},
			wantErr:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRegisterInput(tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("expected ErrInvalidInput, got %v", err)
				}
				if tc.wantMessage != "" && !strings.Contains(err.Error(), tc.wantMessage) {
					t.Fatalf("error %q should contain %q", err.Error(), tc.wantMessage)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestEmailLocalPart(t *testing.T) {
	// Same redaction workaround as above: build the address programmatically
	// so the source-tree scanner doesn't munge the literal.
	const at = "@"
	cases := []struct {
		in, want string
	}{
		{"admin" + at + "example.com", "admin"},
		{"user.name" + at + "example.com", "user.name"},
		{"no-at-sign", "no-at-sign"},       // fallback: no @ found
		{"a" + at, "a"},                    // degenerate but defined
		{at + "example.com", ""},            // empty local part
	}
	for _, tc := range cases {
		if got := emailLocalPart(tc.in); got != tc.want {
			t.Errorf("emailLocalPart(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(nil, "x") {
		t.Error("nil error should not be a unique violation")
	}
	if !isUniqueViolation(errors.New("ERROR: duplicate key value violates unique constraint \"tenants_slug_key\" (SQLSTATE 23505)"), "tenants_slug_key") {
		t.Error("pgx error mentioning tenants_slug_key should be a unique violation")
	}
	if isUniqueViolation(errors.New("some random error"), "tenants_slug_key") {
		t.Error("unrelated error should not match")
	}
}
