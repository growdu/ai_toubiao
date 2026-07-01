package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bidwriter/services/template-svc/internal/model"
	"github.com/bidwriter/services/template-svc/internal/storage"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
)

// ---- fakes ----

type fakeStore struct {
	createFn      func(ctx context.Context, t *model.WordTemplate) error
	getFn         func(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error)
	listFn        func(ctx context.Context) ([]*model.WordTemplate, error)
	updateFn      func(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.WordTemplate, error)
	deleteFn      func(ctx context.Context, id uuid.UUID) error
	clearDefaultFn func(ctx context.Context, kind string) error

	createCalls      int
	clearDefaultCalls int
	getCalls         int
}

func (f *fakeStore) Create(ctx context.Context, t *model.WordTemplate) error {
	f.createCalls++
	if f.createFn != nil {
		return f.createFn(ctx, t)
	}
	return nil
}
func (f *fakeStore) Get(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error) {
	f.getCalls++
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return nil, storeErrNotFound
}
func (f *fakeStore) List(ctx context.Context) ([]*model.WordTemplate, error) {
	if f.listFn != nil {
		return f.listFn(ctx)
	}
	return nil, nil
}
func (f *fakeStore) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.WordTemplate, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, id, req)
	}
	return nil, nil
}
func (f *fakeStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return nil
}
func (f *fakeStore) ClearDefault(ctx context.Context, kind string) error {
	f.clearDefaultCalls++
	if f.clearDefaultFn != nil {
		return f.clearDefaultFn(ctx, kind)
	}
	return nil
}

type fakeStorage struct {
	putFn    func(ctx context.Context, name string, r io.Reader) (string, string, int64, error)
	getFn    func(ctx context.Context, key string) (io.ReadCloser, error)
	deleteFn func(ctx context.Context, key string) error

	putCalls    int
	getCalls    int
	deleteCalls int
	lastDeleted string
}

func (f *fakeStorage) Put(ctx context.Context, name string, r io.Reader) (string, string, int64, error) {
	f.putCalls++
	if f.putFn != nil {
		return f.putFn(ctx, name, r)
	}
	return "key-" + name, "deadbeef", 0, nil
}
func (f *fakeStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	f.getCalls++
	if f.getFn != nil {
		return f.getFn(ctx, key)
	}
	return io.NopCloser(strings.NewReader("")), nil
}
func (f *fakeStorage) Delete(ctx context.Context, key string) error {
	f.deleteCalls++
	f.lastDeleted = key
	if f.deleteFn != nil {
		return f.deleteFn(ctx, key)
	}
	return nil
}

var storeErrNotFound = errors.New("not found")

// ---- helpers ----

func ctxWithTenant(t *testing.T) context.Context {
	t.Helper()
	tid := uuid.NewString()
	return tenant.WithTenant(context.Background(), tid)
}

// ---- tests ----

func TestUpload_HappyPath_NotDefault(t *testing.T) {
	fs := &fakeStore{}
	fst := &fakeStorage{}
	svc := NewTemplateService(fs, fst)

	tpl, err := svc.Upload(ctxWithTenant(t), uuid.New(),
		&model.CreateRequest{Name: "Standard", Kind: "standard"},
		bytes.NewReader([]byte("hello")), "hello.docx", 5)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if tpl.Name != "Standard" || tpl.Kind != "standard" {
		t.Errorf("template fields wrong: %+v", tpl)
	}
	if tpl.StorageKey != "key-hello.docx" {
		t.Errorf("storage key not propagated: %q", tpl.StorageKey)
	}
	if tpl.Checksum != "deadbeef" {
		t.Errorf("checksum not propagated: %q", tpl.Checksum)
	}
	if fs.clearDefaultCalls != 0 {
		t.Errorf("ClearDefault should not be called for non-default template, got %d", fs.clearDefaultCalls)
	}
}

func TestUpload_HappyPath_IsDefault_ClearsOtherDefaults(t *testing.T) {
	fs := &fakeStore{}
	fst := &fakeStorage{}
	svc := NewTemplateService(fs, fst)

	_, err := svc.Upload(ctxWithTenant(t), uuid.New(),
		&model.CreateRequest{Name: "X", Kind: "technical", IsDefault: true},
		bytes.NewReader([]byte("v")), "x.docx", 1)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if fs.clearDefaultCalls != 1 {
		t.Errorf("ClearDefault should be called once when IsDefault=true, got %d", fs.clearDefaultCalls)
	}
}

func TestUpload_ClearDefaultFails_RollsBackStorage(t *testing.T) {
	fs := &fakeStore{
		clearDefaultFn: func(ctx context.Context, kind string) error { return errors.New("boom") },
	}
	fst := &fakeStorage{}
	svc := NewTemplateService(fs, fst)

	_, err := svc.Upload(ctxWithTenant(t), uuid.New(),
		&model.CreateRequest{Name: "X", Kind: "technical", IsDefault: true},
		bytes.NewReader([]byte("v")), "x.docx", 1)
	if err == nil {
		t.Fatal("expected error from ClearDefault")
	}
	if fst.deleteCalls != 1 || fst.lastDeleted != "key-x.docx" {
		t.Errorf("storage.Delete should be called for rollback (compensation), got calls=%d key=%q", fst.deleteCalls, fst.lastDeleted)
	}
	if fs.createCalls != 0 {
		t.Errorf("store.Create should not be called when ClearDefault fails, got %d", fs.createCalls)
	}
}

