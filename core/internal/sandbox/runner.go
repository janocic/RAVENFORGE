// Package sandbox provides secure container execution for tools.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/ravenforge/ravenforge/core/internal/manifest"
	"go.uber.org/zap"
)

// Runner executes tools in a sandboxed environment.
type Runner interface {
	Run(ctx context.Context, config *RunConfig) (*RunResult, error)
	Pull(ctx context.Context, image string) error
	Close() error
}

// RunConfig defines the configuration for a tool run.
type RunConfig struct {
	// Tool manifest
	Manifest *manifest.ToolManifest
	// Input directory to mount
	InputDir string
	// Output directory for results
	OutputDir string
	// Environment variables
	Env map[string]string
	// Resource limits
	Limits ResourceLimits
	// Allow network access
	Network bool
	// Execution timeout
	Timeout time.Duration
	// Run ID for tracking
	RunID string
}

// ResourceLimits defines container resource constraints.
type ResourceLimits struct {
	CPULimit    float64
	MemoryLimit int64
	PidsLimit   int64
}

// RunResult contains the results of a tool execution.
type RunResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Duration time.Duration
	Error    error
}

// DockerRunner implements Runner using Docker.
type DockerRunner struct {
	client *client.Client
	logger *zap.Logger
}

// NewDockerRunner creates a new Docker-based sandbox runner.
func NewDockerRunner(socketPath string, logger *zap.Logger) (*DockerRunner, error) {
	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
	}
	if socketPath != "" {
		opts = append(opts, client.WithHost(socketPath))
	}
	// Otherwise use default (works with Docker Desktop on Windows)

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Docker client: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		cli.Close()
		return nil, fmt.Errorf("connecting to Docker: %w", err)
	}

	return &DockerRunner{
		client: cli,
		logger: logger,
	}, nil
}

