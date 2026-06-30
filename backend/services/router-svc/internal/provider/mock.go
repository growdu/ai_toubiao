package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bidwriter/services/router-svc/internal/model"
)

// MockProvider is a deterministic provider used for unit tests and dev mode.
// It echoes back the last user message and tracks call counts for assertions.
type MockProvider struct {
	mu              sync.Mutex
	calls           int
	respondWith     string
	latency         time.Duration
	costPerCall     float64
	healthShouldErr bool
}

// NewMockProvider builds a mock with sensible defaults.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		respondWith: "mock-response",
		latency:     5 * time.Millisecond,
		costPerCall: 0.0001,
	}
}

// WithResponse sets the canned response (for testing prompt flows).
func (m *MockProvider) WithResponse(s string) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.respondWith = s
	return m
}

// WithLatency overrides the simulated latency.
func (m *MockProvider) WithLatency(d time.Duration) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latency = d
	return m
}

// WithCostPerCall sets the synthetic cost returned to the caller.
func (m *MockProvider) WithCostPerCall(c float64) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.costPerCall = c
	return m
}

// Calls returns the number of Chat invocations observed.
func (m *MockProvider) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// Name implements Provider.
func (m *MockProvider) Name() string { return "mock" }

// Chat implements Provider.
func (m *MockProvider) Chat(ctx context.Context, in ChatInput) (*ChatOutput, error) {
	m.mu.Lock()
	m.calls++
	resp := m.respondWith
	latency := m.latency
	cost := m.costPerCall
	m.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(latency):
	}

	promptTokens := estimateTokens(in.Messages)
	completionTokens := estimateTokens([]model.Message{{Role: "assistant", Content: resp}})

	return &ChatOutput{
		Content:          resp,
		Model:            in.Model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		CostUSD:          cost,
	}, nil
}

// EstimateCost implements Provider.
func (m *MockProvider) EstimateCost(in ChatInput) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.costPerCall
}

// HealthCheck implements Provider.
func (m *MockProvider) HealthCheck(ctx context.Context) error {
	m.mu.Lock()
	err := m.healthShouldErr
	m.mu.Unlock()
	if err {
		return fmt.Errorf("mock: health check failing")
	}
	return nil
}

// estimateTokens uses a 4-chars-per-token heuristic. Good enough for budget
// pre-checks; the real provider returns the authoritative count.
func estimateTokens(msgs []model.Message) int {
	total := 0
	for _, m := range msgs {
		total += len(m.Content) / 4
		if total == 0 && len(m.Content) > 0 {
			total = 1
		}
	}
	return total
}

// ExtractJSON attempts three increasingly aggressive strategies to recover a
// JSON document from a possibly-broken LLM response.
//
//  1. Direct unmarshal.
//  2. Strip JS-style comments, single quotes, trailing commas, and retry.
//  3. Extract the largest {...} or [...] substring and unmarshal that.
//
// It returns the parsed value or an error describing the last failure.
func ExtractJSON(raw string) (any, error) {
	// 1. direct
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v, nil
	}

	// 2. fix common errors
	fixed := fixCommonJSONErrors(raw)
	if err := json.Unmarshal([]byte(fixed), &v); err == nil {
		return v, nil
	}

	// 3. extract substring
	extracted := extractJSONSubstring(raw)
	if extracted != "" && extracted != raw {
		if err := json.Unmarshal([]byte(extracted), &v); err == nil {
			return v, nil
		}
		// also try the fixed version of the extracted substring
		fixed2 := fixCommonJSONErrors(extracted)
		if err := json.Unmarshal([]byte(fixed2), &v); err == nil {
			return v, nil
		}
	}

	return nil, fmt.Errorf("could not extract valid JSON from response (%d bytes)", len(raw))
}

