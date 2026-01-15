// Package artifact manages the artifact store for tool inputs and outputs.
package artifact

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// Store manages artifact storage and retrieval.
type Store struct {
	baseDir string
	db      *sql.DB
	logger  *zap.Logger
	mu      sync.RWMutex
}

// Artifact represents stored artifact metadata.
type Artifact struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	ContentType   string            `json:"content_type"`
	SchemaVersion string            `json:"schema_version,omitempty"`
	Hash          string            `json:"hash"` // SHA256
	Size          int64             `json:"size"`
	ProducerRunID string            `json:"producer_run_id,omitempty"`
	ProducerTool  string            `json:"producer_tool,omitempty"`
	PipelineID    string            `json:"pipeline_id,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Path          string            `json:"-"` // Internal path
}

// NewStore creates a new artifact store.
func NewStore(baseDir string, dbPath string, logger *zap.Logger) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return nil, fmt.Errorf("creating artifact directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := initArtifactSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return &Store{
		baseDir: baseDir,
		db:      db,
		logger:  logger,
	}, nil
}

func initArtifactSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS artifacts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		content_type TEXT NOT NULL,
		schema_version TEXT,
		hash TEXT NOT NULL,
		size INTEGER NOT NULL,
		producer_run_id TEXT,
		producer_tool TEXT,
		pipeline_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata TEXT
	);
	
	CREATE INDEX IF NOT EXISTS idx_artifacts_hash ON artifacts(hash);
	CREATE INDEX IF NOT EXISTS idx_artifacts_producer ON artifacts(producer_run_id);
	CREATE INDEX IF NOT EXISTS idx_artifacts_pipeline ON artifacts(pipeline_id);
	`

	_, err := db.Exec(schema)
	return err
}

// CreateInput creates an artifact from a file path for use as tool input.
type CreateInput struct {
	Name          string
	ContentType   string
	SchemaVersion string
	ProducerRunID string
	ProducerTool  string
	PipelineID    string
	Metadata      map[string]string
}

