// Package auth handles login + JWT issuance for the api-gateway.
// In production this would integrate with the dedicated auth-svc;
// for now we read users directly from the DB.
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials is returned for bad email/password combinations.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrTenantNotFound is returned when the tenant_slug doesn't exist.
var ErrTenantNotFound = errors.New("tenant not found")

// User is the authenticated principal.
type User struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	Email    string
	Role     string
}

// Claims is the JWT payload.
type Claims struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"sub"`
	Role     string `json:"role"`
	Type     string `json:"type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// Service handles authentication.
type Service struct {
	pool        *pgxpool.Pool
	jwtSecret   []byte
	accessTTL   time.Duration
	refreshTTL  time.Duration
}

// New creates a Service.
func New(pool *pgxpool.Pool, jwtSecret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		pool:       pool,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// Login validates credentials and returns the user.
func (s *Service) Login(ctx context.Context, tenantSlug, email, password string) (*User, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.tenant_id, u.email, u.role, u.password_hash
		FROM users u
		JOIN tenants t ON t.id = u.tenant_id
		WHERE t.slug = $1
		  AND u.email = $2
		  AND u.status = 'active'
		  AND t.status = 'active'`,
		tenantSlug, email,
	).Scan(&u.ID, &u.TenantID, &u.Email, &u.Role, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Update last_login_at (best-effort)
	_, _ = s.pool.Exec(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, u.ID)

	return &u, nil
}

// IssueTokens creates access + refresh tokens for a user.
func (s *Service) IssueTokens(u *User) (access, refresh string, expiresIn int, err error) {
	now := time.Now()
	expiresIn = int(s.accessTTL.Seconds())

	access, err = jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		TenantID: u.TenantID.String(),
		UserID:   u.ID.String(),
		Role:     u.Role,
		Type:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			Issuer:    "bidwriter",
		},
	}).SignedString(s.jwtSecret)
	if err != nil {
		return "", "", 0, err
	}

	refresh, err = jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		TenantID: u.TenantID.String(),
		UserID:   u.ID.String(),
		Role:     u.Role,
		Type:     "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshTTL)),
			Issuer:    "bidwriter",
		},
	}).SignedString(s.jwtSecret)
	return access, refresh, expiresIn, err
}

// Verify parses and validates a token.
func (s *Service) Verify(tokenStr string) (*Claims, error) {
	c := &Claims{}
	t, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if !t.Valid {
		return nil, errors.New("invalid token")
	}
	if c.Type != "access" {
		return nil, errors.New("not an access token")
	}
	return c, nil
}