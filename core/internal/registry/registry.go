// Package registry manages the tool registry and discovery.
package registry

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ravenforge/ravenforge/core/internal/manifest"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// Registry manages tool registration and lookup.
type Registry struct {
	db     *sql.DB
	tools  map[string]*manifest.ToolManifest
	mu     sync.RWMutex
	logger *zap.Logger
}

// ToolRecord represents a tool entry in the database.
type ToolRecord struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Description  string    `json:"description"`
	Runtime      string    `json:"runtime"`
	Digest       string    `json:"digest"`
	ManifestPath string    `json:"manifest_path"`
	Manifest     string    `json:"manifest"` // Full manifest JSON
	RegisteredAt time.Time `json:"registered_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Enabled      bool      `json:"enabled"`
}

// New creates a new tool registry.
func New(dbPath string, logger *zap.Logger) (*Registry, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	r := &Registry{
		db:     db,
		tools:  make(map[string]*manifest.ToolManifest),
		logger: logger,
	}

	// Load existing tools from database
	if err := r.loadFromDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("loading tools from database: %w", err)
	}

	return r, nil
}

func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tools (
		id TEXT NOT NULL,
		version TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		runtime TEXT NOT NULL,
		digest TEXT NOT NULL,
		manifest_path TEXT,
		manifest TEXT NOT NULL,
		registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		enabled INTEGER DEFAULT 1,
		PRIMARY KEY (id, version)
	);
	
	CREATE INDEX IF NOT EXISTS idx_tools_id ON tools(id);
	CREATE INDEX IF NOT EXISTS idx_tools_enabled ON tools(enabled);
	`

	_, err := db.Exec(schema)
	return err
}

func (r *Registry) loadFromDB() error {
	rows, err := r.db.Query(`
		SELECT id, version, name, description, runtime, digest, manifest_path, manifest, registered_at, updated_at, enabled
		FROM tools WHERE enabled = 1
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rec ToolRecord
		var manifestJSON string
		if err := rows.Scan(
			&rec.ID, &rec.Version, &rec.Name, &rec.Description,
			&rec.Runtime, &rec.Digest, &rec.ManifestPath,
			&manifestJSON, &rec.RegisteredAt, &rec.UpdatedAt, &rec.Enabled,
		); err != nil {
			return err
		}

		var m manifest.ToolManifest
		if err := json.Unmarshal([]byte(manifestJSON), &m); err != nil {
			r.logger.Warn("failed to unmarshal manifest", zap.String("tool_id", rec.ID), zap.Error(err))
			continue
		}

		key := fmt.Sprintf("%s@%s", rec.ID, rec.Version)
		r.tools[key] = &m
	}

	return rows.Err()
}

// Register adds or updates a tool in the registry.
func (r *Registry) Register(m *manifest.ToolManifest) error {
	if err := m.Validate(); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	manifestJSON, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, err = r.db.Exec(`
		INSERT INTO tools (id, version, name, description, runtime, digest, manifest_path, manifest, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(id, version) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			runtime = excluded.runtime,
			digest = excluded.digest,
			manifest_path = excluded.manifest_path,
			manifest = excluded.manifest,
			updated_at = CURRENT_TIMESTAMP
	`, m.ID, m.Version, m.Name, m.Description, m.Runtime, m.Digest, m.ManifestPath, string(manifestJSON))

	if err != nil {
		return fmt.Errorf("inserting tool: %w", err)
	}

	key := fmt.Sprintf("%s@%s", m.ID, m.Version)
	r.tools[key] = m

	r.logger.Info("registered tool",
		zap.String("id", m.ID),
		zap.String("version", m.Version),
		zap.String("digest", m.Digest),
	)

	return nil
}

// Unregister removes a tool from the registry.
func (r *Registry) Unregister(id, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`UPDATE tools SET enabled = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND version = ?`, id, version)
	if err != nil {
		return fmt.Errorf("disabling tool: %w", err)
	}

	key := fmt.Sprintf("%s@%s", id, version)
	delete(r.tools, key)

	r.logger.Info("unregistered tool", zap.String("id", id), zap.String("version", version))

	return nil
}

// Get retrieves a tool by ID. Returns the latest version if version is empty.
func (r *Registry) Get(id string, version string) (*manifest.ToolManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if version != "" {
		key := fmt.Sprintf("%s@%s", id, version)
		if m, ok := r.tools[key]; ok {
			return m, nil
		}
		return nil, fmt.Errorf("tool not found: %s@%s", id, version)
	}

	// Find latest version
	var latest *manifest.ToolManifest
	for key, m := range r.tools {
		if m.ID == id {
			if latest == nil || compareVersions(m.Version, latest.Version) > 0 {
				_ = key // suppress unused warning
				latest = m
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("tool not found: %s", id)
	}

	return latest, nil
}

// List returns all registered tools.
func (r *Registry) List() []*manifest.ToolManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*manifest.ToolManifest, 0, len(r.tools))
	for _, m := range r.tools {
		result = append(result, m)
	}
	return result
}

// ListByCapability returns tools that have a specific capability.
func (r *Registry) ListByCapability(cap string) []*manifest.ToolManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*manifest.ToolManifest
	for _, m := range r.tools {
		for _, c := range m.Capabilities.Extra {
			if c == cap {
				result = append(result, m)
				break
			}
		}
	}
	return result
}

// Discover scans directories for tool manifests and registers them.
func (r *Registry) Discover(dirs []string) (int, error) {
	tools, err := manifest.DiscoverTools(dirs)
	if err != nil {
		return 0, err
	}

	registered := 0
	for _, m := range tools {
		if err := r.Register(m); err != nil {
			r.logger.Warn("failed to register discovered tool",
				zap.String("path", m.ManifestPath),
				zap.Error(err),
			)
			continue
		}
		registered++
	}

	return registered, nil
}

// Close closes the registry database.
func (r *Registry) Close() error {
	return r.db.Close()
}

// Count returns the number of registered tools.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Simple version comparison (assumes semver format)
func compareVersions(v1, v2 string) int {
	// This is a simplified comparison; a proper implementation would use semver library
	if v1 == v2 {
		return 0
	}
	if v1 > v2 {
		return 1
	}
	return -1
}
