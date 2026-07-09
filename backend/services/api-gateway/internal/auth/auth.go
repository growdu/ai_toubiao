// Package auth handles login + JWT issuance for the api-gateway.
// In production this would integrate with the dedicated auth-svc;
// for now we read users directly from the DB.
package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// slugRegex enforces 3..32 chars of [a-z0-9-] with hyphens not at the
// edges — same shape the front-end uses in slugify().
var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`)

// emailRegex is intentionally permissive — the canonical RFC 5321 regex
// is huge. We just check that there's a non-empty local part, an "@",
// and a non-empty domain with at least one dot.
var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// ErrInvalidCredentials is returned for bad email/password combinations.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrTenantNotFound is returned when the tenant_slug doesn't exist.
var ErrTenantNotFound = errors.New("tenant not found")

// ErrTenantSlugTaken is returned when a Register call hits a slug that
// is already in use. Clients should surface this as "that workspace ID
// is taken, please pick another one".
var ErrTenantSlugTaken = errors.New("tenant slug already taken")

// ErrEmailTaken is returned when the (tenant_id, email) pair collides
// on Register.
var ErrEmailTaken = errors.New("email already in use for this tenant")

// ErrInvalidInput is returned for malformed Register payloads (bad
// password length, malformed email, etc).
var ErrInvalidInput = errors.New("invalid input")

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

// RefreshTokens validates a refresh token and issues a fresh access +
// refresh token pair. The old refresh token remains valid until its
// natural expiry (no revocation list yet); rotating it client-side is
// the caller's responsibility.
func (s *Service) RefreshTokens(refreshToken string) (access, refresh string, expiresIn int, err error) {
	c := &Claims{}
	t, err := jwt.ParseWithClaims(refreshToken, c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !t.Valid {
		return "", "", 0, errors.New("invalid refresh token")
	}
	if c.Type != "refresh" {
		return "", "", 0, errors.New("not a refresh token")
	}
	uid, err := uuid.Parse(c.UserID)
	if err != nil {
		return "", "", 0, fmt.Errorf("parse user id: %w", err)
	}
	tid, err := uuid.Parse(c.TenantID)
	if err != nil {
		return "", "", 0, fmt.Errorf("parse tenant id: %w", err)
	}
	return s.IssueTokens(&User{ID: uid, TenantID: tid, Role: c.Role})
}

// RegisterRequest is the input to Register. We validate fields server-side
// even though the front-end also checks — a CLI / curl user shouldn't be
// able to skip client validation.
type RegisterRequest struct {
	TenantName  string `json:"tenant_name"`    // display name, e.g. "建工集团投标部"
	TenantSlug  string `json:"tenant_slug"`    // URL-safe identifier, e.g. "jiangong"; must match ^[a-z0-9-]{3,32}$
	Email       string `json:"email"`          // login email
	Password    string `json:"password"`       // must be 8..72 characters
	DisplayName string `json:"display_name,omitempty"` // optional, defaults to local-part of email
	InitialPlan string `json:"initial_plan,omitempty"` // optional, one of "free"|"pro"|"enterprise"; defaults to "free"
}

// RegisteredResult is what Register returns on success. We re-use the
// existing User + token issuance paths so the response shape is
// identical to Login.
type RegisteredResult struct {
	User       *User
	TenantID   uuid.UUID
	TenantName string
	TenantSlug string
	Plan       string
}

// Register creates a new tenant + owner account atomically. Steps:
//
//	1. Validate input shape (slug regex, email presence, password length).
//	2. Begin a DB transaction.
//	3. INSERT tenant (catch unique-slug violation → ErrTenantSlugTaken).
//	4. INSERT user with bcrypt-hashed password + role=owner.
//	   Catch unique-(tenant_id,email) → ErrEmailTaken.
//	5. Commit. Return the User so IssueTokens can mint a JWT, just like
//	   Login.
//
// We deliberately don't queue a welcome email here — that integration
// is owned by notify-svc and lives behind a separate endpoint.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*RegisteredResult, error) {
	// ---- 1. shape validation ----
	if err := validateRegisterInput(req); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = emailLocalPart(req.Email)
	}

	plan := req.InitialPlan
	if plan == "" {
		plan = "free"
	}
	if plan != "free" && plan != "pro" && plan != "enterprise" {
		return nil, fmt.Errorf("%w: plan must be free|pro|enterprise", ErrInvalidInput)
	}

	// ---- 2-5. atomic insert ----
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tenantID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO tenants (name, slug, plan, status)
		VALUES ($1, $2, $3, 'active')
		RETURNING id`,
		req.TenantName, req.TenantSlug, plan,
	).Scan(&tenantID)
	if err != nil {
		if isUniqueViolation(err, "tenants_slug_key") {
			return nil, ErrTenantSlugTaken
		}
		return nil, fmt.Errorf("insert tenant: %w", err)
	}

	var user User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (tenant_id, email, password_hash, display_name, role, status)
		VALUES ($1, $2, $3, $4, 'owner', 'active')
		RETURNING id, tenant_id, email, role`,
		tenantID, req.Email, string(hash), displayName,
	).Scan(&user.ID, &user.TenantID, &user.Email, &user.Role)
	if err != nil {
		if isUniqueViolation(err, "users_tenant_id_email_key") {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &RegisteredResult{
		User:       &user,
		TenantID:   tenantID,
		TenantName: req.TenantName,
		TenantSlug: req.TenantSlug,
		Plan:       plan,
	}, nil
}

// validateRegisterInput returns ErrInvalidInput (wrapped with a
// caller-friendly message) for any malformed field. Centralising this
// keeps the handler-side code focused on translating errors to HTTP.
func validateRegisterInput(req RegisterRequest) error {
	if req.TenantName == "" {
		return fmt.Errorf("%w: tenant_name required", ErrInvalidInput)
	}
	if !slugRegex.MatchString(req.TenantSlug) {
		return fmt.Errorf("%w: tenant_slug must match ^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$", ErrInvalidInput)
	}
	if !emailRegex.MatchString(req.Email) {
		return fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	if len(req.Password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	if len(req.Password) > 72 {
		// bcrypt has a 72-byte cap; reject longer passwords up-front so
		// the user gets a clear error instead of a silent truncation.
		return fmt.Errorf("%w: password must be at most 72 characters", ErrInvalidInput)
	}
	return nil
}

// emailLocalPart returns the part of an email address before "@".
// Falls back to the full address if no "@" is found.
func emailLocalPart(email string) string {
	for i, ch := range email {
		if ch == '@' {
			return email[:i]
		}
	}
	return email
}

// isUniqueViolation checks whether the pgx error is a 23505 (unique_violation)
// targeting the given index name. The driver puts the constraint name
// in err message after "constraint" so we substring-match rather than
// parsing the SQLSTATE.
func isUniqueViolation(err error, indexName string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, indexName)
}