// Package audit provides append-only audit logging for SOC-grade traceability.
package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Logger provides append-only audit logging.
type Logger struct {
	path      string
	file      *os.File
	writer    *bufio.Writer
	mu        sync.Mutex
	logger    *zap.Logger
	seqNum    uint64
	lastHash  string
}

// Entry represents a single audit log entry.
type Entry struct {
	// Sequence number for ordering
	SeqNum uint64 `json:"seq_num"`
	// Timestamp in RFC3339Nano format
	Timestamp string `json:"timestamp"`
	// Event type
	EventType EventType `json:"event_type"`
	// Actor who initiated the action
	Actor Actor `json:"actor"`
	// Tool information (if applicable)
	Tool *ToolInfo `json:"tool,omitempty"`
	// Run information (if applicable)
	Run *RunInfo `json:"run,omitempty"`
	// Job information (if applicable)
	Job *JobInfo `json:"job,omitempty"`
	// Pipeline information (if applicable)
	Pipeline *PipelineInfo `json:"pipeline,omitempty"`
	// Artifact hashes involved
	Artifacts *ArtifactHashes `json:"artifacts,omitempty"`
	// Sandbox configuration used
	Sandbox *SandboxInfo `json:"sandbox,omitempty"`
	// Policy decision
	Policy *PolicyDecision `json:"policy,omitempty"`
	// Additional data
	Data map[string]interface{} `json:"data,omitempty"`
	// Hash of the previous entry (chain integrity)
	PrevHash string `json:"prev_hash"`
	// Hash of this entry
	Hash string `json:"hash,omitempty"`
}

// EventType defines the type of audit event.
type EventType string

const (
	EventToolRegistered   EventType = "tool.registered"
	EventToolUnregistered EventType = "tool.unregistered"
	EventRunSubmitted     EventType = "run.submitted"
	EventRunStarted       EventType = "run.started"
	EventRunCompleted     EventType = "run.completed"
	EventRunFailed        EventType = "run.failed"
	EventRunCanceled      EventType = "run.canceled"
	EventPipelineStarted  EventType = "pipeline.started"
	EventPipelineCompleted EventType = "pipeline.completed"
	EventPipelineFailed   EventType = "pipeline.failed"
	EventArtifactCreated  EventType = "artifact.created"
	EventPolicyDenied     EventType = "policy.denied"
	EventPolicyApproved   EventType = "policy.approved"
	EventAPIRequest       EventType = "api.request"
	EventSystemStartup    EventType = "system.startup"
	EventSystemShutdown   EventType = "system.shutdown"
)

// Actor represents who performed the action.
type Actor struct {
	Type     string `json:"type"` // "cli", "api", "system", "scheduler"
	Identity string `json:"identity,omitempty"`
	Token    string `json:"token,omitempty"` // Token ID, not the actual token
	IP       string `json:"ip,omitempty"`
}

// ToolInfo contains tool identification.
type ToolInfo struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

// RunInfo contains run details.
type RunInfo struct {
	ID          string        `json:"id"`
	Status      string        `json:"status"`
	Duration    time.Duration `json:"duration,omitempty"`
	ExitCode    *int          `json:"exit_code,omitempty"`
	ErrorMsg    string        `json:"error_msg,omitempty"`
	StdoutHash  string        `json:"stdout_hash,omitempty"`
	StderrHash  string        `json:"stderr_hash,omitempty"`
}

// JobInfo contains job details.
type JobInfo struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// PipelineInfo contains pipeline details.
type PipelineInfo struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Nodes   []string `json:"nodes,omitempty"`
	Status  string   `json:"status"`
}

// ArtifactHashes contains artifact hash references.
type ArtifactHashes struct {
	Inputs  map[string]string `json:"inputs,omitempty"`
	Outputs map[string]string `json:"outputs,omitempty"`
}

// SandboxInfo contains sandbox configuration.
type SandboxInfo struct {
	Runtime     string   `json:"runtime"`
	Image       string   `json:"image"`
	CPULimit    float64  `json:"cpu_limit"`
	MemoryLimit int64    `json:"memory_limit"`
	PidsLimit   int64    `json:"pids_limit"`
	Network     bool     `json:"network"`
	ReadOnly    bool     `json:"read_only"`
	Mounts      []string `json:"mounts,omitempty"`
	Timeout     string   `json:"timeout"`
}

