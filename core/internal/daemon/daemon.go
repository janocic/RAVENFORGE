// Package daemon implements the core Ravenforge daemon.
package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/ravenforge/ravenforge/core/internal/api"
	"github.com/ravenforge/ravenforge/core/internal/artifact"
	"github.com/ravenforge/ravenforge/core/internal/audit"
	"github.com/ravenforge/ravenforge/core/internal/config"
	"github.com/ravenforge/ravenforge/core/internal/pipeline"
	"github.com/ravenforge/ravenforge/core/internal/policy"
	"github.com/ravenforge/ravenforge/core/internal/registry"
	"github.com/ravenforge/ravenforge/core/internal/sandbox"
	"github.com/ravenforge/ravenforge/core/internal/scheduler"
	"go.uber.org/zap"
)

// Daemon is the main Ravenforge daemon.
type Daemon struct {
	config     *config.Config
	logger     *zap.Logger
	registry   *registry.Registry
	scheduler  *scheduler.Scheduler
	artifacts  *artifact.Store
	audit      *audit.Logger
	policy     *policy.Engine
	sandbox    sandbox.Runner
	apiServer  *api.Server
	httpServer *http.Server
	executor   *pipeline.Executor
}

// New creates a new daemon.
func New(cfg *config.Config, logger *zap.Logger) (*Daemon, error) {
	d := &Daemon{
		config: cfg,
		logger: logger,
	}

	if err := d.initialize(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Daemon) initialize() error {
	var err error

	// Ensure directories exist
	if err := d.config.EnsureDirectories(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Initialize audit logger first (for compliance)
	d.audit, err = audit.New(d.config.Audit.LogPath, d.logger)
	if err != nil {
		return fmt.Errorf("initializing audit logger: %w", err)
	}

	// Initialize tool registry
	d.registry, err = registry.New(d.config.Database.Path, d.logger)
	if err != nil {
		return fmt.Errorf("initializing registry: %w", err)
	}

	// Initialize artifact store
	d.artifacts, err = artifact.NewStore(
		d.config.Artifacts.BaseDir,
		d.config.Database.Path,
		d.logger,
	)
	if err != nil {
		return fmt.Errorf("initializing artifact store: %w", err)
	}

	// Initialize policy engine
	d.policy = policy.New()
	if err := d.policy.LoadFromFile(d.config.Policy.PolicyFile); err != nil {
		d.logger.Warn("failed to load policy file, using defaults", zap.Error(err))
	}

	// Initialize sandbox runner
	d.sandbox, err = sandbox.NewDockerRunner(d.config.Sandbox.DockerSocket, d.logger)
	if err != nil {
		return fmt.Errorf("initializing sandbox runner: %w", err)
	}

	// Initialize scheduler
	d.scheduler, err = scheduler.New(scheduler.Config{
		DBPath:       d.config.Database.Path,
		Workers:      d.config.Scheduler.Workers,
		MaxQueueSize: d.config.Scheduler.MaxQueueSize,
		Handler:      d.executeJob,
	}, d.logger)
	if err != nil {
		return fmt.Errorf("initializing scheduler: %w", err)
	}

	// Initialize pipeline executor
	d.executor = pipeline.NewExecutor(d.logger)
	d.executor.ExecuteTool = d.executeTool

	// Initialize API server
	d.apiServer = api.New(api.Config{
		Registry:  d.registry,
		Scheduler: d.scheduler,
		Artifacts: d.artifacts,
		Audit:     d.audit,
	}, d.logger)

	d.apiServer.OnSubmitRun = d.handleSubmitRun
	d.apiServer.OnSubmitPipeline = d.handleSubmitPipeline

	return nil
}

// Run starts the daemon and blocks until shutdown.
func (d *Daemon) Run() error {
	// Log startup
	d.audit.LogSystemStartup(map[string]interface{}{
		"version":   "1.0.0",
		"config":    d.config.Database.Path,
		"tool_dirs": d.config.ToolDirs,
	})

	// Discover tools
	count, err := d.registry.Discover(d.config.ToolDirs)
	if err != nil {
		d.logger.Warn("error during tool discovery", zap.Error(err))
	}
	d.logger.Info("discovered tools", zap.Int("count", count))

	// Start scheduler
	if err := d.scheduler.Start(); err != nil {
		return fmt.Errorf("starting scheduler: %w", err)
	}

	// Start HTTP server
	addr := fmt.Sprintf("%s:%d", d.config.Server.Host, d.config.Server.Port)
	d.httpServer = &http.Server{
		Addr:         addr,
		Handler:      d.apiServer.Handler(),
		ReadTimeout:  d.config.Server.Timeout,
		WriteTimeout: d.config.Server.Timeout,
	}

	errCh := make(chan error, 1)
	go func() {
		d.logger.Info("starting API server", zap.String("addr", addr))
		if err := d.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		d.logger.Info("received shutdown signal", zap.String("signal", sig.String()))
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	return d.Shutdown()
}

// Shutdown gracefully shuts down the daemon.
func (d *Daemon) Shutdown() error {
	d.logger.Info("shutting down daemon")

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := d.httpServer.Shutdown(ctx); err != nil {
		d.logger.Error("error shutting down HTTP server", zap.Error(err))
	}

	// Stop scheduler
	if err := d.scheduler.Stop(); err != nil {
		d.logger.Error("error stopping scheduler", zap.Error(err))
	}

	// Close sandbox runner
	if err := d.sandbox.Close(); err != nil {
		d.logger.Error("error closing sandbox runner", zap.Error(err))
	}

	// Close artifact store
	if err := d.artifacts.Close(); err != nil {
		d.logger.Error("error closing artifact store", zap.Error(err))
	}

	// Close registry
	if err := d.registry.Close(); err != nil {
		d.logger.Error("error closing registry", zap.Error(err))
	}

	// Log shutdown
	d.audit.LogSystemShutdown()

	// Close audit logger last
	if err := d.audit.Close(); err != nil {
		d.logger.Error("error closing audit logger", zap.Error(err))
	}

	d.logger.Info("daemon shutdown complete")
	return nil
}

// handleSubmitRun handles a run submission from the API.
func (d *Daemon) handleSubmitRun(ctx context.Context, req api.SubmitRunRequest) (*scheduler.Job, error) {
	// Get tool manifest
	tool, err := d.registry.Get(req.ToolID, req.ToolVersion)
	if err != nil {
		return nil, fmt.Errorf("tool not found: %w", err)
	}

	// Evaluate policy
	decision := d.policy.Evaluate(tool)

	// Create actor for audit
	actor := audit.Actor{
		Type: "api",
	}

	if !decision.Allowed {
		d.audit.LogPolicyDenied(actor, audit.ToolInfo{
			ID:      tool.ID,
			Version: tool.Version,
			Digest:  tool.Digest,
		}, audit.PolicyDecision{
			Allowed:       false,
			PolicyVersion: decision.PolicyVersion,
			MatchedRules:  decision.MatchedRules,
			Reason:        decision.Reason,
		})
		return nil, fmt.Errorf("policy denied: %s", decision.Reason)
	}

	// Create job
	job := &scheduler.Job{
		ToolID:         tool.ID,
		ToolVersion:    tool.Version,
		InputArtifacts: req.Inputs,
		Params:         req.Params,
	}

	// Submit to scheduler
	if err := d.scheduler.Submit(job); err != nil {
		return nil, fmt.Errorf("submitting job: %w", err)
	}

	return job, nil
}

// handleSubmitPipeline handles a pipeline submission from the API.
func (d *Daemon) handleSubmitPipeline(ctx context.Context, req api.SubmitPipelineRequest) (*pipeline.PipelineRun, error) {
	actor := audit.Actor{
		Type: "api",
	}

	// Log pipeline start
	d.audit.LogPipelineStarted(actor, audit.PipelineInfo{
		ID:     req.Pipeline.ID,
		Name:   req.Pipeline.Name,
		Status: "STARTED",
	})

	// Execute pipeline
	run, err := d.executor.Execute(ctx, req.Pipeline, req.Inputs)
	if err != nil {
		d.audit.Log(audit.Entry{
			EventType: audit.EventPipelineFailed,
			Actor:     audit.Actor{Type: "system"},
			Pipeline: &audit.PipelineInfo{
				ID:     req.Pipeline.ID,
				Name:   req.Pipeline.Name,
				Status: "FAILED",
			},
			Data: map[string]interface{}{
				"error": err.Error(),
			},
		})
		return nil, err
	}

	// Log completion
	d.audit.LogPipelineCompleted(audit.PipelineInfo{
		ID:     run.PipelineID,
		Name:   run.PipelineName,
		Status: run.Status,
	}, audit.ArtifactHashes{
		Outputs: run.Outputs,
	})

	return run, nil
}

// executeJob is the scheduler's job handler.
func (d *Daemon) executeJob(ctx context.Context, job *scheduler.Job) (*scheduler.JobResult, error) {
	// Get tool manifest
	tool, err := d.registry.Get(job.ToolID, job.ToolVersion)
	if err != nil {
		return nil, fmt.Errorf("tool not found: %w", err)
	}

	// Prepare input directory
	inputDir, err := d.prepareInputDir(job.InputArtifacts)
	if err != nil {
		return nil, fmt.Errorf("preparing inputs: %w", err)
	}
	defer os.RemoveAll(inputDir)

	// Prepare output directory
	outputDir, err := os.MkdirTemp("", "rf-output-*")
	if err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}
	defer os.RemoveAll(outputDir)

	// Build run config
	runConfig := &sandbox.RunConfig{
		Manifest:  tool,
		InputDir:  inputDir,
		OutputDir: outputDir,
		Env:       make(map[string]string),
		Limits: sandbox.ResourceLimits{
			CPULimit:    tool.Resources.CPU,
			MemoryLimit: parseMemory(tool.Resources.Memory),
			PidsLimit:   tool.Resources.MaxPids,
		},
		Network: tool.Capabilities.Network,
		Timeout: d.config.Scheduler.DefaultTimeout,
		RunID:   job.RunID,
	}

	// Add params as environment variables
	for k, v := range job.Params {
		runConfig.Env[fmt.Sprintf("RF_PARAM_%s", k)] = fmt.Sprintf("%v", v)
	}

	// Log run start
	sandboxInfo := audit.SandboxInfo{
		Runtime:     "docker",
		Image:       tool.Entrypoint.Image,
		CPULimit:    runConfig.Limits.CPULimit,
		MemoryLimit: runConfig.Limits.MemoryLimit,
		PidsLimit:   runConfig.Limits.PidsLimit,
		Network:     runConfig.Network,
		ReadOnly:    true,
		Timeout:     runConfig.Timeout.String(),
	}

	d.audit.LogRunStarted(audit.RunInfo{
		ID:     job.RunID,
		Status: "RUNNING",
	}, sandboxInfo)

	// Execute in sandbox
	result, err := d.sandbox.Run(ctx, runConfig)
	if err != nil {
		d.audit.LogRunFailed(audit.RunInfo{
			ID:     job.RunID,
			Status: "FAILED",
		}, err.Error())
		return nil, err
	}

	// Process outputs
	outputArtifacts, err := d.processOutputs(job.RunID, tool.ID, outputDir)
	if err != nil {
		d.logger.Warn("error processing outputs", zap.Error(err))
	}

	// Store stdout/stderr as artifacts if non-empty
	if len(result.Stdout) > 0 {
		d.artifacts.CreateFromBytes(result.Stdout, artifact.CreateInput{
			Name:          "stdout",
			ContentType:   "text/plain",
			ProducerRunID: job.RunID,
			ProducerTool:  tool.ID,
		})
	}
	if len(result.Stderr) > 0 {
		d.artifacts.CreateFromBytes(result.Stderr, artifact.CreateInput{
			Name:          "stderr",
			ContentType:   "text/plain",
			ProducerRunID: job.RunID,
			ProducerTool:  tool.ID,
		})
	}

	// Log completion
	runInfo := audit.RunInfo{
		ID:       job.RunID,
		Status:   "SUCCEEDED",
		Duration: result.Duration,
		ExitCode: &result.ExitCode,
	}
	if result.ExitCode != 0 {
		runInfo.Status = "FAILED"
	}

	d.audit.LogRunCompleted(runInfo, audit.ArtifactHashes{
		Outputs: outputArtifacts,
	})

	return &scheduler.JobResult{
		Success:         result.ExitCode == 0,
		ExitCode:        result.ExitCode,
		OutputArtifacts: outputArtifacts,
		Error:           result.Error,
	}, nil
}

// executeTool executes a tool for the pipeline executor.
func (d *Daemon) executeTool(ctx context.Context, toolID, toolVersion string, inputs map[string]string, params map[string]interface{}) (map[string]string, error) {
	job := &scheduler.Job{
		ID:             uuid.New().String(),
		RunID:          uuid.New().String(),
		ToolID:         toolID,
		ToolVersion:    toolVersion,
		InputArtifacts: inputs,
		Params:         params,
	}

	result, err := d.executeJob(ctx, job)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("tool execution failed with exit code %d", result.ExitCode)
	}

	return result.OutputArtifacts, nil
}

func (d *Daemon) prepareInputDir(inputs map[string]string) (string, error) {
	dir, err := os.MkdirTemp("", "rf-input-*")
	if err != nil {
		return "", err
	}

	for name, artifactID := range inputs {
		path, err := d.artifacts.GetPath(artifactID)
		if err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("getting artifact %s: %w", artifactID, err)
		}

		// Symlink the artifact into the input directory
		linkPath := filepath.Join(dir, name)
		if err := os.Symlink(path, linkPath); err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("creating symlink: %w", err)
		}
	}

	return dir, nil
}

func (d *Daemon) processOutputs(runID, toolID, outputDir string) (map[string]string, error) {
	outputs := make(map[string]string)

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return outputs, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(outputDir, entry.Name())
		art, err := d.artifacts.CreateFromFile(path, artifact.CreateInput{
			Name:          entry.Name(),
			ContentType:   "application/octet-stream", // Could be improved with content detection
			ProducerRunID: runID,
			ProducerTool:  toolID,
		})
		if err != nil {
			d.logger.Warn("failed to store output artifact",
				zap.String("file", entry.Name()),
				zap.Error(err),
			)
			continue
		}

		outputs[entry.Name()] = art.ID
	}

	return outputs, nil
}

func parseMemory(s string) int64 {
	// Simple parser for memory strings like "512Mi", "1Gi"
	var value int64
	var unit string
	fmt.Sscanf(s, "%d%s", &value, &unit)

	switch unit {
	case "Ki":
		return value * 1024
	case "Mi":
		return value * 1024 * 1024
	case "Gi":
		return value * 1024 * 1024 * 1024
	default:
		return value
	}
}

func hashBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
