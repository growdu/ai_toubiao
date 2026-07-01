package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bidwriter/shared/pkg/httperr"
)

// upstream is a controllable fake upstream: records every request and can
// be configured to return any status/body. One per simulated service.
type upstream struct {
	server *httptest.Server
	hits   atomic.Int64
	// last request fields:
	lastMethod string
	lastPath   string
	lastQuery  string
	lastBody   []byte
	lastHeader http.Header
}

func newUpstream(t *testing.T, handler http.HandlerFunc) *upstream {
	t.Helper()
	u := &upstream{}
	u.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		u.lastMethod = r.Method
		u.lastPath = r.URL.Path
		u.lastQuery = r.URL.RawQuery
		u.lastBody = body
		u.lastHeader = r.Header.Clone()
		handler(w, r)
	}))
	t.Cleanup(u.server.Close)
	return u
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func newGateway(t *testing.T, routes []Route) http.Handler {
	t.Helper()
	return New(routes)
}

func TestNew_RoutesRequestToCorrectUpstream(t *testing.T) {
	workflow := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("from-workflow"))
	})
	project := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("from-project"))
	})

	gw := newGateway(t, []Route{
		{Prefix: "/api/v1/bids", Upstream: mustParseURL(t, workflow.server.URL)},
		{Prefix: "/api/v1/projects", Upstream: mustParseURL(t, project.server.URL)},
	})

	cases := []struct {
		path string
		want string
	}{
		{"/api/v1/bids", "from-workflow"},
		{"/api/v1/bids/", "from-workflow"},
		{"/api/v1/bids/abc-123", "from-workflow"},
		{"/api/v1/bids/abc/export/pdf", "from-workflow"},
		{"/api/v1/projects", "from-project"},
		{"/api/v1/projects/xyz", "from-project"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			gw.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("status=%d, want 200; body=%s", rr.Code, rr.Body.String())
			}
			if got := rr.Body.String(); got != tc.want {
				t.Errorf("body=%q, want %q", got, tc.want)
			}
		})
	}

	if got := workflow.hits.Load(); got != 4 {
		t.Errorf("workflow hits=%d, want 4", got)
	}
	if got := project.hits.Load(); got != 2 {
		t.Errorf("project hits=%d, want 2", got)
	}
}

func TestNew_UnknownRouteReturns404(t *testing.T) {
	// Use an upstream that simply won't be hit.
	gw := newGateway(t, []Route{
		{Prefix: "/api/v1/bids", Upstream: mustParseURL(t, httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).URL)},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	rr := httptest.NewRecorder()
	gw.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", rr.Code)
	}
}

func TestNew_PreservesMethodBodyAndQueryString(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	gw := newGateway(t, []Route{{Prefix: "/api/v1/bids", Upstream: mustParseURL(t, up.server.URL)}})

	body := strings.NewReader(`{"project_id":"p1","title":"hi"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bids?trace=1&cursor=abc", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	gw.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status=%d, want 201", rr.Code)
	}
	if up.lastMethod != http.MethodPost {
		t.Errorf("upstream method=%q, want POST", up.lastMethod)
	}
	if up.lastQuery != "trace=1&cursor=abc" {
		t.Errorf("upstream query=%q, want trace=1&cursor=abc", up.lastQuery)
	}
	if string(up.lastBody) != `{"project_id":"p1","title":"hi"}` {
		t.Errorf("upstream body=%q", string(up.lastBody))
	}
	if up.lastHeader.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type not forwarded: %q", up.lastHeader.Get("Content-Type"))
	}
}

func TestNew_ForwardsAuthorizationAndCustomHeaders(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {})
	gw := newGateway(t, []Route{{Prefix: "/api/v1/bids", Upstream: mustParseURL(t, up.server.URL)}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bids/x", nil)
	req.Header.Set("Authorization", "Bearer test-token-xyz")
	req.Header.Set("X-Tenant-Id", "tenant-1")
	req.Header.Set("X-Request-Id", "rid-42")
	rr := httptest.NewRecorder()
	gw.ServeHTTP(rr, req)

	if up.lastHeader.Get("Authorization") != "Bearer test-token-xyz" {
		t.Errorf("Authorization not forwarded: %q", up.lastHeader.Get("Authorization"))
	}
	if up.lastHeader.Get("X-Tenant-Id") != "tenant-1" {
		t.Errorf("X-Tenant-Id not forwarded: %q", up.lastHeader.Get("X-Tenant-Id"))
	}
	if up.lastHeader.Get("X-Request-Id") != "rid-42" {
		t.Errorf("X-Request-Id not forwarded: %q", up.lastHeader.Get("X-Request-Id"))
	}
}

func TestNew_UpstreamDown_Returns502WithEnvelope(t *testing.T) {
	// Point at a port that is guaranteed to be closed (1 is reserved).
	gw := newGateway(t, []Route{{Prefix: "/api/v1/bids", Upstream: mustParseURL(t, "http://127.0.0.1:1")}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bids/abc", nil)
	rr := httptest.NewRecorder()
	gw.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status=%d, want 502; body=%s", rr.Code, rr.Body.String())
	}
	var env httperr.Response
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("body not JSON envelope: %v; raw=%s", err, rr.Body.String())
	}
	if env.Error.Code != httperr.CodeServiceUnavailable {
		t.Errorf("error.code=%q, want %q", env.Error.Code, httperr.CodeServiceUnavailable)
	}
	if !strings.Contains(strings.ToLower(env.Error.Message), "upstream") {
		t.Errorf("message should mention upstream: %q", env.Error.Message)
	}
}

func TestNew_TrailingSlashVariantsBothWork(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	})
	gw := newGateway(t, []Route{{Prefix: "/api/v1/bids", Upstream: mustParseURL(t, up.server.URL)}})

	for _, p := range []string{"/api/v1/bids", "/api/v1/bids/"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rr := httptest.NewRecorder()
		gw.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s: status=%d", p, rr.Code)
		}
	}
}

func TestNew_UpstreamResponseBodyPassedThrough(t *testing.T) {
	// Upstream returns a body with a custom Content-Type; the proxy must
	// surface both verbatim — important for /export/pdf returning
	// application/pdf bytes.
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("%PDF-1.4\nfake body"))
	})
	gw := newGateway(t, []Route{{Prefix: "/api/v1/bids", Upstream: mustParseURL(t, up.server.URL)}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bids/x/export/pdf", nil)
	rr := httptest.NewRecorder()
	gw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type=%q, want application/pdf", ct)
	}
	if !strings.HasPrefix(rr.Body.String(), "%PDF-1.4") {
		t.Errorf("body not PDF: %q", rr.Body.String())
	}
}

func TestTransport_HasSensibleDefaults(t *testing.T) {
	tr := Transport()
	if tr.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns=%d, want 100", tr.MaxIdleConns)
	}
	if tr.MaxIdleConnsPerHost != 20 {
		t.Errorf("MaxIdleConnsPerHost=%d, want 20", tr.MaxIdleConnsPerHost)
	}
	if tr.IdleConnTimeout.Seconds() != 90 {
		t.Errorf("IdleConnTimeout=%v, want 90s", tr.IdleConnTimeout)
	}
	if !tr.DisableCompression {
		t.Error("DisableCompression should be true (let upstream decide)")
	}
	if tr.DisableKeepAlives {
		t.Error("DisableKeepAlives should default to false (keep-alive for proxy hot path)")
	}
}