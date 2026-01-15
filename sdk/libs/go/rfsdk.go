// Package rfsdk provides SDK utilities for Ravenforge tool development.
package rfsdk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultInputDir is the default input directory inside containers.
	DefaultInputDir = "/rf/in"
	// DefaultOutputDir is the default output directory inside containers.
	DefaultOutputDir = "/rf/out"
)

// Config holds SDK configuration.
type Config struct {
	InputDir  string
	OutputDir string
	RunID     string
}

// DefaultConfig returns the default SDK configuration from environment.
func DefaultConfig() *Config {
	inputDir := os.Getenv("RF_INPUT_DIR")
	if inputDir == "" {
		inputDir = DefaultInputDir
	}

	outputDir := os.Getenv("RF_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = DefaultOutputDir
	}

	return &Config{
		InputDir:  inputDir,
		OutputDir: outputDir,
		RunID:     os.Getenv("RF_RUN_ID"),
	}
}

// Tool provides utilities for tool implementations.
type Tool struct {
	config *Config
	logger *Logger
}

// NewTool creates a new tool instance.
func NewTool() *Tool {
	return &Tool{
		config: DefaultConfig(),
		logger: NewLogger(),
	}
}

// NewToolWithConfig creates a new tool with custom configuration.
func NewToolWithConfig(config *Config) *Tool {
	return &Tool{
		config: config,
		logger: NewLogger(),
	}
}

// Logger returns the tool's logger.
func (t *Tool) Logger() *Logger {
	return t.logger
}

// InputDir returns the input directory path.
func (t *Tool) InputDir() string {
	return t.config.InputDir
}

// OutputDir returns the output directory path.
func (t *Tool) OutputDir() string {
	return t.config.OutputDir
}

// RunID returns the current run ID.
func (t *Tool) RunID() string {
	return t.config.RunID
}

// ReadInput reads an input file by name.
func (t *Tool) ReadInput(name string) ([]byte, error) {
	path := filepath.Join(t.config.InputDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading input %s: %w", name, err)
	}
	return data, nil
}

// ReadInputJSON reads and parses a JSON input file.
func (t *Tool) ReadInputJSON(name string, v interface{}) error {
	data, err := t.ReadInput(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// ListInputs returns all input file names.
func (t *Tool) ListInputs() ([]string, error) {
	entries, err := os.ReadDir(t.config.InputDir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// WriteOutput writes an output file.
func (t *Tool) WriteOutput(name string, data []byte) error {
	path := filepath.Join(t.config.OutputDir, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing output %s: %w", name, err)
	}
	return nil
}

// WriteOutputJSON writes a JSON output file.
func (t *Tool) WriteOutputJSON(name string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	return t.WriteOutput(name, data)
}

// Result represents the tool execution result.
type Result struct {
	Status  string            `json:"status"`
	Outputs []OutputMeta      `json:"outputs"`
	Error   string            `json:"error,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// OutputMeta describes an output artifact.
type OutputMeta struct {
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Hash        string `json:"hash"`
	Size        int64  `json:"size"`
}

// WriteResult writes the result.json file.
func (t *Tool) WriteResult(result *Result) error {
	return t.WriteOutputJSON("result.json", result)
}

// Success creates a success result and writes it.
func (t *Tool) Success() error {
	outputs, err := t.collectOutputMeta()
	if err != nil {
		return err
	}

	return t.WriteResult(&Result{
		Status:  "success",
		Outputs: outputs,
	})
}

// Fail creates a failure result and writes it.
func (t *Tool) Fail(err error) error {
	return t.WriteResult(&Result{
		Status: "failed",
		Error:  err.Error(),
	})
}

func (t *Tool) collectOutputMeta() ([]OutputMeta, error) {
	entries, err := os.ReadDir(t.config.OutputDir)
	if err != nil {
		return nil, err
	}

	var outputs []OutputMeta
	for _, e := range entries {
		if e.IsDir() || e.Name() == "result.json" {
			continue
		}

		path := filepath.Join(t.config.OutputDir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}

		hash, err := hashFile(path)
		if err != nil {
			continue
		}

		outputs = append(outputs, OutputMeta{
			Name:        e.Name(),
			ContentType: "application/octet-stream", // Could be improved
			Hash:        hash,
			Size:        info.Size(),
		})
	}

	return outputs, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// Logger provides structured logging for tools.
type Logger struct {
	encoder *json.Encoder
}

// LogEntry represents a log entry.
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"msg"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// NewLogger creates a new logger.
func NewLogger() *Logger {
	return &Logger{
		encoder: json.NewEncoder(os.Stdout),
	}
}

func (l *Logger) log(level, msg string, fields map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		Message:   msg,
		Fields:    fields,
	}
	l.encoder.Encode(entry)
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log("debug", msg, f)
}

// Info logs an info message.
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log("info", msg, f)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log("warn", msg, f)
}

// Error logs an error message.
func (l *Logger) Error(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log("error", msg, f)
}

// GetParam returns a parameter value from environment.
func GetParam(name string) string {
	return os.Getenv("RF_PARAM_" + name)
}

// GetParamWithDefault returns a parameter value or default.
func GetParamWithDefault(name, defaultValue string) string {
	v := GetParam(name)
	if v == "" {
		return defaultValue
	}
	return v
}
