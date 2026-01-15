// Package scheduler implements the async job queue and worker system.
package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// JobStatus represents the state of a job.
type JobStatus string

const (
	JobStatusQueued    JobStatus = "QUEUED"
	JobStatusRunning   JobStatus = "RUNNING"
	JobStatusSucceeded JobStatus = "SUCCEEDED"
	JobStatusFailed    JobStatus = "FAILED"
	JobStatusCanceled  JobStatus = "CANCELED"
)

// Job represents a scheduled job.
type Job struct {
	ID          string                 `json:"id"`
	ToolID      string                 `json:"tool_id"`
	ToolVersion string                 `json:"tool_version"`
	Status      JobStatus              `json:"status"`
	Params      map[string]interface{} `json:"params"`
	InputArtifacts  map[string]string  `json:"input_artifacts"`
	OutputArtifacts map[string]string  `json:"output_artifacts,omitempty"`
	PipelineID  string                 `json:"pipeline_id,omitempty"`
	NodeID      string                 `json:"node_id,omitempty"`
	Priority    int                    `json:"priority"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	ExitCode    *int                   `json:"exit_code,omitempty"`
	Error       string                 `json:"error,omitempty"`
	RunID       string                 `json:"run_id,omitempty"`
}

// JobHandler is called when a job is ready to execute.
type JobHandler func(ctx context.Context, job *Job) (*JobResult, error)

// JobResult contains the outcome of job execution.
type JobResult struct {
	Success         bool
	ExitCode        int
	OutputArtifacts map[string]string
	Error           error
}

// Scheduler manages the job queue and workers.
type Scheduler struct {
	db           *sql.DB
	handler      JobHandler
	workers      int
	maxQueueSize int
	logger       *zap.Logger

	queue    chan *Job
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	mu       sync.RWMutex
	running  map[string]*Job
	
	// Per-tool concurrency limits
	toolLimits map[string]int
	toolSem    map[string]chan struct{}
	toolMu     sync.Mutex
}

// Config holds scheduler configuration.
type Config struct {
	DBPath       string
	Workers      int
	MaxQueueSize int
	Handler      JobHandler
}

// New creates a new scheduler.
func New(cfg Config, logger *zap.Logger) (*Scheduler, error) {
	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := initJobSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Scheduler{
		db:           db,
		handler:      cfg.Handler,
		workers:      cfg.Workers,
		maxQueueSize: cfg.MaxQueueSize,
		logger:       logger,
		queue:        make(chan *Job, cfg.MaxQueueSize),
		ctx:          ctx,
		cancel:       cancel,
		running:      make(map[string]*Job),
		toolLimits:   make(map[string]int),
		toolSem:      make(map[string]chan struct{}),
	}

	return s, nil
}

func initJobSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		tool_id TEXT NOT NULL,
		tool_version TEXT NOT NULL,
		status TEXT NOT NULL,
		params TEXT,
		input_artifacts TEXT,
		output_artifacts TEXT,
		pipeline_id TEXT,
		node_id TEXT,
		priority INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		started_at DATETIME,
		completed_at DATETIME,
		exit_code INTEGER,
		error TEXT,
		run_id TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_jobs_pipeline ON jobs(pipeline_id);
	CREATE INDEX IF NOT EXISTS idx_jobs_tool ON jobs(tool_id);
	`

	_, err := db.Exec(schema)
	return err
}

// Start begins processing jobs.
func (s *Scheduler) Start() error {
	// Recover queued jobs from database
	if err := s.recoverJobs(); err != nil {
		return fmt.Errorf("recovering jobs: %w", err)
	}

	// Start worker goroutines
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}

	s.logger.Info("scheduler started", zap.Int("workers", s.workers))
	return nil
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() error {
	s.logger.Info("stopping scheduler")
	s.cancel()
	
	// Close the queue to signal workers
	close(s.queue)
	
	// Wait for workers to finish
	s.wg.Wait()
	
	return s.db.Close()
}