// Create stores a new artifact from a reader.
func (s *Store) Create(r io.Reader, input CreateInput) (*Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	artifactDir := filepath.Join(s.baseDir, id)
	if err := os.MkdirAll(artifactDir, 0750); err != nil {
		return nil, fmt.Errorf("creating artifact directory: %w", err)
	}

	dataPath := filepath.Join(artifactDir, "data")
	f, err := os.Create(dataPath)
	if err != nil {
		return nil, fmt.Errorf("creating artifact file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(f, hasher)

	size, err := io.Copy(writer, r)
	if err != nil {
		os.RemoveAll(artifactDir)
		return nil, fmt.Errorf("writing artifact data: %w", err)
	}

	hash := hex.EncodeToString(hasher.Sum(nil))

	artifact := &Artifact{
		ID:            id,
		Name:          input.Name,
		ContentType:   input.ContentType,
		SchemaVersion: input.SchemaVersion,
		Hash:          hash,
		Size:          size,
		ProducerRunID: input.ProducerRunID,
		ProducerTool:  input.ProducerTool,
		PipelineID:    input.PipelineID,
		CreatedAt:     time.Now().UTC(),
		Metadata:      input.Metadata,
		Path:          dataPath,
	}

	metadataJSON, _ := json.Marshal(artifact.Metadata)

	_, err = s.db.Exec(`
		INSERT INTO artifacts (id, name, content_type, schema_version, hash, size, producer_run_id, producer_tool, pipeline_id, created_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, artifact.ID, artifact.Name, artifact.ContentType, artifact.SchemaVersion,
		artifact.Hash, artifact.Size, artifact.ProducerRunID, artifact.ProducerTool,
		artifact.PipelineID, artifact.CreatedAt, string(metadataJSON))

	if err != nil {
		os.RemoveAll(artifactDir)
		return nil, fmt.Errorf("storing artifact metadata: %w", err)
	}

	// Write metadata file
	metaPath := filepath.Join(artifactDir, "metadata.json")
	metaData, _ := json.MarshalIndent(artifact, "", "  ")
	if err := os.WriteFile(metaPath, metaData, 0640); err != nil {
		s.logger.Warn("failed to write metadata file", zap.Error(err))
	}

	s.logger.Info("created artifact",
		zap.String("id", id),
		zap.String("name", input.Name),
		zap.String("hash", hash),
		zap.Int64("size", size),
	)

	return artifact, nil
}

// CreateFromFile creates an artifact from a file path.
func (s *Store) CreateFromFile(path string, input CreateInput) (*Artifact, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	return s.Create(f, input)
}

// CreateFromBytes creates an artifact from byte data.
func (s *Store) CreateFromBytes(data []byte, input CreateInput) (*Artifact, error) {
	r := &byteReader{data: data}
	return s.Create(r, input)
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// Get retrieves an artifact by ID.
func (s *Store) Get(id string) (*Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var artifact Artifact
	var metadataJSON string

	err := s.db.QueryRow(`
		SELECT id, name, content_type, schema_version, hash, size, producer_run_id, producer_tool, pipeline_id, created_at, metadata
		FROM artifacts WHERE id = ?
	`, id).Scan(
		&artifact.ID, &artifact.Name, &artifact.ContentType, &artifact.SchemaVersion,
		&artifact.Hash, &artifact.Size, &artifact.ProducerRunID, &artifact.ProducerTool,
		&artifact.PipelineID, &artifact.CreatedAt, &metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("artifact not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying artifact: %w", err)
	}

	if metadataJSON != "" {
		json.Unmarshal([]byte(metadataJSON), &artifact.Metadata)
	}

	artifact.Path = filepath.Join(s.baseDir, id, "data")

	return &artifact, nil
}

// GetByHash retrieves an artifact by its content hash.
func (s *Store) GetByHash(hash string) (*Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var id string
	err := s.db.QueryRow(`SELECT id FROM artifacts WHERE hash = ? LIMIT 1`, hash).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("artifact not found with hash: %s", hash)
	}
	if err != nil {
		return nil, fmt.Errorf("querying artifact: %w", err)
	}

	s.mu.RUnlock()
	defer s.mu.RLock()
	return s.Get(id)
}

// Open returns a reader for the artifact data.
func (s *Store) Open(id string) (io.ReadCloser, error) {
	artifact, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	return os.Open(artifact.Path)
}

// GetPath returns the filesystem path to the artifact data.
func (s *Store) GetPath(id string) (string, error) {
	artifact, err := s.Get(id)
	if err != nil {
		return "", err
	}
	return artifact.Path, nil
}

// List returns all artifacts, optionally filtered.
func (s *Store) List(filter ListFilter) ([]*Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, name, content_type, schema_version, hash, size, producer_run_id, producer_tool, pipeline_id, created_at, metadata FROM artifacts WHERE 1=1`
	args := []interface{}{}

	if filter.PipelineID != "" {
		query += ` AND pipeline_id = ?`
		args = append(args, filter.PipelineID)
	}
	if filter.ProducerRunID != "" {
		query += ` AND producer_run_id = ?`
		args = append(args, filter.ProducerRunID)
	}
	if filter.ProducerTool != "" {
		query += ` AND producer_tool = ?`
		args = append(args, filter.ProducerTool)
	}

	query += ` ORDER BY created_at DESC`

	if filter.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, filter.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []*Artifact
	for rows.Next() {
		var a Artifact
		var metadataJSON string
		if err := rows.Scan(
			&a.ID, &a.Name, &a.ContentType, &a.SchemaVersion,
			&a.Hash, &a.Size, &a.ProducerRunID, &a.ProducerTool,
			&a.PipelineID, &a.CreatedAt, &metadataJSON,
		); err != nil {
			return nil, err
		}
		if metadataJSON != "" {
			json.Unmarshal([]byte(metadataJSON), &a.Metadata)
		}
		a.Path = filepath.Join(s.baseDir, a.ID, "data")
		artifacts = append(artifacts, &a)
	}

	return artifacts, rows.Err()
}

// ListFilter defines artifact list filtering options.
type ListFilter struct {
	PipelineID    string
	ProducerRunID string
	ProducerTool  string
	Limit         int
}

// Verify checks the integrity of an artifact.
func (s *Store) Verify(id string) (bool, error) {
	artifact, err := s.Get(id)
	if err != nil {
		return false, err
	}

	f, err := os.Open(artifact.Path)
	if err != nil {
		return false, fmt.Errorf("opening artifact file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return false, fmt.Errorf("hashing artifact: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	return actualHash == artifact.Hash, nil
}

// Close closes the artifact store.
func (s *Store) Close() error {
	return s.db.Close()
}

// Count returns the number of stored artifacts.
func (s *Store) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM artifacts`).Scan(&count)
	return count, err
}

// TotalSize returns the total size of all artifacts.
func (s *Store) TotalSize() (int64, error) {
	var total int64
	err := s.db.QueryRow(`SELECT COALESCE(SUM(size), 0) FROM artifacts`).Scan(&total)
	return total, err
}
