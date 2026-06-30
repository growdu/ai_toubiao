package router_test

import (
	"testing"

	"github.com/bidwriter/services/router-svc/internal/router"
)

func TestDefaultRoutes_Valid(t *testing.T) {
	cfg := router.DefaultRoutes()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default routes invalid: %v", err)
	}
	if cfg.GetRoute("rfp_parse") == nil {
		t.Error("expected rfp_parse route")
	}
	if cfg.GetRoute("audit_agent") == nil {
		t.Error("expected audit_agent route")
	}
	if cfg.GetRoute("audit_agent").Fallback != nil && len(cfg.GetRoute("audit_agent").Fallback) != 0 {
		t.Error("audit_agent should have no fallback by design")
	}
}

func TestParseRoutesConfig_Valid(t *testing.T) {
	yaml := []byte(`
version: 1
routes:
  - task: foo
    primary: {provider: mock, model: m1}
    fallback:
      - {provider: mock, model: m2}
    budget: {max_input_tokens: 1000, max_cost_usd: 0.10, timeout_seconds: 30}
    cache: {enabled: true, ttl_seconds: 3600}
`)
	cfg, err := router.ParseRoutesConfig(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.GetRoute("foo") == nil {
		t.Error("expected foo route")
	}
	if cfg.GetRoute("foo").Cache.TTL().Seconds() != 3600 {
		t.Errorf("ttl: %v", cfg.GetRoute("foo").Cache.TTL())
	}
}

func TestParseRoutesConfig_DuplicateTask(t *testing.T) {
	yaml := []byte(`
version: 1
routes:
  - task: dup
    primary: {provider: mock, model: m1}
  - task: dup
    primary: {provider: mock, model: m2}
`)
	_, err := router.ParseRoutesConfig(yaml)
	if err == nil {
		t.Error("expected duplicate task error")
	}
}

func TestParseRoutesConfig_MissingPrimary(t *testing.T) {
	yaml := []byte(`
version: 1
routes:
  - task: bad
`)
	_, err := router.ParseRoutesConfig(yaml)
	if err == nil {
		t.Error("expected missing primary error")
	}
}

func TestRoutesConfig_Tasks(t *testing.T) {
	cfg := router.DefaultRoutes()
	tasks := cfg.Tasks()
	if len(tasks) == 0 {
		t.Error("expected non-empty tasks")
	}
	found := map[string]bool{}
	for _, t := range tasks {
		found[t] = true
	}
	for _, want := range []string{"rfp_parse", "outline_generate", "content_generate", "audit_agent"} {
		if !found[want] {
			t.Errorf("missing task %q", want)
		}
	}
}