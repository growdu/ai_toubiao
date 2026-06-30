// Package proxy forwards HTTP requests to upstream services.
// Adds request_id propagation, panic recovery, and timeout.
package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
)

// Route maps a path prefix to an upstream URL.
type Route struct {
	Prefix   string
	Upstream *url.URL
}

// New returns a reverse proxy handler.
// The path is forwarded as-is — upstream services handle their own routing.
// We still match on `prefix` so the mux only routes requests to the right upstream.
func New(routes []Route) http.Handler {
	mux := http.NewServeMux()
	for _, r := range routes {
		prefix := r.Prefix
		upstream := r.Upstream
		rp := httputil.NewSingleHostReverseProxy(upstream)
		rp.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
			rid := logger.RequestIDFrom(req.Context())
			if errors.Is(err, context.Canceled) {
				return
			}
			httperr.Write(w, http.StatusBadGateway,
				httperr.CodeServiceUnavailable,
				"upstream unavailable", rid, nil)
		}

		// Register both /prefix and /prefix/* so trailing slash works either way.
		mux.Handle(prefix+"/", rp)
		mux.Handle(prefix, rp)
	}
	return mux
}

// Transport is a tuned http.Transport for proxying.
func Transport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // let upstream decide
	}
}