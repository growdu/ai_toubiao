package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims represents the JWT payload used by bidwriter services.
// Tokens are HS256-signed by the api-gateway.
type Claims struct {
	Sub      string `json:"sub"`
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
	Email    string `json:"email"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
}

// parseJWT verifies an HS256 JWT signature and returns its claims.
// It rejects expired tokens.
func parseJWT(token, secret string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed token")
	}

	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("malformed header: %w", err)
	}
	var h struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(header, &h); err != nil {
		return nil, fmt.Errorf("invalid header json: %w", err)
	}
	if h.Alg != "HS256" {
		return nil, fmt.Errorf("unsupported alg: %s", h.Alg)
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, errors.New("bad signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("malformed payload: %w", err)
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, fmt.Errorf("invalid payload json: %w", err)
	}
	if c.Exp > 0 && time.Now().Unix() >= c.Exp {
		return nil, errors.New("token expired")
	}
	return &c, nil
}