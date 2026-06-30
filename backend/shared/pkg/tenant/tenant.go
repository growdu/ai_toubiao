// Package tenant provides tenant_id propagation through context.
// Every request must carry a tenant_id; all DB queries must filter by it.
package tenant

import (
	"context"
	"errors"
)

type ctxKey struct{}

var ErrNoTenant = errors.New("no tenant in context")

// WithTenant stores tenant_id in the context.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, tenantID)
}

// FromContext retrieves tenant_id. Returns ErrNoTenant if absent.
func FromContext(ctx context.Context) (string, error) {
	v, ok := ctx.Value(ctxKey{}).(string)
	if !ok || v == "" {
		return "", ErrNoTenant
	}
	return v, nil
}

// MustFromContext panics if tenant_id is missing.
// Use only in handlers after auth middleware has run.
func MustFromContext(ctx context.Context) string {
	id, err := FromContext(ctx)
	if err != nil {
		panic(err)
	}
	return id
}