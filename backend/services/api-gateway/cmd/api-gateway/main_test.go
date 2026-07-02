package main

import (
	"net/url"
	"testing"

	"github.com/bidwriter/services/api-gateway/internal/config"
	"github.com/bidwriter/services/api-gateway/internal/proxy"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return u
}

// hasPrefix returns true if routes contains a Route with the given Prefix
// whose Upstream matches the supplied URL (string compare for stability).
func hasPrefix(routes []proxy.Route, prefix, upstream string) bool {
	for _, r := range routes {
		if r.Prefix != prefix {
			continue
		}
		if r.Upstream == nil {
			continue
		}
		if r.Upstream.String() == upstream {
			return true
		}
	}
	return false
}

func TestBuildRoutes_IncludesKnowledgeRoute(t *testing.T) {
	cfg := &config.Config{
		ProjectSvcURL:  "http://project:8081",
		DocumentSvcURL: "http://document:8082",
		WorkflowSvcURL: "http://workflow:8083",
		KnowledgeSvcURL: "http://knowledge:8084",
	}

	got, err := buildRoutes(cfg)
	if err != nil {
		t.Fatalf("buildRoutes: %v", err)
	}

	if !hasPrefix(got, "/api/v1/knowledge", "http://knowledge:8084") {
		t.Errorf("routes missing /api/v1/knowledge -> http://knowledge:8084, got: %+v", got)
	}
}

func TestBuildRoutes_IncludesAllExpectedPrefixes(t *testing.T) {
	cfg := &config.Config{
		ProjectSvcURL:  "http://project:8081",
		DocumentSvcURL: "http://document:8082",
		WorkflowSvcURL: "http://workflow:8083",
		KnowledgeSvcURL: "http://knowledge:8084",
	}

	got, err := buildRoutes(cfg)
	if err != nil {
		t.Fatalf("buildRoutes: %v", err)
	}

	expected := []struct {
		prefix   string
		upstream string
	}{
		{"/api/v1/projects", "http://project:8081"},
		{"/api/v1/documents", "http://document:8082"},
		{"/api/v1/bids", "http://workflow:8083"},
		{"/api/v1/knowledge", "http://knowledge:8084"},
	}
	for _, e := range expected {
		if !hasPrefix(got, e.prefix, e.upstream) {
			t.Errorf("routes missing %s -> %s", e.prefix, e.upstream)
		}
	}
}

func TestBuildRoutes_InvalidURLReturnsError(t *testing.T) {
	cfg := &config.Config{
		ProjectSvcURL:  "://bad-url",
		DocumentSvcURL: "http://document:8082",
		WorkflowSvcURL: "http://workflow:8083",
		KnowledgeSvcURL: "http://knowledge:8084",
	}

	_, err := buildRoutes(cfg)
	if err == nil {
		t.Fatalf("buildRoutes: expected error for invalid URL, got nil")
	}
}
