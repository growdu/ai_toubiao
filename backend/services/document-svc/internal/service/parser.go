// Package service implements business logic for document-svc.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/bidwriter/services/document-svc/internal/model"
	"github.com/bidwriter/services/document-svc/internal/storage"
	"github.com/bidwriter/services/document-svc/internal/store"
	"github.com/google/uuid"
)

// ParserService handles RFP document parsing.
type ParserService struct {
	store        *store.Store
	storage      storage.Storage
	log          *slog.Logger
	routerClient *RouterClient
}

// NewParserService creates a ParserService.
func NewParserService(s *store.Store, st storage.Storage, log *slog.Logger, routerURL string) *ParserService {
	return &ParserService{store: s, storage: st, log: log, routerClient: NewRouterClient(routerURL)}
}

// Parse triggers parsing of a document and returns the structured result.
// For async=true, it returns immediately after updating status.
// For async=false, it blocks until parsing completes.
func (p *ParserService) Parse(ctx context.Context, docID uuid.UUID, async bool) (*model.ParseResult, error) {
	// TODO: In Phase 1.2, this will call the actual parser (PyMuPDF/unioffice + LLM).
	// For now, return a mock result so the API surface is complete.
	p.log.Info("parse requested", slog.String("doc_id", docID.String()), slog.Bool("async", async))

	// Update document status to parsing
	now := time.Now()
	ps := &model.ParseStatus{
		Progress:  10,
		StartedAt: &now,
	}
	updateReq := &model.UpdateRequest{
		Status:      statusPtr(model.StatusParsing),
		ParseStatus: ps,
		Version:     1, // TODO: fetch current version
	}
	_, err := p.store.Update(ctx, docID, updateReq)
	if err != nil {
		return nil, fmt.Errorf("update status to parsing: %w", err)
	}

	if async {
		// TODO: enqueue to Asynq queue for background processing
		return nil, nil
	}

	// Synchronous parsing (dev mode placeholder)
	result, err := p.doParse(ctx, docID)
	if err != nil {
		finished := time.Now()
		p.store.Update(ctx, docID, &model.UpdateRequest{
			Status:      statusPtr(model.StatusFailed),
			ParseStatus: &model.ParseStatus{Progress: 0, Error: err.Error(), FinishedAt: &finished},
			Version:     2, // TODO
		})
		return nil, err
	}

	// Update document with parse result in metadata
	metadata, _ := json.Marshal(map[string]any{"parse_result": result})
	finished := time.Now()
	p.store.Update(ctx, docID, &model.UpdateRequest{
		Status:      statusPtr(model.StatusParsed),
		ParseStatus: &model.ParseStatus{Progress: 100, FinishedAt: &finished},
		Metadata:    metadata,
		Version:     2, // TODO
	})

	return result, nil
}

// doParse performs the actual parsing: extract text + LLM structured extraction.
func (p *ParserService) doParse(ctx context.Context, docID uuid.UUID) (*model.ParseResult, error) {
	// 1. Fetch document to get storage key
	doc, err := p.store.Get(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}

	// 2. Read file content from storage
	rc, err := p.storage.Get(ctx, doc.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("storage get: %w", err)
	}
	content, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}

	// 3. Route to appropriate parser based on mime type
	var text string
	switch {
	case strings.HasPrefix(doc.MimeType, "text/"):
		text = string(content)
	case strings.Contains(doc.MimeType, "pdf"):
		text = p.extractTextFromPDF(content)
	case strings.Contains(doc.MimeType, "word") || strings.Contains(doc.MimeType, "document"):
		text = p.extractTextFromWord(content)
	default:
		text = string(content)
	}

	p.log.Info("text extracted", slog.String("doc_id", docID.String()), slog.Int("text_size", len(text)))

	// 4. Call LLM to extract structured information from text.
	result, err := p.extractStructFromLLM(ctx, doc.TenantID, text)
	if err != nil {
		p.log.Warn("LLM extraction failed, returning partial result", slog.Any("error", err))
		// Return partial result with at least basic info.
		result = &model.ParseResult{
			RawTextSize: len(text),
			ParsedAt:    time.Now(),
		}
	}

	result.RawTextSize = len(text)
	p.log.Info("parse complete", slog.String("doc_id", docID.String()), slog.Int("text_size", len(text)))
	return result, nil
}

