// Package model defines Project domain types.
package model

import (
	"time"

	"github.com/google/uuid"
)

// ProjectStatus is the lifecycle state.
type ProjectStatus string

const (
	StatusDraft     ProjectStatus = "draft"
	StatusActive    ProjectStatus = "active"
	StatusCompleted ProjectStatus = "completed"
	StatusArchived  ProjectStatus = "archived"
)

// Project is the root aggregate. Matches docs/architecture/data-model.md.
type Project struct {
	ID             uuid.UUID    `json:"id"             db:"id"`
	TenantID       uuid.UUID    `json:"tenant_id"      db:"tenant_id"`
	Name           string       `json:"name"           db:"name"`
	Description    *string      `json:"description"    db:"description"`
	Industry       *string      `json:"industry"       db:"industry"`
	TemplateID     *uuid.UUID   `json:"template_id"    db:"template_id"`
	Status         ProjectStatus `json:"status"        db:"status"`
	EstimatedValue *float64     `json:"estimated_value" db:"estimated_value"`
	Currency       string       `json:"currency"       db:"currency"`
	Deadline       *time.Time   `json:"deadline"       db:"deadline"`
	OwnerID        uuid.UUID    `json:"owner_id"       db:"owner_id"`
	Version        int          `json:"version"        db:"version"`
	CreatedAt      time.Time    `json:"created_at"     db:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"     db:"updated_at"`
}

// CreateRequest is the API payload.
type CreateRequest struct {
	Name           string     `json:"name"            validate:"required,min=1,max=256"`
	Description    *string    `json:"description"     validate:"omitempty,max=2000"`
	Industry       *string    `json:"industry"        validate:"omitempty,max=64"`
	TemplateID     *uuid.UUID `json:"template_id"`
	EstimatedValue *float64   `json:"estimated_value" validate:"omitempty,gte=0"`
	Currency       string     `json:"currency"        validate:"omitempty,len=3"`
	Deadline       *time.Time `json:"deadline"`
}

// UpdateRequest allows partial updates.
type UpdateRequest struct {
	Name           *string    `json:"name"            validate:"omitempty,min=1,max=256"`
	Description    *string    `json:"description"     validate:"omitempty,max=2000"`
	Industry       *string    `json:"industry"        validate:"omitempty,max=64"`
	Status         *ProjectStatus `json:"status"      validate:"omitempty,oneof=draft active completed archived"`
	EstimatedValue *float64   `json:"estimated_value" validate:"omitempty,gte=0"`
	Currency       *string    `json:"currency"        validate:"omitempty,len=3"`
	Deadline       *time.Time `json:"deadline"`
	Version        int        `json:"version"         validate:"required,gte=1"`
}