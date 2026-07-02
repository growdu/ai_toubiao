package store

import (
	"encoding/json"
	"testing"

	"github.com/bidwriter/services/workflow-svc/internal/model"
)

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

func TestSetPausedFrom_EmptyMetadata_AddsKey(t *testing.T) {
	got := setPausedFrom(nil, model.StateGenerating)
	if string(got) != `{"paused_from":"generating"}` {
		t.Errorf("got %s, want {\"paused_from\":\"generating\"}", got)
	}
}

func TestSetPausedFrom_ExistingMetadata_PreservesOtherKeys(t *testing.T) {
	in := []byte(`{"k":"v","n":1}`)
	got := setPausedFrom(in, model.StateAuditing)
	// JSON object key order is implementation-defined; parse and compare.
	var m map[string]any
	if err := jsonUnmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["paused_from"] != "auditing" {
		t.Errorf("paused_from = %v, want auditing", m["paused_from"])
	}
	if m["k"] != "v" || m["n"] != float64(1) {
		t.Errorf("other keys lost or wrong: %+v", m)
	}
}

func TestSetPausedFrom_GarbageJSON_TreatsAsEmpty(t *testing.T) {
	got := setPausedFrom([]byte("not json"), model.StateFacts)
	if string(got) != `{"paused_from":"facts"}` {
		t.Errorf("got %s, want {\"paused_from\":\"facts\"}", got)
	}
}

func TestClearPausedFrom_RemovesKey(t *testing.T) {
	in := []byte(`{"paused_from":"generating","k":"v"}`)
	got := clearPausedFrom(in)
	var m map[string]any
	if err := jsonUnmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["paused_from"]; ok {
		t.Errorf("paused_from should be removed, got %+v", m)
	}
	if m["k"] != "v" {
		t.Errorf("k lost: %+v", m)
	}
}

func TestClearPausedFrom_NoPausedFrom_IsNoOp(t *testing.T) {
	in := []byte(`{"k":"v"}`)
	got := clearPausedFrom(in)
	if string(got) != `{"k":"v"}` {
		t.Errorf("got %s, want {\"k\":\"v\"}", got)
	}
}

func TestPausedFrom_Empty_ReturnsFalse(t *testing.T) {
	_, ok := pausedFrom(nil)
	if ok {
		t.Error("expected ok=false for empty metadata")
	}
}

func TestPausedFrom_Present_ReturnsValue(t *testing.T) {
	in := []byte(`{"paused_from":"outlining","other":"x"}`)
	got, ok := pausedFrom(in)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != model.StateOutlining {
		t.Errorf("got %s, want outlining", got)
	}
}

func TestPausedFrom_PresentButEmpty_ReturnsFalse(t *testing.T) {
	in := []byte(`{"paused_from":""}`)
	_, ok := pausedFrom(in)
	if ok {
		t.Error("expected ok=false when paused_from is empty string")
	}
}

func TestPausedFrom_NotAString_ReturnsFalse(t *testing.T) {
	in := []byte(`{"paused_from":42}`)
	_, ok := pausedFrom(in)
	if ok {
		t.Error("expected ok=false when paused_from is not a string")
	}
}

func TestEnsureJSONObject_VariousInputs(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want map[string]any
	}{
		{"nil", nil, map[string]any{}},
		{"empty", []byte{}, map[string]any{}},
		{"garbage", []byte("not json"), map[string]any{}},
		{"array", []byte("[1,2]"), map[string]any{}}, // top-level array -> nil
		{"object", []byte(`{"a":1}`), map[string]any{"a": float64(1)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ensureJSONObject(c.in)
			if len(got) != len(c.want) {
				t.Errorf("len = %d, want %d (got %+v)", len(got), len(c.want), got)
				return
			}
			for k, v := range c.want {
				if got[k] != v {
					t.Errorf("[%s] got[%q] = %v, want %v", c.name, k, got[k], v)
				}
			}
		})
	}
}
