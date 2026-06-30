// Package model defines Document domain types.
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// DocumentStatus is the lifecycle state.
type DocumentStatus string

const (
	StatusUploading DocumentStatus = "uploading"
	StatusReady     DocumentStatus = "ready"
	StatusParsing   DocumentStatus = "parsing"
	StatusParsed    DocumentStatus = "parsed"
	StatusFailed    DocumentStatus = "failed"
	StatusDeleted   DocumentStatus = "deleted"
)

// DocumentKind categorizes documents.
type DocumentKind string

const (
	KindTender      DocumentKind = "tender"
	KindProposal    DocumentKind = "proposal"
	KindSpec        DocumentKind = "spec"
	KindAttachment  DocumentKind = "attachment"
	KindReference   DocumentKind = "reference"
)

// ParseStatus is structured progress info during parsing.
type ParseStatus struct {
	Progress   int        `json:"progress"`              // 0-100
	Error      string     `json:"error,omitempty"`
	PageCount  int        `json:"page_count,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// Document is the root aggregate for document-svc.
type Document struct {
	ID            uuid.UUID     `json:"id"             db:"id"`
	TenantID      uuid.UUID     `json:"tenant_id"      db:"tenant_id"`
	ProjectID     uuid.UUID     `json:"project_id"     db:"project_id"`
	Name          string        `json:"name"           db:"name"`
	Kind          DocumentKind  `json:"kind"           db:"kind"`
	MimeType      string        `json:"mime_type"      db:"mime_type"`
	SizeBytes     int64         `json:"size_bytes"     db:"size_bytes"`
	StorageKey    string        `json:"-"              db:"storage_key"`  // never expose
	ChecksumSHA256 string       `json:"checksum_sha256" db:"checksum_sha256"`
	Status        DocumentStatus `json:"status"        db:"status"`
	ParseStatus   *ParseStatus  `json:"parse_status,omitempty" db:"parse_status"`
	Metadata      json.RawMessage `json:"metadata"     db:"metadata"`
	UploadedBy    uuid.UUID     `json:"uploaded_by"    db:"uploaded_by"`
	Version       int           `json:"version"        db:"version"`
	CreatedAt     time.Time     `json:"created_at"     db:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"     db:"updated_at"`
}

// CreateRequest is the API payload for new documents.
type CreateRequest struct {
	ProjectID      uuid.UUID `json:"project_id"      validate:"required"`
	Name           string    `json:"name"            validate:"required,min=1,max=512"`
	Kind           DocumentKind `json:"kind"          validate:"required,oneof=tender proposal spec attachment reference"`
	MimeType       string    `json:"mime_type"       validate:"required,mime"`
	SizeBytes      int64     `json:"size_bytes"      validate:"required,gt=0"`
	ChecksumSHA256 string    `json:"checksum_sha256" validate:"required,hex64"`
	Metadata       json.RawMessage `json:"metadata"  validate:"omitempty"`
}

// UpdateRequest allows partial updates.
type UpdateRequest struct {
	Name        *string          `json:"name"        validate:"omitempty,min=1,max=512"`
	Kind        *DocumentKind    `json:"kind"        validate:"omitempty,oneof=tender proposal spec attachment reference"`
	ParseStatus *ParseStatus     `json:"parse_status"`
	Status      *DocumentStatus  `json:"status"      validate:"omitempty,oneof=uploading ready parsing parsed failed"`
	Metadata    json.RawMessage  `json:"metadata"`
	Version     int              `json:"version"     validate:"required,gte=1"`
}

// ParseRequest is the API payload to trigger RFP parsing.
type ParseRequest struct {
	Async bool `json:"async"` // if true, parse in background and return immediately
}

// ParseResult is the structured result of RFP parsing.
type ParseResult struct {
	// 项目基本信息
	ProjectName  string  `json:"project_name,omitempty"`
	Industry     string  `json:"industry,omitempty"`
	Issuer       string  `json:"issuer,omitempty"`
	BidDeadline  string  `json:"bid_deadline,omitempty"`
	Budget       float64 `json:"budget,omitempty"`
	Currency     string  `json:"currency,omitempty"`

	// 章节结构
	Sections []RFPSection `json:"sections,omitempty"`

	// 评分项
	ScoringItems []ScoringItem `json:"scoring_items,omitempty"`

	// ★号条款（关键要求）
	StarClauses []StarClause `json:"star_clauses,omitempty"`

	// 资质要求
	Qualifications []Qualification `json:"qualifications,omitempty"`

	// 暗标规则
	DarkLabelRules []string `json:"dark_label_rules,omitempty"`

	// 解析元数据
	PageCount    int               `json:"page_count,omitempty"`
	ParsedAt     time.Time         `json:"parsed_at"`
	ParserModel  string            `json:"parser_model,omitempty"`
	RawTextSize  int               `json:"raw_text_size,omitempty"`
	Metadata     json.RawMessage   `json:"metadata,omitempty"`
}

// RFPSection represents a top-level section in the RFP.
type RFPSection struct {
	Title      string        `json:"title"`
	Level      int           `json:"level"` // 1, 2, 3
	PageNumber int           `json:"page_number,omitempty"`
	Children   []RFPSection  `json:"children,omitempty"`
}

// ScoringItem is a scored requirement from the RFP.
type ScoringItem struct {
	ID          string  `json:"id"`                    // e.g. "FR-3.2-A"
	Requirement string  `json:"requirement"`           // 原文要求
	Weight      float64 `json:"weight,omitempty"`      // 权重 0-100
	ChapterHint string  `json:"chapter_hint,omitempty"` // 建议落到的章节
}

// StarClause is a critical (★) requirement that must be fully addressed.
type StarClause struct {
	ID       string `json:"id"`
	Clause   string `json:"clause"`    // ★号条款原文
	PageNum  int    `json:"page_num"`
	Keywords []string `json:"keywords,omitempty"`
}

// Qualification is an eligibility requirement.
type Qualification struct {
	Type        string `json:"type"`         // "certificate", "experience", "team", etc.
	Requirement string `json:"requirement"`
	Verified    bool   `json:"verified"`     // 是否已在知识库中验证
}