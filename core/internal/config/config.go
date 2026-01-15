// Package config handles configuration loading and validation for the Ravenforge daemon.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete daemon configuration.
type Config struct {
	// Server configuration
	Server ServerConfig `yaml:"server"`

	// Tool directories to scan for manifests
	ToolDirs []string `yaml:"tool_dirs"`

	// Artifact storage configuration
	Artifacts ArtifactConfig `yaml:"artifacts"`

	// Audit logging configuration
	Audit AuditConfig `yaml:"audit"`

	// Policy configuration
	Policy PolicyConfig `yaml:"policy"`

	// Scheduler configuration
	Scheduler SchedulerConfig `yaml:"scheduler"`

	// Sandbox configuration
	Sandbox SandboxConfig `yaml:"sandbox"`

	// Database configuration
	Database DatabaseConfig `yaml:"database"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging"`
}

// ServerConfig holds API server settings.
type ServerConfig struct {
	// Host to bind to
	Host string `yaml:"host"`
	// Port to listen on
	Port int `yaml:"port"`
	// Enable TLS
	TLS bool `yaml:"tls"`
	// TLS certificate file
	CertFile string `yaml:"cert_file"`
	// TLS key file
	KeyFile string `yaml:"key_file"`
	// Request timeout
	Timeout time.Duration `yaml:"timeout"`
}

// ArtifactConfig holds artifact store settings.
type ArtifactConfig struct {
	// Base directory for artifact storage
	BaseDir string `yaml:"base_dir"`
	// Maximum artifact size in bytes
	MaxSize int64 `yaml:"max_size"`
}

// AuditConfig holds audit logging settings.
type AuditConfig struct {
	// Path to the audit log file
	LogPath string `yaml:"log_path"`
	// Enable audit logging
	Enabled bool `yaml:"enabled"`
	// Rotation settings
	MaxSizeMB   int `yaml:"max_size_mb"`
	MaxBackups  int `yaml:"max_backups"`
	MaxAgeDays  int `yaml:"max_age_days"`
}

// PolicyConfig holds policy engine settings.
type PolicyConfig struct {
	// Path to the policy file
	PolicyFile string `yaml:"policy_file"`
	// Default policy mode (enforce, audit, disabled)
	DefaultMode string `yaml:"default_mode"`
}

// SchedulerConfig holds job scheduler settings.
type SchedulerConfig struct {
	// Number of concurrent workers
	Workers int `yaml:"workers"`
	// Maximum queue size
	MaxQueueSize int `yaml:"max_queue_size"`
	// Job timeout default
	DefaultTimeout time.Duration `yaml:"default_timeout"`
}

// SandboxConfig holds sandbox runner settings.
type SandboxConfig struct {
	// Container runtime (docker, containerd)
	Runtime string `yaml:"runtime"`
	// Docker socket path
	DockerSocket string `yaml:"docker_socket"`
	// Default resource limits
	DefaultLimits ResourceLimits `yaml:"default_limits"`
	// Network mode (none, host, bridge)
	DefaultNetwork string `yaml:"default_network"`
}

// ResourceLimits defines container resource constraints.
type ResourceLimits struct {
	// CPU limit (e.g., "1.0" for one core)
	CPULimit float64 `yaml:"cpu_limit"`
	// Memory limit in bytes
	MemoryLimit int64 `yaml:"memory_limit"`
	// Maximum number of processes
	PidsLimit int64 `yaml:"pids_limit"`
	// Execution timeout
	Timeout time.Duration `yaml:"timeout"`
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	// Database driver (sqlite3)
	Driver string `yaml:"driver"`
	// Database path
	Path string `yaml:"path"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	// Log level (debug, info, warn, error)
	Level string `yaml:"level"`
	// Log format (json, console)
	Format string `yaml:"format"`
	// Log file path (empty for stdout)
	File string `yaml:"file"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:    "127.0.0.1",
			Port:    7433,
			TLS:     false,
			Timeout: 30 * time.Second,
		},
		ToolDirs: []string{
			"/etc/ravenforge/tools",
			"/var/lib/ravenforge/tools",
		},
		Artifacts: ArtifactConfig{
			BaseDir: "/var/lib/ravenforge/artifacts",
			MaxSize: 1 << 30, // 1GB
		},
		Audit: AuditConfig{
			LogPath:    "/var/log/ravenforge/audit.jsonl",
			Enabled:    true,
			MaxSizeMB:  100,
			MaxBackups: 10,
			MaxAgeDays: 90,
		},
		Policy: PolicyConfig{
			PolicyFile:  "/etc/ravenforge/policy.yaml",
			DefaultMode: "enforce",
		},
		Scheduler: SchedulerConfig{
			Workers:        4,
			MaxQueueSize:   1000,
			DefaultTimeout: 5 * time.Minute,
		},
		Sandbox: SandboxConfig{
			Runtime:        "docker",
			DockerSocket:   "/var/run/docker.sock",
			DefaultNetwork: "none",
			DefaultLimits: ResourceLimits{
				CPULimit:    1.0,
				MemoryLimit: 512 * 1024 * 1024, // 512MB
				PidsLimit:   100,
				Timeout:     5 * time.Minute,
			},
		},
		Database: DatabaseConfig{
			Driver: "sqlite3",
			Path:   "/var/lib/ravenforge/ravenforge.db",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// Load reads configuration from a YAML file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Use defaults if no config file
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Scheduler.Workers < 1 {
		return fmt.Errorf("scheduler workers must be at least 1")
	}

	if c.Sandbox.Runtime != "docker" && c.Sandbox.Runtime != "containerd" {
		return fmt.Errorf("unsupported sandbox runtime: %s", c.Sandbox.Runtime)
	}

	validModes := map[string]bool{"enforce": true, "audit": true, "disabled": true}
	if !validModes[c.Policy.DefaultMode] {
		return fmt.Errorf("invalid policy mode: %s", c.Policy.DefaultMode)
	}

	return nil
}

// EnsureDirectories creates required directories.
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.Artifacts.BaseDir,
		filepath.Dir(c.Audit.LogPath),
		filepath.Dir(c.Database.Path),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	return nil
}

// Save writes the configuration to a YAML file.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0640); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}
