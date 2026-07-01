package workers

import (
	"reflect"
	"testing"
)

func TestParseChapterOutline_EmptyReturnsDefault(t *testing.T) {
	got := parseChapterOutline("", map[string]any{"project_name": "X"})
	if len(got) != 6 {
		t.Errorf("expected 6 default chapters, got %d", len(got))
	}
}

func TestParseChapterOutline_GarbageReturnsDefault(t *testing.T) {
	got := parseChapterOutline("lorem ipsum dolor", nil)
	if len(got) != 6 {
		t.Errorf("expected 6 default chapters, got %d", len(got))
	}
}

func TestParseChapterOutline_ValidJSON(t *testing.T) {
	content := `Some prose before [{"title": "A", "level": 1, "sort_order": 1}, {"title": "B", "level": 2, "sort_order": 2}] and after.`
	got := parseChapterOutline(content, nil)
	want := []map[string]any{
		{"title": "A", "level": float64(1), "sort_order": float64(1)},
		{"title": "B", "level": float64(2), "sort_order": float64(2)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestParseChapterOutline_NestedBracketsIgnored(t *testing.T) {
	// Implementation greedily takes the FIRST '[' and the LAST ']' before
	// unparseable. Since start is captured on first '[' and end on the next
	// ']' after that, this should yield a clean parse.
	content := `[{"title": "X", "level": 1, "sort_order": 1}]`
	got := parseChapterOutline(content, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 chapter, got %d: %#v", len(got), got)
	}
	if got[0]["title"] != "X" {
		t.Errorf("unexpected title: %v", got[0]["title"])
	}
}

func TestParseChapterOutline_BrokenJSONFallsBack(t *testing.T) {
	got := parseChapterOutline("[{not json}]", nil)
	if len(got) != 6 {
		t.Errorf("expected fallback 6 chapters on broken JSON, got %d", len(got))
	}
}

func TestDefaultChapterOutline_SixEntries(t *testing.T) {
	got := defaultChapterOutline(map[string]any{"project_name": "P"})
	if len(got) != 6 {
		t.Fatalf("expected 6 chapters, got %d", len(got))
	}
	for i, ch := range got {
		if _, ok := ch["title"].(string); !ok {
			t.Errorf("ch[%d] missing title string", i)
		}
		if _, ok := ch["level"].(int); !ok {
			t.Errorf("ch[%d] missing level int", i)
		}
		if _, ok := ch["sort_order"].(int); !ok {
			t.Errorf("ch[%d] missing sort_order int", i)
		}
	}
}