// extractStructFromLLM calls the LLM to extract structured RFP information.
func (p *ParserService) extractStructFromLLM(ctx context.Context, tenantID uuid.UUID, text string) (*model.ParseResult, error) {
	if p.routerClient == nil {
		return &model.ParseResult{ParsedAt: time.Now()}, nil
	}

	// Truncate text if too long (LLM context limit). Use first 8000 chars.
	truncated := text
	if len(text) > 8000 {
		truncated = text[:8000]
	}

	prompt := `你是一个专业的标书解析助手。请从以下招标文档文本中提取结构化信息，以JSON格式返回。

提取以下字段：
- project_name: 项目名称
- industry: 所属行业
- issuer: 招标方/发包方
- bid_deadline: 投标截止时间
- budget: 预算金额（数字）
- currency: 货币单位（如CNY、USD）
- sections: 章节结构数组，每项含title(标题)、level(层级1-3)、page_number(页码)
- scoring_items: 评分项数组，每项含id、requirement(要求原文)、weight(权重0-100)、chapter_hint(建议章节)
- star_clauses: ★号条款数组，每项含id、clause(条款原文)、page_num(页码)、keywords(关键词)
- qualifications: 资质要求数组，每项含type(类型)、requirement(要求描述)
- dark_label_rules: 暗标规则字符串数组（检测到的匿名化要求）

请只返回JSON，不要有其他文字。`

	messages := []Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: truncated},
	}

	resp, err := p.routerClient.Chat(ctx, tenantID, "rfp_parse", messages, 4096)
	if err != nil {
		return nil, fmt.Errorf("router call: %w", err)
	}

	// Try to extract JSON from response.
	jsonStr := extractJSONFromResponse(resp.Content)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in LLM response")
	}

	var result model.ParseResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parse LLM JSON: %w", err)
	}
	result.ParsedAt = time.Now()
	result.ParserModel = resp.Model

	// Detect dark bid risk.
	darkRules := detectDarkBidRules(text)
	if len(darkRules) > 0 {
		result.DarkLabelRules = darkRules
	}

	return &result, nil
}

// extractJSONFromResponse extracts the first JSON object/array from LLM text.
func extractJSONFromResponse(s string) string {
	// Try direct unmarshal first.
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return s
	}
	// Find first { or [ and last } or ].
	start := -1
	end := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '{' && start < 0 {
			start = i
		}
		if s[i] == '}' && start >= 0 {
			end = i + 1
			break
		}
	}
	if start >= 0 && end > start {
		return s[start:end]
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '[' && start < 0 {
			start = i
		}
		if s[i] == ']' && start >= 0 {
			end = i + 1
			break
		}
	}
	if start >= 0 && end > start {
		return s[start:end]
	}
	return ""
}

// detectDarkBidRules scans text for common dark-bid risk patterns.
// Returns a list of detected rule descriptions.
func detectDarkBidRules(text string) []string {
	var rules []string
	lower := strings.ToLower(text)

	darkPatterns := []struct {
		pattern string
		label   string
	}{
		{"联系", "文本中可能包含联系方式，违反暗标规则"},
		{"投标单位", "出现'投标单位'可能泄露投标人身份"},
		{"公司地址", "出现'公司地址'可能泄露投标人身份"},
		{"联系人", "出现'联系人'可能泄露投标人身份"},
		{"联系电话", "出现'联系电话'可能泄露投标人身份"},
		{"公司名称", "出现'公司名称'可能泄露投标人身份"},
	}

	for _, dp := range darkPatterns {
		if strings.Contains(lower, dp.pattern) {
			rules = append(rules, dp.label)
		}
	}
	return rules
}

// extractTextFromPDF extracts text from PDF bytes.
// Tries pdftotext (poppler-utils) first for accurate extraction, then
// falls back to the built-in regex extractor.
func (p *ParserService) extractTextFromPDF(content []byte) string {
	text := extractPDFTextRobust(context.Background(), content)
	if text == "" {
		return string(content)
	}
	return text
}

// extractPDFText tries to extract readable text from PDF content streams.
func extractPDFText(data []byte) string {
	// Match text between BT (Begin Text) and ET (End Text) markers.
	// This is a simplified PDF text extraction — real PDFs use Tj, TJ, ' operators.
	var buf strings.Builder
	textRe := regexp.MustCompile(`BT\s*([\s\S]*?)\s*ET`)
	matches := textRe.FindAllSubmatch(data, -1)
	for _, m := range matches {
		textBlock := string(m[1])
		// Extract string literals in parentheses: (text content)
		strRe := regexp.MustCompile(`\(([^)]*)\)`)
		strMatches := strRe.FindAllStringSubmatch(textBlock, -1)
		for _, sm := range strMatches {
			t := sm[1]
			// Unescape common PDF escape sequences.
			t = strings.ReplaceAll(t, `\\`, `\`)
			t = strings.ReplaceAll(t, `\)`, ")")
			t = strings.ReplaceAll(t, `\n`, "\n")
			t = strings.ReplaceAll(t, `\r`, "\r")
			t = strings.ReplaceAll(t, `\t`, "\t")
			if strings.TrimSpace(t) != "" {
				buf.WriteString(t)
				buf.WriteString(" ")
			}
		}
	}
	return strings.TrimSpace(buf.String())
}

// extractTextFromWord extracts text from .docx bytes using paragraph-aware
// OOXML parsing for better structure preservation.
func (p *ParserService) extractTextFromWord(content []byte) string {
	return extractDOCXText(content)
}

// GetParseResult retrieves the stored parse result for a document.
func (p *ParserService) GetParseResult(ctx context.Context, docID uuid.UUID) (*model.ParseResult, error) {
	doc, err := p.store.Get(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}

	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(doc.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	resultData, ok := metadata["parse_result"]
	if !ok {
		return nil, fmt.Errorf("parse result not found")
	}

	var result model.ParseResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return nil, fmt.Errorf("unmarshal parse result: %w", err)
	}

	return &result, nil
}

// statusPtr returns a pointer to a DocumentStatus (helper for UpdateRequest).
func statusPtr(s model.DocumentStatus) *model.DocumentStatus {
	return &s
}