// Submit queues a new job.
func (s *Scheduler) Submit(job *Job) error {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	if job.RunID == "" {
		job.RunID = uuid.New().String()
	}
	job.Status = JobStatusQueued
	job.CreatedAt = time.Now().UTC()

	// Persist to database
	if err := s.saveJob(job); err != nil {
		return fmt.Errorf("saving job: %w", err)
	}

	// Queue for execution
	select {
	case s.queue <- job:
		s.logger.Info("job queued",
			zap.String("job_id", job.ID),
			zap.String("tool_id", job.ToolID),
		)
		return nil
	default:
		return fmt.Errorf("job queue full")
	}
}

// GetJob retrieves a job by ID.
func (s *Scheduler) GetJob(id string) (*Job, error) {
	var job Job
	var paramsJSON, inputJSON, outputJSON sql.NullString
	var startedAt, completedAt sql.NullTime
	var exitCode sql.NullInt64

	err := s.db.QueryRow(`
		SELECT id, tool_id, tool_version, status, params, input_artifacts, output_artifacts,
		       pipeline_id, node_id, priority, created_at, started_at, completed_at, exit_code, error, run_id
		FROM jobs WHERE id = ?
	`, id).Scan(
		&job.ID, &job.ToolID, &job.ToolVersion, &job.Status,
		&paramsJSON, &inputJSON, &outputJSON,
		&job.PipelineID, &job.NodeID, &job.Priority,
		&job.CreatedAt, &startedAt, &completedAt, &exitCode, &job.Error, &job.RunID,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	if paramsJSON.Valid {
		json.Unmarshal([]byte(paramsJSON.String), &job.Params)
	}
	if inputJSON.Valid {
		json.Unmarshal([]byte(inputJSON.String), &job.InputArtifacts)
	}
	if outputJSON.Valid {
		json.Unmarshal([]byte(outputJSON.String), &job.OutputArtifacts)
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}
	if exitCode.Valid {
		code := int(exitCode.Int64)
		job.ExitCode = &code
	}

	return &job, nil
}

// ListJobs returns jobs matching the filter.
func (s *Scheduler) ListJobs(filter JobFilter) ([]*Job, error) {
	query := `SELECT id FROM jobs WHERE 1=1`
	args := []interface{}{}

	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.PipelineID != "" {
		query += ` AND pipeline_id = ?`
		args = append(args, filter.PipelineID)
	}
	if filter.ToolID != "" {
		query += ` AND tool_id = ?`
		args = append(args, filter.ToolID)
	}

	query += ` ORDER BY created_at DESC`
	if filter.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, filter.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		job, err := s.GetJob(id)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// JobFilter defines filtering options for job listing.
type JobFilter struct {
	Status     JobStatus
	PipelineID string
	ToolID     string
	Limit      int
}

// Cancel cancels a job.
func (s *Scheduler) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update status
	_, err := s.db.Exec(`
		UPDATE jobs SET status = ?, completed_at = ? WHERE id = ? AND status IN (?, ?)
	`, JobStatusCanceled, time.Now().UTC(), id, JobStatusQueued, JobStatusRunning)

	return err
}

// SetToolConcurrencyLimit sets the maximum concurrent runs for a tool.
func (s *Scheduler) SetToolConcurrencyLimit(toolID string, limit int) {
	s.toolMu.Lock()
	defer s.toolMu.Unlock()
	
	s.toolLimits[toolID] = limit
	s.toolSem[toolID] = make(chan struct{}, limit)
}

func (s *Scheduler) worker(id int) {
	defer s.wg.Done()

	s.logger.Debug("worker started", zap.Int("worker_id", id))

	for job := range s.queue {
		if s.ctx.Err() != nil {
			return
		}

		s.processJob(job)
	}

	s.logger.Debug("worker stopped", zap.Int("worker_id", id))
}

func (s *Scheduler) processJob(job *Job) {
	// Check for tool-specific concurrency limit
	s.toolMu.Lock()
	sem, hasSem := s.toolSem[job.ToolID]
	s.toolMu.Unlock()

	if hasSem {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		case <-s.ctx.Done():
			return
		}
	}

	// Mark as running
	s.mu.Lock()
	s.running[job.ID] = job
	s.mu.Unlock()

	now := time.Now().UTC()
	job.Status = JobStatusRunning
	job.StartedAt = &now

	s.updateJob(job)

	s.logger.Info("executing job",
		zap.String("job_id", job.ID),
		zap.String("tool_id", job.ToolID),
	)

	// Execute the job
	result, err := s.handler(s.ctx, job)

	// Update status based on result
	completedAt := time.Now().UTC()
	job.CompletedAt = &completedAt

	if err != nil {
		job.Status = JobStatusFailed
		job.Error = err.Error()
		s.logger.Error("job failed",
			zap.String("job_id", job.ID),
			zap.Error(err),
		)
	} else if result != nil {
		if result.Success {
			job.Status = JobStatusSucceeded
			job.ExitCode = &result.ExitCode
			job.OutputArtifacts = result.OutputArtifacts
		} else {
			job.Status = JobStatusFailed
			job.ExitCode = &result.ExitCode
			if result.Error != nil {
				job.Error = result.Error.Error()
			}
		}
		s.logger.Info("job completed",
			zap.String("job_id", job.ID),
			zap.String("status", string(job.Status)),
		)
	}

	s.updateJob(job)

	// Remove from running
	s.mu.Lock()
	delete(s.running, job.ID)
	s.mu.Unlock()
}

