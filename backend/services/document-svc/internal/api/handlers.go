// Package api implements the HTTP layer for document-svc.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bidwriter/services/document-svc/internal/model"
	"github.com/bidwriter/services/document-svc/internal/service"
	"github.com/bidwriter/services/document-svc/internal/storage"
	"github.com/bidwriter/services/document-svc/internal/store"
	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/bidwriter/shared/pkg/validator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxUploadBytes = 100 << 20 // 100 MB

type Handlers struct {
	Store   *store.Store
	Storage storage.Storage
	Parser  *service.ParserService
	Log     *slog.Logger
}

func (h *Handlers) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ready"}`))
	})

	r.Route("/api/v1/documents", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.upload)        // multipart upload
		r.Post("/json", h.createJSON) // metadata-only registration
		r.Get("/{id}", h.get)
		r.Patch("/{id}", h.update)
		r.Delete("/{id}", h.delete)
		r.Get("/{id}/content", h.download)
		// Parse endpoints
		r.Post("/{id}/parse", h.parse)
		r.Get("/{id}/parse-result", h.getParseResult)
	})

	return r
}

// GET /api/v1/documents?project_id=&limit=&cursor=
func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	var projectID *uuid.UUID
	if p := q.Get("project_id"); p != "" {
		u, err := uuid.Parse(p)
		if err != nil {
			httperr.InvalidInput(w, rid, "invalid project_id", nil)
			return
		}
		projectID = &u
	}
	var cursor *uuid.UUID
	if c := q.Get("cursor"); c != "" {
		u, err := uuid.Parse(c)
		if err != nil {
			httperr.InvalidInput(w, rid, "invalid cursor", nil)
			return
		}
		cursor = &u
	}

	docs, err := h.Store.List(r.Context(), projectID, limit, cursor)
	if err != nil {
		h.Log.Error("list documents", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": docs,
		"meta": map[string]any{"count": len(docs)},
	})
}

// POST /api/v1/documents/json — metadata-only registration (file already uploaded elsewhere)
func (h *Handlers) createJSON(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	var req model.CreateRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}
	if err := validator.Validate(&req); err != nil {
		httperr.InvalidInput(w, rid, err.Error(), nil)
		return
	}

	d := &model.Document{
		ProjectID:      req.ProjectID,
		Name:           req.Name,
		Kind:           req.Kind,
		MimeType:       req.MimeType,
		SizeBytes:      req.SizeBytes,
		ChecksumSHA256: req.ChecksumSHA256,
		StorageKey:     "external://" + req.ChecksumSHA256, // placeholder
		Metadata:       req.Metadata,
	}

	// uploaded_by comes from upstream header (api-gateway)
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		d.UploadedBy = uuid.MustParse(uid)
	}

	if err := h.Store.Create(r.Context(), d); err != nil {
		h.Log.Error("create document", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": d})
}

// POST /api/v1/documents — multipart upload
func (h *Handlers) upload(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		httperr.InvalidInput(w, rid, "invalid multipart form: "+err.Error(), nil)
		return
	}

	projectIDStr := r.FormValue("project_id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		httperr.InvalidInput(w, rid, "project_id is required", nil)
		return
	}

	fh, header, err := r.FormFile("file")
	if err != nil {
		httperr.InvalidInput(w, rid, "file is required", nil)
		return
	}
	defer fh.Close() // ensure underlying file is closed

	// Stream to storage, computing SHA256
	key, checksum, size, err := h.Storage.Put(r.Context(), header.Filename, fh)
	if err != nil {
		h.Log.Error("storage put", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}

	d := &model.Document{
		ProjectID:      projectID,
		Name:           header.Filename,
		Kind:           model.DocumentKind(r.FormValue("kind")),
		MimeType:       header.Header.Get("Content-Type"),
		SizeBytes:      size,
		StorageKey:     key,
		ChecksumSHA256: checksum,
	}
	if d.Kind == "" {
		d.Kind = model.KindAttachment
	}
	if d.MimeType == "" {
		d.MimeType = "application/octet-stream"
	}
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		d.UploadedBy = uuid.MustParse(uid)
	}

	if err := h.Store.Create(r.Context(), d); err != nil {
		// Best-effort cleanup of orphaned storage object
		_ = h.Storage.Delete(r.Context(), key)
		h.Log.Error("create document", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": d})
}

// GET /api/v1/documents/{id}
func (h *Handlers) get(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	d, err := h.Store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "文档")
		return
	}
	if err != nil {
		h.Log.Error("get document", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": d})
}

// PATCH /api/v1/documents/{id}
func (h *Handlers) update(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	var req model.UpdateRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}
	if err := validator.Validate(&req); err != nil {
		httperr.InvalidInput(w, rid, err.Error(), nil)
		return
	}
	d, err := h.Store.Update(r.Context(), id, &req)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "文档")
		return
	}
	if errors.Is(err, store.ErrVersionConflict) {
		httperr.Write(w, http.StatusConflict, httperr.CodeVersionConflict,
			"版本冲突", rid, nil)
		return
	}
	if err != nil {
		h.Log.Error("update document", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": d})
}

// DELETE /api/v1/documents/{id}
func (h *Handlers) delete(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	d, err := h.Store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "文档")
		return
	}
	if err != nil {
		h.Log.Error("get for delete", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	if err := h.Store.Delete(r.Context(), id); err != nil {
		h.Log.Error("delete document", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	// Best-effort storage cleanup (do not fail the request)
	_ = h.Storage.Delete(r.Context(), d.StorageKey)
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/documents/{id}/content — download
func (h *Handlers) download(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	d, err := h.Store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "文档")
		return
	}
	if err != nil {
		h.Log.Error("get for download", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	rc, err := h.Storage.Get(r.Context(), d.StorageKey)
	if err != nil {
		h.Log.Error("storage get", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", d.MimeType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+d.Name+`"`)
	_, _ = io.Copy(w, rc)
}

// POST /api/v1/documents/{id}/parse — trigger RFP parsing
func (h *Handlers) parse(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}

	var req model.ParseRequest
	if err := readJSON(r.Body, &req); err != nil {
		// If no body, default to async=true
		req.Async = true
	}

	result, err := h.Parser.Parse(r.Context(), id, req.Async)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "文档")
		return
	}
	if err != nil {
		h.Log.Error("parse document", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}

	if req.Async {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"message": "parse started in background",
			"doc_id":  id,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

// GET /api/v1/documents/{id}/parse-result — get parsing result
func (h *Handlers) getParseResult(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}

	result, err := h.Parser.GetParseResult(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "文档")
		return
	}
	if err != nil {
		h.Log.Error("get parse result", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(body io.ReadCloser, v any) error {
	if body == nil {
		return errors.New("empty body")
	}
	defer body.Close()
	dec := json.NewDecoder(io.LimitReader(body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// avoid unused import
var _ = bytes.NewReader