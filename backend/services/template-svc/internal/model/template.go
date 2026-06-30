package model

import (
	"time"

	"github.com/google/uuid"
)

// WordTemplate represents a Word document template.
type WordTemplate struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	TenantID    uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	Name        string     `json:"name" db:"name"`
	Description string     `json:"description" db:"description"`
	Kind        string     `json:"kind" db:"kind"` // "standard", "technical", "commercial"
	StorageKey  string     `json:"storage_key" db:"storage_key"`
	SizeBytes   int64      `json:"size_bytes" db:"size_bytes"`
	Checksum    string     `json:"checksum" db:"checksum"`
	Version     int        `json:"version" db:"version"`
	IsDefault   bool       `json:"is_default" db:"is_default"`
	CreatedBy   uuid.UUID  `json:"created_by" db:"created_by"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// CreateRequest for uploading/creating a template.
type CreateRequest struct {
	Name        string `json:"name" validate:"required,max=200"`
	Description string `json:"description" validate:"max=1000"`
	Kind        string `json:"kind" validate:"required,oneof=standard technical commercial"`
	IsDefault   bool   `json:"is_default"`
}

// UpdateRequest for updating template metadata.
type UpdateRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,max=200"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=1000"`
	IsDefault   *bool   `json:"is_default,omitempty"`
}