// PolicyDecision contains policy evaluation results.
type PolicyDecision struct {
	Allowed       bool     `json:"allowed"`
	PolicyVersion string   `json:"policy_version"`
	MatchedRules  []string `json:"matched_rules,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	RequiredGates []string `json:"required_gates,omitempty"`
}

// New creates a new audit logger.
func New(path string, logger *zap.Logger) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, fmt.Errorf("creating audit log directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("opening audit log file: %w", err)
	}

	l := &Logger{
		path:   path,
		file:   file,
		writer: bufio.NewWriter(file),
		logger: logger,
	}

	// Initialize sequence number and last hash from existing log
	if err := l.initFromExisting(); err != nil {
		file.Close()
		return nil, fmt.Errorf("initializing from existing log: %w", err)
	}

	return l, nil
}

func (l *Logger) initFromExisting() error {
	file, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			l.seqNum = 0
			l.lastHash = ""
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lastLine string
	for scanner.Scan() {
		lastLine = scanner.Text()
	}

	if lastLine != "" {
		var entry Entry
		if err := json.Unmarshal([]byte(lastLine), &entry); err != nil {
			return fmt.Errorf("parsing last entry: %w", err)
		}
		l.seqNum = entry.SeqNum
		l.lastHash = entry.Hash
	}

	return scanner.Err()
}

// Log writes an audit entry.
func (l *Logger) Log(entry Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.seqNum++
	entry.SeqNum = l.seqNum
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	entry.PrevHash = l.lastHash

	// Compute hash of entry (without the hash field itself)
	entryForHash := entry
	entryForHash.Hash = ""
	hashData, _ := json.Marshal(entryForHash)
	hash := sha256.Sum256(hashData)
	entry.Hash = hex.EncodeToString(hash[:])

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling audit entry: %w", err)
	}

	if _, err := l.writer.Write(data); err != nil {
		return fmt.Errorf("writing audit entry: %w", err)
	}
	if _, err := l.writer.WriteString("\n"); err != nil {
		return fmt.Errorf("writing newline: %w", err)
	}
	if err := l.writer.Flush(); err != nil {
		return fmt.Errorf("flushing audit log: %w", err)
	}
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("syncing audit log: %w", err)
	}

	l.lastHash = entry.Hash

	return nil
}

// LogToolRegistered logs a tool registration event.
func (l *Logger) LogToolRegistered(actor Actor, tool ToolInfo) error {
	return l.Log(Entry{
		EventType: EventToolRegistered,
		Actor:     actor,
		Tool:      &tool,
	})
}

// LogRunSubmitted logs a run submission event.
func (l *Logger) LogRunSubmitted(actor Actor, tool ToolInfo, run RunInfo, artifacts ArtifactHashes, sandbox SandboxInfo, policy PolicyDecision) error {
	return l.Log(Entry{
		EventType: EventRunSubmitted,
		Actor:     actor,
		Tool:      &tool,
		Run:       &run,
		Artifacts: &artifacts,
		Sandbox:   &sandbox,
		Policy:    &policy,
	})
}

// LogRunStarted logs a run start event.
func (l *Logger) LogRunStarted(run RunInfo, sandbox SandboxInfo) error {
	return l.Log(Entry{
		EventType: EventRunStarted,
		Actor:     Actor{Type: "scheduler"},
		Run:       &run,
		Sandbox:   &sandbox,
	})
}

// LogRunCompleted logs a run completion event.
func (l *Logger) LogRunCompleted(run RunInfo, artifacts ArtifactHashes) error {
	return l.Log(Entry{
		EventType: EventRunCompleted,
		Actor:     Actor{Type: "scheduler"},
		Run:       &run,
		Artifacts: &artifacts,
	})
}

// LogRunFailed logs a run failure event.
func (l *Logger) LogRunFailed(run RunInfo, errMsg string) error {
	run.ErrorMsg = errMsg
	return l.Log(Entry{
		EventType: EventRunFailed,
		Actor:     Actor{Type: "scheduler"},
		Run:       &run,
	})
}

// LogPipelineStarted logs a pipeline start event.
func (l *Logger) LogPipelineStarted(actor Actor, pipeline PipelineInfo) error {
	return l.Log(Entry{
		EventType: EventPipelineStarted,
		Actor:     actor,
		Pipeline:  &pipeline,
	})
}

// LogPipelineCompleted logs a pipeline completion event.
func (l *Logger) LogPipelineCompleted(pipeline PipelineInfo, artifacts ArtifactHashes) error {
	return l.Log(Entry{
		EventType: EventPipelineCompleted,
		Actor:     Actor{Type: "scheduler"},
		Pipeline:  &pipeline,
		Artifacts: &artifacts,
	})
}

// LogPolicyDenied logs a policy denial event.
func (l *Logger) LogPolicyDenied(actor Actor, tool ToolInfo, policy PolicyDecision) error {
	return l.Log(Entry{
		EventType: EventPolicyDenied,
		Actor:     actor,
		Tool:      &tool,
		Policy:    &policy,
	})
}

// LogSystemStartup logs a system startup event.
func (l *Logger) LogSystemStartup(data map[string]interface{}) error {
	return l.Log(Entry{
		EventType: EventSystemStartup,
		Actor:     Actor{Type: "system"},
		Data:      data,
	})
}

// LogSystemShutdown logs a system shutdown event.
func (l *Logger) LogSystemShutdown() error {
	return l.Log(Entry{
		EventType: EventSystemShutdown,
		Actor:     Actor{Type: "system"},
	})
}

// Tail returns the last n entries from the audit log.
func (l *Logger) Tail(n int) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Flush any pending writes
	l.writer.Flush()

	file, err := os.Open(l.path)
	if err != nil {
		return nil, fmt.Errorf("opening audit log: %w", err)
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // Skip malformed entries
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning audit log: %w", err)
	}

	if len(entries) > n {
		entries = entries[len(entries)-n:]
	}

	return entries, nil
}

// TailSince returns entries since a given sequence number.
func (l *Logger) TailSince(sinceSeq uint64) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.writer.Flush()

	file, err := os.Open(l.path)
	if err != nil {
		return nil, fmt.Errorf("opening audit log: %w", err)
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.SeqNum > sinceSeq {
			entries = append(entries, entry)
		}
	}

	return entries, scanner.Err()
}

// Verify verifies the integrity of the audit log chain.
func (l *Logger) Verify() (bool, uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.writer.Flush()

	file, err := os.Open(l.path)
	if err != nil {
		return false, 0, fmt.Errorf("opening audit log: %w", err)
	}
	defer file.Close()

	var prevHash string
	var verified uint64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return false, verified, fmt.Errorf("parsing entry %d: %w", verified+1, err)
		}

		// Check chain
		if entry.PrevHash != prevHash {
			return false, verified, fmt.Errorf("chain broken at entry %d", entry.SeqNum)
		}

		// Verify hash
		entryForHash := entry
		expectedHash := entry.Hash
		entryForHash.Hash = ""
		hashData, _ := json.Marshal(entryForHash)
		hash := sha256.Sum256(hashData)
		actualHash := hex.EncodeToString(hash[:])

		if actualHash != expectedHash {
			return false, verified, fmt.Errorf("hash mismatch at entry %d", entry.SeqNum)
		}

		prevHash = entry.Hash
		verified++
	}

	return true, verified, scanner.Err()
}

// Export exports the audit log to a writer.
func (l *Logger) Export(w io.Writer) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.writer.Flush()

	file, err := os.Open(l.path)
	if err != nil {
		return fmt.Errorf("opening audit log: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	return err
}

// Close closes the audit logger.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.writer.Flush(); err != nil {
		return err
	}
	return l.file.Close()
}

// Path returns the audit log file path.
func (l *Logger) Path() string {
	return l.path
}

// SeqNum returns the current sequence number.
func (l *Logger) SeqNum() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.seqNum
}