// Run executes a tool in a sandboxed container.
func (r *DockerRunner) Run(ctx context.Context, config *RunConfig) (*RunResult, error) {
	start := time.Now()
	result := &RunResult{}

	// Apply timeout
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	// Prepare container configuration
	containerConfig, hostConfig, err := r.prepareConfig(config)
	if err != nil {
		result.Error = err
		return result, err
	}

	r.logger.Info("creating container",
		zap.String("run_id", config.RunID),
		zap.String("image", config.Manifest.Entrypoint.Image),
	)

	// Create container
	resp, err := r.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		result.Error = fmt.Errorf("creating container: %w", err)
		return result, result.Error
	}
	containerID := resp.ID

	// Ensure cleanup
	defer func() {
		removeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		r.client.ContainerRemove(removeCtx, containerID, container.RemoveOptions{Force: true})
	}()

	// Start container
	if err := r.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		result.Error = fmt.Errorf("starting container: %w", err)
		return result, result.Error
	}

	r.logger.Info("container started",
		zap.String("run_id", config.RunID),
		zap.String("container_id", containerID[:12]),
	)

	// Wait for completion
	statusCh, errCh := r.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			result.Error = fmt.Errorf("waiting for container: %w", err)
			return result, result.Error
		}
	case status := <-statusCh:
		result.ExitCode = int(status.StatusCode)
	case <-ctx.Done():
		result.Error = ctx.Err()
		// Kill the container on timeout
		r.client.ContainerKill(context.Background(), containerID, "KILL")
		return result, result.Error
	}

	// Capture logs
	stdout, stderr, err := r.captureLogs(ctx, containerID)
	if err != nil {
		r.logger.Warn("failed to capture logs", zap.Error(err))
	}
	result.Stdout = stdout
	result.Stderr = stderr
	result.Duration = time.Since(start)

	r.logger.Info("container completed",
		zap.String("run_id", config.RunID),
		zap.Int("exit_code", result.ExitCode),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

func (r *DockerRunner) prepareConfig(config *RunConfig) (*container.Config, *container.HostConfig, error) {
	m := config.Manifest

	// Build command
	cmd := m.Entrypoint.Command
	if len(m.Entrypoint.Args) > 0 {
		cmd = append(cmd, m.Entrypoint.Args...)
	}

	// Build environment
	env := []string{}
	for k, v := range m.Entrypoint.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	// Add run ID
	env = append(env, fmt.Sprintf("RF_RUN_ID=%s", config.RunID))

	containerConfig := &container.Config{
		Image:      m.Entrypoint.Image,
		Cmd:        cmd,
		Env:        env,
		WorkingDir: m.Entrypoint.WorkDir,
		// Security: disable stdin
		AttachStdin: false,
		OpenStdin:   false,
	}

	// Prepare mounts
	mounts := []mount.Mount{}

	// Input directory (read-only)
	if config.InputDir != "" {
		absInput, err := filepath.Abs(config.InputDir)
		if err != nil {
			return nil, nil, fmt.Errorf("resolving input path: %w", err)
		}
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   absInput,
			Target:   "/rf/in",
			ReadOnly: true,
		})
	}

	// Output directory (writable)
	if config.OutputDir != "" {
		absOutput, err := filepath.Abs(config.OutputDir)
		if err != nil {
			return nil, nil, fmt.Errorf("resolving output path: %w", err)
		}
		// Ensure output directory exists
		if err := os.MkdirAll(absOutput, 0750); err != nil {
			return nil, nil, fmt.Errorf("creating output directory: %w", err)
		}
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   absOutput,
			Target:   "/rf/out",
			ReadOnly: false,
		})
	}

	// Calculate memory in bytes
	memoryLimit := config.Limits.MemoryLimit
	if memoryLimit == 0 {
		memoryLimit = 512 * 1024 * 1024 // 512MB default
	}

	// Calculate CPU period and quota
	cpuLimit := config.Limits.CPULimit
	if cpuLimit == 0 {
		cpuLimit = 1.0
	}
	cpuPeriod := int64(100000) // 100ms
	cpuQuota := int64(cpuLimit * float64(cpuPeriod))

	pidsLimit := config.Limits.PidsLimit
	if pidsLimit == 0 {
		pidsLimit = 100
	}

	// Network mode
	networkMode := "none"
	if config.Network {
		networkMode = "bridge"
	}

	hostConfig := &container.HostConfig{
		Mounts:      mounts,
		NetworkMode: container.NetworkMode(networkMode),
		// Security hardening
		ReadonlyRootfs: true,
		SecurityOpt:    []string{"no-new-privileges:true"},
		CapDrop:        []string{"ALL"},
		// Resource limits
		Resources: container.Resources{
			Memory:     memoryLimit,
			CPUPeriod:  cpuPeriod,
			CPUQuota:   cpuQuota,
			PidsLimit:  &pidsLimit,
			MemorySwap: memoryLimit, // Disable swap
		},
		// Disable privileged mode
		Privileged: false,
		// Disable user namespace remapping override
		UsernsMode: "",
		// Tmpfs for /tmp
		Tmpfs: map[string]string{
			"/tmp": "rw,noexec,nosuid,size=64m",
		},
	}

	return containerConfig, hostConfig, nil
}

func (r *DockerRunner) captureLogs(ctx context.Context, containerID string) ([]byte, []byte, error) {
	logs, err := r.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return nil, nil, err
	}
	defer logs.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, logs); err != nil {
		return nil, nil, err
	}

	return stdout.Bytes(), stderr.Bytes(), nil
}

// Pull pulls a container image.
func (r *DockerRunner) Pull(ctx context.Context, imageName string) error {
	r.logger.Info("pulling image", zap.String("image", imageName))

	reader, err := r.client.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}
	defer reader.Close()

	// Drain the reader to complete the pull
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("reading pull response: %w", err)
	}

	return nil
}

// Close closes the Docker client.
func (r *DockerRunner) Close() error {
	return r.client.Close()
}

// ImageExists checks if an image exists locally.
func (r *DockerRunner) ImageExists(ctx context.Context, imageName string) (bool, error) {
	_, _, err := r.client.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetSandboxInfo returns sandbox configuration for audit logging.
func (r *DockerRunner) GetSandboxInfo(config *RunConfig) map[string]interface{} {
	return map[string]interface{}{
		"runtime":      "docker",
		"image":        config.Manifest.Entrypoint.Image,
		"cpu_limit":    config.Limits.CPULimit,
		"memory_limit": config.Limits.MemoryLimit,
		"pids_limit":   config.Limits.PidsLimit,
		"network":      config.Network,
		"read_only":    true,
		"timeout":      config.Timeout.String(),
	}
}
