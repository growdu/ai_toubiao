package workers

// Task type constants for Asynq queues.
const (
	// TaskOutlineGenerate enqueues outline generation.
	TaskOutlineGenerate = "workflow:outline_generate"
	// TaskChapterGenerate enqueues chapter content generation.
	TaskChapterGenerate = "workflow:chapter_generate"
	// TaskAudit enqueues compliance audit.
	TaskAudit = "workflow:audit"
	// TaskExport enqueues document export.
	TaskExport = "workflow:export"
)

// Queue names for Asynq.
const (
	QueuePlanner  = "planner"
	QueueChapter  = "chapter"
	QueueAuditor  = "auditor"
	QueueExporter = "exporter"
)
