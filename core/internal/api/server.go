// Package api implements the REST API server.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ravenforge/ravenforge/core/internal/artifact"
	"github.com/ravenforge/ravenforge/core/internal/audit"
	"github.com/ravenforge/ravenforge/core/internal/manifest"
	"github.com/ravenforge/ravenforge/core/internal/pipeline"
	"github.com/ravenforge/ravenforge/core/internal/registry"
	"github.com/ravenforge/ravenforge/core/internal/scheduler"
	"go.uber.org/zap"
)

// Server implements the REST API.
type Server struct {
	router    *chi.Mux
	logger    *zap.Logger
	registry  *registry.Registry
	scheduler *scheduler.Scheduler
	artifacts *artifact.Store
	audit     *audit.Logger

	// Callbacks
	OnSubmitRun      func(ctx context.Context, req SubmitRunRequest) (*scheduler.Job, error)
	OnSubmitPipeline func(ctx context.Context, req SubmitPipelineRequest) (*pipeline.PipelineRun, error)
}

// Config holds API server configuration.
type Config struct {
	Registry  *registry.Registry
	Scheduler *scheduler.Scheduler
	Artifacts *artifact.Store
	Audit     *audit.Logger
}

// New creates a new API server.
func New(cfg Config, logger *zap.Logger) *Server {
	s := &Server{
		logger:    logger,
		registry:  cfg.Registry,
		scheduler: cfg.Scheduler,
		artifacts: cfg.Artifacts,
		audit:     cfg.Audit,
	}

	s.setupRouter()
	return s
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.loggingMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Health check
	r.Get("/health", s.handleHealth)

	// API v1 routes
	r.Route("/v1", func(r chi.Router) {
		// Tools
		r.Route("/tools", func(r chi.Router) {
			r.Get("/", s.handleListTools)
			r.Get("/{id}", s.handleGetTool)
		})

		// Runs
		r.Route("/runs", func(r chi.Router) {
			r.Post("/", s.handleSubmitRun)
			r.Get("/{runId}", s.handleGetRun)
		})

		// Pipelines
		r.Route("/pipelines", func(r chi.Router) {
			r.Post("/", s.handleSubmitPipeline)
		})

		// Jobs
		r.Route("/jobs", func(r chi.Router) {
			r.Get("/", s.handleListJobs)
			r.Get("/{jobId}", s.handleGetJob)
			r.Post("/{jobId}/cancel", s.handleCancelJob)
		})

		// Artifacts
		r.Route("/artifacts", func(r chi.Router) {
			r.Get("/{artifactId}", s.handleGetArtifact)
			r.Get("/{artifactId}/data", s.handleGetArtifactData)
		})

		// Audit
		r.Route("/audit", func(r chi.Router) {
			r.Get("/tail", s.handleAuditTail)
		})

		// Stats
		r.Get("/stats", s.handleStats)
	})

	s.router = r
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		
		defer func() {
			s.logger.Info("api request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", time.Since(start)),
				zap.String("remote_addr", r.RemoteAddr),
			)
		}()

		next.ServeHTTP(ww, r)
	})
}

// Response helpers

type errorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	s.writeJSON(w, status, errorResponse{
		Error: message,
		Code:  code,
	})
}

// Health check

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// Tools endpoints

type toolResponse struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Version      string                  `json:"version"`
	Description  string                  `json:"description"`
	Runtime      string                  `json:"runtime"`
	Inputs       []manifest.IOSpec       `json:"inputs"`
	Outputs      []manifest.IOSpec       `json:"outputs"`
	Capabilities manifest.Capabilities   `json:"capabilities"`
	Resources    manifest.Resources      `json:"resources"`
	Digest       string                  `json:"digest"`
}

func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	tools := s.registry.List()
	
	response := make([]toolResponse, 0, len(tools))
	for _, t := range tools {
		response = append(response, toolToResponse(t))
	}

	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleGetTool(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	version := r.URL.Query().Get("version")

	tool, err := s.registry.Get(id, version)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "TOOL_NOT_FOUND", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, toolToResponse(tool))
}

func toolToResponse(t *manifest.ToolManifest) toolResponse {
	return toolResponse{
		ID:           t.ID,
		Name:         t.Name,
		Version:      t.Version,
		Description:  t.Description,
		Runtime:      string(t.Runtime),
		Inputs:       t.Inputs,
		Outputs:      t.Outputs,
		Capabilities: t.Capabilities,
		Resources:    t.Resources,
		Digest:       t.Digest,
	}
}

// Runs endpoints

// SubmitRunRequest is the request body for submitting a run.
type SubmitRunRequest struct {
	ToolID      string                 `json:"tool_id"`
	ToolVersion string                 `json:"tool_version,omitempty"`
	Inputs      map[string]string      `json:"inputs"`       // Input name -> artifact ID
	Params      map[string]interface{} `json:"params,omitempty"`
}

