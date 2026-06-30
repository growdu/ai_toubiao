// Package service implements business logic for document-svc.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/bidwriter/services/document-svc/internal/model"
	"github.com/bidwriter/services/document-svc/internal/storage"
	"github.com/bidwriter/services/document-svc/internal/store"
	"github.com/google/uuid"
)

// ParserService handles RFP document parsing.
type ParserService struct {
	store   *store.Store
	storage storage.Storage
	log     *slog.Logger
}

// NewParserService creates a ParserService.
func NewParserService(s *store.Store, st storage.Storage, log *slog.Logger) *ParserService {
	return &ParserService{store: s, storage: st, log: log}
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

// doParse performs the actual parsing (placeholder implementation).
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
	defer rc.Close()

	content, err := io.ReadAll(rc)
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

	// 4. TODO: Call LLM to extract structured information
	// For now, return a placeholder result
	result := &model.ParseResult{
		ProjectName: "示例项目",
		Industry:    "工程建设",
		Budget:      1000000.00,
		Currency:    "CNY",
		ParsedAt:    time.Now(),
		ParserModel: "claude-sonnet-4-20250514",
		RawTextSize: len(text),
		Sections: []model.RFPSection{
			{Title: "第一章 总则", Level: 1, PageNumber: 1},
			{Title: "第二章 招标内容", Level: 1, PageNumber: 3},
			{Title: "第三章 投标人资格", Level: 1, PageNumber: 5},
		},
		ScoringItems: []model.ScoringItem{
			{ID: "FR-3.2-A", Requirement: "投标人须具有建筑工程施工总承包一级资质", Weight: 10},
			{ID: "FR-3.2-B", Requirement: "近三年内具有类似项目经验", Weight: 15},
		},
		StarClauses: []model.StarClause{
			{ID: "SC-1", Clause: "★投标人必须具有ISO9001质量管理体系认证", PageNum: 3, Keywords: []string{"ISO9001", "认证"}},
			{ID: "SC-2", Clause: "★项目负责人须具有一级建造师执业资格", PageNum: 4, Keywords: []string{"一级建造师"}},
		},
		Qualifications: []model.Qualification{
			{Type: "certificate", Requirement: "营业执照", Verified: false},
			{Type: "certificate", Requirement: "安全生产许可证", Verified: false},
		},
		Metadata: json.RawMessage(`{}`),
	}

	p.log.Info("parse complete", slog.String("doc_id", docID.String()), slog.Int("text_size", len(text)))
	return result, nil
}

// extractTextFromPDF extracts text from PDF bytes (placeholder - real impl uses PyMuPDF).
func (p *ParserService) extractTextFromPDF(content []byte) string {
	// TODO: Use PyMuPDF (github.com/geniusina/pymupdf)
	return string(content)
}

// extractTextFromWord extracts text from Word bytes (placeholder - real impl uses unioffice).
func (p *ParserService) extractTextFromWord(content []byte) string {
	// TODO: Use unioffice (github.com/unioffice/unioffice)
	return string(content)
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