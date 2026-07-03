package workers

// Config holds all external dependencies needed by the Asynq worker pool.
// It is built from the service-level config and injected into every worker
// so that no service URL is ever hardcoded.
type Config struct {
	RedisAddr    string // e.g. "redis:6379" or "localhost:6379"
	RouterURL    string // router-svc base URL
	KnowledgeURL string // knowledge-svc base URL
	DocumentURL  string // document-svc base URL
	AuditURL     string // audit-svc base URL
}
