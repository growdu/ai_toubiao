// Package auth provides JWT validation and tenant/user extraction.
// In production this would call the auth-svc or validate JWTs locally.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the JWT payload.
type Claims struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"sub"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Verifier validates JWTs using HS256.
type Verifier struct {
	secret []byte
}

// NewVerifier creates a Verifier.
func NewVerifier(secret string) *Verifier {
	return &Verifier{secret: []byte(secret)}
}

// Verify parses and validates a token.
func (v *Verifier) Verify(tokenStr string) (*Claims, error) {
	c := &Claims{}
	t, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return v.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !t.Valid {
		return nil, errors.New("invalid token")
	}
	if _, err := uuid.Parse(c.TenantID); err != nil {
		return nil, errors.New("invalid tenant_id in token")
	}
	if _, err := uuid.Parse(c.UserID); err != nil {
		return nil, errors.New("invalid sub in token")
	}
	return c, nil
}

// IssueToken is a helper for tests.
func IssueToken(secret, tenantID, userID, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	c := Claims{
		TenantID: tenantID,
		UserID:   userID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "bidwriter",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
}