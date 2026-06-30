// Package model defines domain types for knowledge-svc.
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// KBMaterial represents a knowledge base material.
type KBMaterial struct {
	ID        uuid.UUID       `json:"id" db:"id"`
	TenantID  uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	Category  string          `json:"category" db:"category"`
	Title     string          `json:"title" db:"title"`
	Summary   string          `json:"summary,omitempty" db:"summary"`
	Content   string          `json:"content,omitempty" db:"content"`
	FilePath  string          `json:"file_path,omitempty" db:"file_path"`
	FileSize  int64           `json:"file_size,omitempty" db:"file_size"`
	MimeType  string          `json:"mime_type,omitempty" db:"mime_type"`
	Status    string          `json:"status" db:"status"`
	Metadata  json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	ChunkCount int            `json:"chunk_count" db:"chunk_count"`
	IndexedAt *time.Time      `json:"indexed_at,omitempty" db:"indexed_at"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// KBChunk represents a chunk of a knowledge base material.
type KBChunk struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	MaterialID   uuid.UUID       `json:"material_id" db:"material_id"`
	TenantID     uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	Content      string          `json:"content" db:"content"`
	ChunkIndex   int             `json:"chunk_index" db:"chunk_index"`
	CharStart    int             `json:"char_start,omitempty" db:"char_start"`
	CharEnd      int             `json:"char_end,omitempty" db:"char_end"`
	SourceLocation string        `json:"source_location,omitempty" db:"source_location"`
	HitCount     int             `json:"hit_count" db:"hit_count"`
	UsedCount    int             `json:"used_count" db:"used_count"`
	Metadata     json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}

// KBSearchResult is the result of a semantic search.
type KBSearchResult struct {
	ChunkID       uuid.UUID `json:"chunk_id"`
	MaterialID    uuid.UUID `json:"material_id"`
	MaterialTitle string    `json:"material_title"`
	Content       string    `json:"content"`
	Score         float64   `json:"score"`
	ChunkIndex    int       `json:"chunk_index"`
}

// SearchRequest is the search API request.
type SearchRequest struct {
	Query    string `json:"query" validate:"required,min=1"`
	TopK     int    `json:"top_k" validate:"omitempty,gt=0,lte=50"`
	Category string `json:"category,omitempty"`
}

// SearchResponse is the search API response.
type SearchResponse struct {
	Hits  []KBSearchResult `json:"hits"`
	Total int              `json:"total"`
}

// CreateMaterialRequest is the request to create a KB material.
type CreateMaterialRequest struct {
	Category string `json:"category" validate:"required,oneof=certificate case patent team equipment qualification other"`
	Title    string `json:"title" validate:"required,min=1,max=512"`
	Content  string `json:"content,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// IngestRequest triggers re-indexing of a material.
type IngestRequest struct {
	MaterialID uuid.UUID `json:"material_id" validate:"required"`
	Force      bool      `json:"force"` // re-index even if already indexed
}