func TestUpload_StoreCreateFails_RollsBackStorage(t *testing.T) {
	fs := &fakeStore{
		createFn: func(ctx context.Context, t *model.WordTemplate) error { return errors.New("dup") },
	}
	fst := &fakeStorage{}
	svc := NewTemplateService(fs, fst)

	_, err := svc.Upload(ctxWithTenant(t), uuid.New(),
		&model.CreateRequest{Name: "X", Kind: "standard"},
		bytes.NewReader([]byte("v")), "y.docx", 1)
	if err == nil {
		t.Fatal("expected error from store.Create")
	}
	if fst.deleteCalls != 1 || fst.lastDeleted != "key-y.docx" {
		t.Errorf("storage.Delete should compensate failed Create, got calls=%d key=%q", fst.deleteCalls, fst.lastDeleted)
	}
}

func TestUpload_StoragePutFails_PropagatesError(t *testing.T) {
	fs := &fakeStore{}
	fst := &fakeStorage{
		putFn: func(ctx context.Context, name string, r io.Reader) (string, string, int64, error) {
			return "", "", 0, errors.New("disk full")
		},
	}
	svc := NewTemplateService(fs, fst)

	_, err := svc.Upload(ctxWithTenant(t), uuid.New(),
		&model.CreateRequest{Name: "X", Kind: "standard"},
		bytes.NewReader([]byte("v")), "y.docx", 1)
	if err == nil {
		t.Fatal("expected error from storage.Put")
	}
	if fs.createCalls != 0 {
		t.Errorf("store.Create should not be called when storage.Put fails")
	}
}

func TestDownload_HappyPath(t *testing.T) {
	want := &model.WordTemplate{ID: uuid.New(), Name: "Z", StorageKey: "k1"}
	fs := &fakeStore{
		getFn: func(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error) { return want, nil },
	}
	fst := &fakeStorage{
		getFn: func(ctx context.Context, key string) (io.ReadCloser, error) {
			if key != "k1" {
				t.Errorf("storage.Get called with wrong key %q", key)
			}
			return io.NopCloser(strings.NewReader("body")), nil
		},
	}
	svc := NewTemplateService(fs, fst)

	rc, tpl, err := svc.Download(ctxWithTenant(t), want.ID)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer rc.Close()
	if tpl.ID != want.ID {
		t.Errorf("template id mismatch: got %v want %v", tpl.ID, want.ID)
	}
	body, _ := io.ReadAll(rc)
	if string(body) != "body" {
		t.Errorf("body mismatch: %q", body)
	}
}

func TestDownload_StoreError_ReturnsNilTriple(t *testing.T) {
	fs := &fakeStore{
		getFn: func(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error) {
			return nil, storeErrNotFound
		},
	}
	fst := &fakeStorage{}
	svc := NewTemplateService(fs, fst)

	rc, tpl, err := svc.Download(ctxWithTenant(t), uuid.New())
	if !errors.Is(err, storeErrNotFound) {
		t.Errorf("err mismatch: got %v", err)
	}
	if rc != nil || tpl != nil {
		t.Errorf("expected nil triple on error, got rc=%v tpl=%v", rc, tpl)
	}
}

func TestDelete_HappyPath_AlsoDeletesFromStorage(t *testing.T) {
	fs := &fakeStore{
		getFn: func(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error) {
			return &model.WordTemplate{ID: id, StorageKey: "key-abc"}, nil
		},
	}
	fst := &fakeStorage{}
	svc := NewTemplateService(fs, fst)

	if err := svc.Delete(ctxWithTenant(t), uuid.New()); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if fst.lastDeleted != "key-abc" {
		t.Errorf("storage should be cleaned up, got lastDeleted=%q", fst.lastDeleted)
	}
}

func TestDelete_StorageDeleteFailureIgnored(t *testing.T) {
	fs := &fakeStore{
		getFn: func(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error) {
			return &model.WordTemplate{ID: id, StorageKey: "k1"}, nil
		},
	}
	fst := &fakeStorage{
		deleteFn: func(ctx context.Context, key string) error { return errors.New("blob gone") },
	}
	svc := NewTemplateService(fs, fst)

	if err := svc.Delete(ctxWithTenant(t), uuid.New()); err != nil {
		t.Errorf("storage.Delete failure should be swallowed, got %v", err)
	}
}

func TestUpdate_IsDefaultSet_ClearsDefaultsFirst(t *testing.T) {
	got := &model.WordTemplate{ID: uuid.New(), Kind: "standard"}
	fs := &fakeStore{
		getFn: func(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error) { return got, nil },
		updateFn: func(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.WordTemplate, error) {
			return got, nil
		},
	}
	fst := &fakeStorage{}
	svc := NewTemplateService(fs, fst)

	isDef := true
	_, err := svc.Update(ctxWithTenant(t), got.ID, &model.UpdateRequest{IsDefault: &isDef})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if fs.clearDefaultCalls != 1 {
		t.Errorf("ClearDefault should run when promoting to default, got %d", fs.clearDefaultCalls)
	}
}

func TestUpdate_NonDefault_NoClearDefault(t *testing.T) {
	fs := &fakeStore{
		getFn: func(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error) {
			return &model.WordTemplate{ID: id, Kind: "standard"}, nil
		},
	}
	fst := &fakeStorage{}
	svc := NewTemplateService(fs, fst)

	name := "renamed"
	_, err := svc.Update(ctxWithTenant(t), uuid.New(), &model.UpdateRequest{Name: &name})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if fs.clearDefaultCalls != 0 {
		t.Errorf("ClearDefault should not run for non-default update, got %d", fs.clearDefaultCalls)
	}
}

// Compile-time assertion: *fakeStorage satisfies storage.Storage so the test
// compiles even if the interface changes.
var _ storage.Storage = (*fakeStorage)(nil)
