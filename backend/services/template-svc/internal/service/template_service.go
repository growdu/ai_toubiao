package service

import (
	"context"
	"io"

	"github.com/bidwriter/services/template-svc/internal/model"
	"github.com/bidwriter/services/template-svc/internal/storage"
	"github.com/bidwriter/services/template-svc/internal/store"
	"github.com/google/uuid"
)

// TemplateService handles template business logic.
type TemplateService struct {
	store   *store.Store
	storage storage.Storage
}

func NewTemplateService(s *store.Store, stg storage.Storage) *TemplateService {
	return &TemplateService{store: s, storage: stg}
}

// Upload handles uploading a new template.
func (s *TemplateService) Upload(ctx context.Context, userID uuid.UUID, req *model.CreateRequest, file io.Reader, filename string, size int64) (*model.WordTemplate, error) {
	// Upload file to storage
	key, checksum, _, err := s.storage.Put(ctx, filename, file)
	if err != nil {
		return nil, err
	}

	t := &model.WordTemplate{
		Name:        req.Name,
		Description: req.Description,
		Kind:        req.Kind,
		StorageKey:  key,
		SizeBytes:   size,
		Checksum:    checksum,
		IsDefault:   req.IsDefault,
		CreatedBy:   userID,
	}

	// If this is the new default, clear other defaults first
	if req.IsDefault {
		if err := s.store.ClearDefault(ctx, req.Kind); err != nil {
			_ = s.storage.Delete(ctx, key)
			return nil, err
		}
	}

	if err := s.store.Create(ctx, t); err != nil {
		_ = s.storage.Delete(ctx, key)
		return nil, err
	}
	return t, nil
}

// Download returns the template file content.
func (s *TemplateService) Download(ctx context.Context, id uuid.UUID) (io.ReadCloser, *model.WordTemplate, error) {
	t, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	rc, err := s.storage.Get(ctx, t.StorageKey)
	if err != nil {
		return nil, nil, err
	}
	return rc, t, nil
}

// List returns all templates for the current tenant.
func (s *TemplateService) List(ctx context.Context) ([]*model.WordTemplate, error) {
	return s.store.List(ctx)
}

// Get returns a template by ID.
func (s *TemplateService) Get(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error) {
	return s.store.Get(ctx, id)
}

// Delete removes a template and its file.
func (s *TemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	t, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.storage.Delete(ctx, t.StorageKey)
	return nil
}

// Update updates template metadata.
func (s *TemplateService) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.WordTemplate, error) {
	if req.IsDefault != nil && *req.IsDefault {
		t, err := s.store.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := s.store.ClearDefault(ctx, t.Kind); err != nil {
			return nil, err
		}
	}
	return s.store.Update(ctx, id, req)
}