type submitRunResponse struct {
	RunID string `json:"run_id"`
	JobID string `json:"job_id"`
}

func (s *Server) handleSubmitRun(w http.ResponseWriter, r *http.Request) {
	var req SubmitRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.ToolID == "" {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "tool_id is required")
		return
	}

	if s.OnSubmitRun == nil {
		s.writeError(w, http.StatusInternalServerError, "NOT_CONFIGURED", "run handler not configured")
		return
	}

	job, err := s.OnSubmitRun(r.Context(), req)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "SUBMIT_FAILED", err.Error())
		return
	}

	s.writeJSON(w, http.StatusAccepted, submitRunResponse{
		RunID: job.RunID,
		JobID: job.ID,
	})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")

	// Look up job by run ID
	jobs, err := s.scheduler.ListJobs(scheduler.JobFilter{})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	var job *scheduler.Job
	for _, j := range jobs {
		if j.RunID == runID {
			job = j
			break
		}
	}

	if job == nil {
		s.writeError(w, http.StatusNotFound, "RUN_NOT_FOUND", "run not found")
		return
	}

	s.writeJSON(w, http.StatusOK, job)
}

// Pipelines endpoints

// SubmitPipelineRequest is the request body for submitting a pipeline.
type SubmitPipelineRequest struct {
	Pipeline *pipeline.Pipeline `json:"pipeline"`
	Inputs   map[string]string  `json:"inputs"` // Input name -> artifact ID
}

func (s *Server) handleSubmitPipeline(w http.ResponseWriter, r *http.Request) {
	var req SubmitPipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Pipeline == nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "pipeline is required")
		return
	}

	if err := req.Pipeline.Validate(); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_PIPELINE", err.Error())
		return
	}

	if s.OnSubmitPipeline == nil {
		s.writeError(w, http.StatusInternalServerError, "NOT_CONFIGURED", "pipeline handler not configured")
		return
	}

	run, err := s.OnSubmitPipeline(r.Context(), req)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "SUBMIT_FAILED", err.Error())
		return
	}

	s.writeJSON(w, http.StatusAccepted, run)
}

// Jobs endpoints

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	filter := scheduler.JobFilter{}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = scheduler.JobStatus(status)
	}
	if toolID := r.URL.Query().Get("tool_id"); toolID != "" {
		filter.ToolID = toolID
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}

	jobs, err := s.scheduler.ListJobs(filter)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")

	job, err := s.scheduler.GetJob(jobID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "JOB_NOT_FOUND", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")

	if err := s.scheduler.Cancel(jobID); err != nil {
		s.writeError(w, http.StatusInternalServerError, "CANCEL_FAILED", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{
		"status": "canceled",
	})
}

// Artifacts endpoints

func (s *Server) handleGetArtifact(w http.ResponseWriter, r *http.Request) {
	artifactID := chi.URLParam(r, "artifactId")

	art, err := s.artifacts.Get(artifactID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "ARTIFACT_NOT_FOUND", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, art)
}

func (s *Server) handleGetArtifactData(w http.ResponseWriter, r *http.Request) {
	artifactID := chi.URLParam(r, "artifactId")

	art, err := s.artifacts.Get(artifactID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "ARTIFACT_NOT_FOUND", err.Error())
		return
	}

	reader, err := s.artifacts.Open(artifactID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "READ_FAILED", err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", art.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", art.Size))
	w.Header().Set("X-Artifact-Hash", art.Hash)
	w.WriteHeader(http.StatusOK)

	io.Copy(w, reader)
}

// Audit endpoints

func (s *Server) handleAuditTail(w http.ResponseWriter, r *http.Request) {
	n := 100 // Default
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			n = l
		}
	}

	sinceSeq := uint64(0)
	if since := r.URL.Query().Get("since"); since != "" {
		if s, err := strconv.ParseUint(since, 10, 64); err == nil {
			sinceSeq = s
		}
	}

	var entries []audit.Entry
	var err error

	if sinceSeq > 0 {
		entries, err = s.audit.TailSince(sinceSeq)
	} else {
		entries, err = s.audit.Tail(n)
	}

	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, entries)
}

// Stats endpoint

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	schedulerStats := s.scheduler.Stats()
	
	artifactCount, _ := s.artifacts.Count()
	artifactSize, _ := s.artifacts.TotalSize()

	stats := map[string]interface{}{
		"tools":     s.registry.Count(),
		"scheduler": schedulerStats,
		"artifacts": map[string]interface{}{
			"count": artifactCount,
			"size":  artifactSize,
		},
		"audit_seq": s.audit.SeqNum(),
	}

	s.writeJSON(w, http.StatusOK, stats)
}
