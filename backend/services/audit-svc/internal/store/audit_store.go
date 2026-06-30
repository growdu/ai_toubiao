package store

import (
	"context"
	"errors"
	"time"

	"github.com/bidwriter/services/audit-svc/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateReport creates a new audit report.
func (s *Store) CreateReport(ctx context.Context, r *model.AuditReport) error {
	r.ID = uuid.New()
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_reports (id, bid_job_id, tenant_id, status, critical, major, minor, total_issues, passed, started_at, finished_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, r.ID, r.BidJobID, r.TenantID, r.Status, r.Critical, r.Major, r.Minor, r.TotalIssues, r.Passed, r.StartedAt, r.FinishedAt, r.CreatedAt, r.UpdatedAt)
	return err
}

// GetReport retrieves an audit report by bid job ID.
func (s *Store) GetReport(ctx context.Context, bidJobID uuid.UUID) (*model.AuditReport, error) {
	var r model.AuditReport
	err := s.pool.QueryRow(ctx, `
		SELECT id, bid_job_id, tenant_id, status, critical, major, minor, total_issues, passed, started_at, finished_at, created_at, updated_at
		FROM audit_reports WHERE bid_job_id = $1 ORDER BY created_at DESC LIMIT 1
	`, bidJobID).Scan(&r.ID, &r.BidJobID, &r.TenantID, &r.Status, &r.Critical, &r.Major, &r.Minor, &r.TotalIssues, &r.Passed, &r.StartedAt, &r.FinishedAt, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &r, err
}

// UpdateReport updates an existing audit report.
func (s *Store) UpdateReport(ctx context.Context, r *model.AuditReport) error {
	r.UpdatedAt = time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE audit_reports SET status=$1, critical=$2, major=$3, minor=$4, total_issues=$5, passed=$6, started_at=$7, finished_at=$8, updated_at=$9
		WHERE id=$10
	`, r.Status, r.Critical, r.Major, r.Minor, r.TotalIssues, r.Passed, r.StartedAt, r.FinishedAt, r.UpdatedAt, r.ID)
	return err
}

// BatchInsertIssues inserts multiple audit issues.
func (s *Store) BatchInsertIssues(ctx context.Context, issues []*model.AuditIssue) error {
	if len(issues) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	for _, iss := range issues {
		iss.ID = uuid.New()
		iss.CreatedAt = time.Now()
		batch.Queue(`
			INSERT INTO audit_issues (id, report_id, bid_job_id, tenant_id, chapter_id, chapter_title, severity, dimension, issue, suggestion, evidence, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, iss.ID, iss.ReportID, iss.BidJobID, iss.TenantID, iss.ChapterID, iss.ChapterTitle, iss.Severity, iss.Dimension, iss.Issue, iss.Suggestion, iss.Evidence, iss.Status, iss.CreatedAt)
	}
	br := tx.SendBatch(ctx, batch)
	defer br.Close()
	for range issues {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return br.Close()
}

// GetIssuesByReportID retrieves all issues for a report.
func (s *Store) GetIssuesByReportID(ctx context.Context, reportID uuid.UUID) ([]*model.AuditIssue, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, report_id, bid_job_id, tenant_id, chapter_id, chapter_title, severity, dimension, issue, suggestion, evidence, status, resolved_by, resolved_at, created_at
		FROM audit_issues WHERE report_id = $1 ORDER BY
			CASE severity WHEN 'critical' THEN 1 WHEN 'major' THEN 2 ELSE 3 END,
			created_at ASC
	`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []*model.AuditIssue
	for rows.Next() {
		var iss model.AuditIssue
		if err := rows.Scan(&iss.ID, &iss.ReportID, &iss.BidJobID, &iss.TenantID, &iss.ChapterID, &iss.ChapterTitle, &iss.Severity, &iss.Dimension, &iss.Issue, &iss.Suggestion, &iss.Evidence, &iss.Status, &iss.ResolvedBy, &iss.ResolvedAt, &iss.CreatedAt); err != nil {
			return nil, err
		}
		issues = append(issues, &iss)
	}
	return issues, rows.Err()
}

// ResolveIssue marks an issue as resolved.
func (s *Store) ResolveIssue(ctx context.Context, issueID, userID uuid.UUID, decision string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE audit_issues SET status=$1, resolved_by=$2, resolved_at=$3 WHERE id=$4
	`, decision, userID, time.Now(), issueID)
	return err
}

// GetBidJobWithChapters retrieves bid job and its chapters for audit.
func (s *Store) GetBidJobWithChapters(ctx context.Context, bidJobID uuid.UUID) (*BidJobWithChapters, error) {
	var result BidJobWithChapters

	// Get bid job
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, project_id, status, created_at FROM bid_jobs WHERE id = $1
	`, bidJobID).Scan(&result.BidJob.ID, &result.BidJob.TenantID, &result.BidJob.ProjectID, &result.BidJob.Status, &result.BidJob.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	// Get chapters
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, chapter_type, status, content, target_word_count, min_word_count
		FROM chapter_specs WHERE bid_job_id = $1 ORDER BY created_at ASC
	`, bidJobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ch ChapterInfo
		if err := rows.Scan(&ch.ID, &ch.Title, &ch.ChapterType, &ch.Status, &ch.Content, &ch.TargetWordCount, &ch.MinWordCount); err != nil {
			return nil, err
		}
		result.Chapters = append(result.Chapters, ch)
	}
	return &result, rows.Err()
}

type BidJobWithChapters struct {
	BidJob   BidJobInfo    `db:"bid_job"`
	Chapters []ChapterInfo `db:"chapters"`
}

type BidJobInfo struct {
	ID        uuid.UUID `db:"id"`
	TenantID  uuid.UUID `db:"tenant_id"`
	ProjectID uuid.UUID `db:"project_id"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
}

type ChapterInfo struct {
	ID              uuid.UUID  `db:"id"`
	Title           string     `db:"title"`
	ChapterType     string     `db:"chapter_type"`
	Status          string     `db:"status"`
	Content         *string    `db:"content"`
	TargetWordCount int        `db:"target_word_count"`
	MinWordCount    int        `db:"min_word_count"`
}
