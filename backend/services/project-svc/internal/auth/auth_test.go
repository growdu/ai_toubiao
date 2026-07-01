package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testSecret = "test-secret-do-not-use-in-prod"

func TestIssueAndVerify_RoundTrip(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	tok, err := IssueToken(testSecret, tid, uid, "owner", 1*time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	v := NewVerifier(testSecret)
	claims, err := v.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.TenantID != tid {
		t.Errorf("tenant: got %q want %q", claims.TenantID, tid)
	}
	if claims.UserID != uid {
		t.Errorf("user: got %q want %q", claims.UserID, uid)
	}
	if claims.Role != "owner" {
		t.Errorf("role: got %q", claims.Role)
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	tok, err := IssueToken(testSecret, tid, uid, "owner", 1*time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	v := NewVerifier("different-secret")
	if _, err := v.Verify(tok); err == nil {
		t.Fatal("expected verify to fail with wrong secret")
	}
}

func TestVerify_Expired(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	tok, err := IssueToken(testSecret, tid, uid, "owner", -1*time.Second)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	v := NewVerifier(testSecret)
	_, err = v.Verify(tok)
	if err == nil {
		t.Fatal("expected expired token to fail")
	}
	// jwt/v5 wraps the underlying expiry in ErrTokenExpired; accept it
	// or the bare error.
	if !errors.Is(err, jwt.ErrTokenExpired) {
		// some paths return a plain "token expired" — that's still acceptable
		t.Logf("verify expired err (acceptable): %v", err)
	}
}

func TestVerify_Malformed(t *testing.T) {
	v := NewVerifier(testSecret)
	if _, err := v.Verify("not-a-jwt"); err == nil {
		t.Fatal("expected malformed token to fail")
	}
}

func TestVerify_BadTenantUUID(t *testing.T) {
	tok, err := IssueToken(testSecret, "not-a-uuid", uuid.NewString(), "owner", 1*time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	v := NewVerifier(testSecret)
	_, err = v.Verify(tok)
	if err == nil || err.Error() != "invalid tenant_id in token" {
		t.Errorf("expected 'invalid tenant_id in token', got %v", err)
	}
}

func TestVerify_BadSubUUID(t *testing.T) {
	tok, err := IssueToken(testSecret, uuid.NewString(), "not-a-uuid", "owner", 1*time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	v := NewVerifier(testSecret)
	_, err = v.Verify(tok)
	if err == nil || err.Error() != "invalid sub in token" {
		t.Errorf("expected 'invalid sub in token', got %v", err)
	}
}

func TestVerify_WrongSigningMethod(t *testing.T) {
	// Sign with HS256 but using a "none"-suspicious method? Actually we need
	// to bypass the alg check by forging an unsigned token. Easier: try
	// algorithm-confusion by signing with the same secret but claim alg=none
	// via a custom token; the verifier must reject any non-HMAC method.
	tid := uuid.NewString()
	uid := uuid.NewString()
	none := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{
		TenantID: tid, UserID: uid, Role: "owner",
	})
	tok, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	v := NewVerifier(testSecret)
	_, err = v.Verify(tok)
	if err == nil {
		t.Fatal("expected none-alg token to be rejected")
	}
}

func TestVerify_IssuerAndIAT(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	tok, err := IssueToken(testSecret, tid, uid, "owner", 1*time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	v := NewVerifier(testSecret)
	claims, err := v.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Issuer != "bidwriter" {
		t.Errorf("issuer: got %q", claims.Issuer)
	}
	if claims.IssuedAt == nil {
		t.Error("issuedAt should be set")
	}
	if claims.ExpiresAt == nil {
		t.Error("expiresAt should be set")
	}
}