func fixCommonJSONErrors(s string) string {
	// Strip JS-style comments — naive but good enough for LLM outputs.
	s = stripLineComments(s)
	s = stripBlockComments(s)
	// Replace single-quoted strings with double-quoted ones, but only outside
	// of double-quoted strings (best-effort).
	s = replaceSingleQuoted(s)
	// Strip trailing commas in objects/arrays.
	s = stripTrailingCommas(s)
	// Collapse embedded newlines inside strings.
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func stripLineComments(s string) string {
	var b strings.Builder
	inStr := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			b.WriteByte(c)
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '/' {
			// skip to end of line
			for i < len(s) && s[i] != '\n' {
				i++
			}
			if i < len(s) {
				b.WriteByte('\n')
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func stripBlockComments(s string) string {
	var b strings.Builder
	inStr := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			b.WriteByte(c)
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			i++ // skip closing '/'
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// replaceSingleQuoted converts JS-style single-quoted strings to JSON
// double-quoted ones. The strategy is two-pass:
//  1. Find each `'` (outside double quotes) that delimits a string literal.
//  2. Replace the quote pair with `"`, escaping embedded `'` and `\`.
//
// This is deliberately conservative — when uncertain, it leaves the input alone.
func replaceSingleQuoted(s string) string {
	out := make([]byte, 0, len(s))
	inDouble := false
	i := 0
	for i < len(s) {
		c := s[i]
		if inDouble {
			// copy until next unescaped double quote
			out = append(out, c)
			if c == '\\' && i+1 < len(s) {
				out = append(out, s[i+1])
				i += 2
				continue
			}
			if c == '"' {
				inDouble = false
			}
			i++
			continue
		}
		// outside double quotes
		if c == '"' {
			inDouble = true
			out = append(out, c)
			i++
			continue
		}
		if c != '\'' {
			out = append(out, c)
			i++
			continue
		}
		// candidate single-quoted string: find the closing `'`
		j := i + 1
		for j < len(s) {
			if s[j] == '\\' && j+1 < len(s) {
				j += 2
				continue
			}
			if s[j] == '\'' {
				break
			}
			j++
		}
		if j >= len(s) {
			// unterminated; copy as-is
			out = append(out, c)
			i++
			continue
		}
		// Look at what comes after the closing quote: must be a JSON delimiter
		// (whitespace, comma, colon, brace, bracket, end-of-input). Otherwise we
		// are likely looking at an apostrophe in prose (e.g. "don't"); bail.
		k := j + 1
		if k < len(s) {
			nc := s[k]
			switch nc {
			case ' ', '\t', '\n', '\r', ',', ':', '}', ']', '[', '{':
				// OK, treat as string boundary
			default:
				out = append(out, '\'')
				i++
				continue
			}
		}
		// Also check that what precedes the opening quote is plausible.
		if i > 0 {
			prev := s[i-1]
			switch prev {
			case ' ', '\t', '\n', '\r', ',', ':', '{', '[':
				// OK
			default:
				out = append(out, '\'')
				i++
				continue
			}
		}
		// Convert: emit ", copy content escaping, emit "
		out = append(out, '"')
		inner := s[i+1 : j]
		for idx := 0; idx < len(inner); idx++ {
			ch := inner[idx]
			if ch == '\\' && idx+1 < len(inner) {
				next := inner[idx+1]
				switch next {
				case '\'':
					out = append(out, '"')
				case '"':
					out = append(out, '\\', '"')
				case '\\':
					out = append(out, '\\', '\\')
				case 'n':
					out = append(out, '\\', 'n')
				case 't':
					out = append(out, '\\', 't')
				default:
					out = append(out, '\\', next)
				}
				idx++
				continue
			}
			if ch == '"' {
				out = append(out, '\\', '"')
				continue
			}
			out = append(out, ch)
		}
		out = append(out, '"')
		i = j + 1
	}
	return string(out)
}

func stripTrailingCommas(s string) string {
	var b strings.Builder
	inStr := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			b.WriteByte(c)
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			b.WriteByte(c)
			continue
		}
		if c == ',' {
			// peek ahead for whitespace then closing brace/bracket
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j < len(s) && (s[j] == '}' || s[j] == ']') {
				// skip the comma
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// extractJSONSubstring returns the largest balanced {...} or [...] block from s.
// If none is found it returns "".
func extractJSONSubstring(s string) string {
	openers := map[byte]byte{'{': '}', '[': ']'}

	bestStart := -1
	bestEnd := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '{' && c != '[' {
			continue
		}
		// find matching close
		matched, end := matchBrace(s, i, openers[c])
		if matched && (bestStart == -1 || end-i > bestEnd-bestStart) {
			bestStart = i
			bestEnd = end
		}
	}
	if bestStart < 0 {
		return ""
	}
	return s[bestStart : bestEnd+1]
}

func matchBrace(s string, start int, closer byte) (bool, int) {
	pairs := map[byte]byte{'{': '}', '[': ']'}
	closers := map[byte]bool{'}': true, ']': true}
	depth := 0
	inStr := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			continue
		}
		if _, ok := pairs[c]; ok {
			depth++
			continue
		}
		if closers[c] {
			depth--
			if depth == 0 && c == closer {
				return true, i
			}
			if depth < 0 {
				return false, i
			}
		}
	}
	return false, -1
}