func (s *Scheduler) saveJob(job *Job) error {
	paramsJSON, _ := json.Marshal(job.Params)
	inputJSON, _ := json.Marshal(job.InputArtifacts)
	outputJSON, _ := json.Marshal(job.OutputArtifacts)

	_, err := s.db.Exec(`
		INSERT INTO jobs (id, tool_id, tool_version, status, params, input_artifacts, output_artifacts,
		                  pipeline_id, node_id, priority, created_at, run_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, job.ID, job.ToolID, job.ToolVersion, job.Status,
		string(paramsJSON), string(inputJSON), string(outputJSON),
		job.PipelineID, job.NodeID, job.Priority, job.CreatedAt, job.RunID)

	return err
}

func (s *Scheduler) updateJob(job *Job) error {
	outputJSON, _ := json.Marshal(job.OutputArtifacts)

	_, err := s.db.Exec(`
		UPDATE jobs SET status = ?, output_artifacts = ?, started_at = ?, completed_at = ?,
		                exit_code = ?, error = ? WHERE id = ?
	`, job.Status, string(outputJSON), job.StartedAt, job.CompletedAt,
		job.ExitCode, job.Error, job.ID)

	return err
}

func (s *Scheduler) recoverJobs() error {
	// Re-queue jobs that were running or queued when we stopped
	rows, err := s.db.Query(`
		SELECT id FROM jobs WHERE status IN (?, ?)
		ORDER BY priority DESC, created_at ASC
	`, JobStatusQueued, JobStatusRunning)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}

		job, err := s.GetJob(id)
		if err != nil {
			continue
		}

		// Reset to queued
		job.Status = JobStatusQueued
		job.StartedAt = nil
		s.updateJob(job)

		select {
		case s.queue <- job:
			s.logger.Info("recovered job", zap.String("job_id", id))
		default:
			s.logger.Warn("queue full, job not recovered", zap.String("job_id", id))
		}
	}

	return rows.Err()
}

// RunningJobs returns currently running jobs.
func (s *Scheduler) RunningJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0, len(s.running))
	for _, job := range s.running {
		jobs = append(jobs, job)
	}
	return jobs
}

// QueueLength returns the current queue length.
func (s *Scheduler) QueueLength() int {
	return len(s.queue)
}

// Stats returns scheduler statistics.
func (s *Scheduler) Stats() SchedulerStats {
	var stats SchedulerStats
	
	s.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE status = ?`, JobStatusQueued).Scan(&stats.Queued)
	s.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE status = ?`, JobStatusRunning).Scan(&stats.Running)
	s.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE status = ?`, JobStatusSucceeded).Scan(&stats.Succeeded)
	s.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE status = ?`, JobStatusFailed).Scan(&stats.Failed)
	s.db.QueryRow(`SELECT COUNT(*) FROM jobs`).Scan(&stats.Total)

	return stats
}

// SchedulerStats contains scheduler statistics.
type SchedulerStats struct {
	Total     int `json:"total"`
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}
