// Package manifest handles tool manifest parsing, validation, and management.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// ToolManifest represents the complete tool definition.
type ToolManifest struct {
	// Unique identifier for the tool (e.g., "ingest-jsonlines")
	ID string `yaml:"id" json:"id"`
	// Human-readable name
	Name string `yaml:"name" json:"name"`
	// Semantic version (e.g., "1.0.0")
	Version string `yaml:"version" json:"version"`
	// Tool description
	Description string `yaml:"description" json:"description"`
	// API version this manifest conforms to
	ToolAPIVersion string `yaml:"tool_api_version" json:"tool_api_version"`
	// Runtime type (oci, wasi)
	Runtime RuntimeType `yaml:"runtime" json:"runtime"`
	// Entrypoint configuration
	Entrypoint Entrypoint `yaml:"entrypoint" json:"entrypoint"`
	// Input specifications
	Inputs []IOSpec `yaml:"inputs" json:"inputs"`
	// Output specifications
	Outputs []IOSpec `yaml:"outputs" json:"outputs"`
	// Capability requirements
	Capabilities Capabilities `yaml:"capabilities" json:"capabilities"`
	// Resource limits
	Resources Resources `yaml:"resources" json:"resources"`
	// Additional metadata
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	// Path to the manifest file (set during loading)
	ManifestPath string `yaml:"-" json:"manifest_path,omitempty"`
	// SHA256 digest of the manifest (computed)
	Digest string `yaml:"-" json:"digest,omitempty"`
}

// RuntimeType defines the execution runtime.
type RuntimeType string

const (
	RuntimeOCI  RuntimeType = "oci"
	RuntimeWASI RuntimeType = "wasi"
)

// Entrypoint defines how to start the tool.
type Entrypoint struct {
	// OCI image reference
	Image string `yaml:"image,omitempty" json:"image,omitempty"`
	// Command to run
	Command []string `yaml:"command,omitempty" json:"command,omitempty"`
	// Arguments
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`
	// Working directory inside container
	WorkDir string `yaml:"workdir,omitempty" json:"workdir,omitempty"`
	// Environment variables (non-sensitive defaults)
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// IOSpec defines an input or output specification.
type IOSpec struct {
	// Name of the input/output
	Name string `yaml:"name" json:"name"`
	// Content type (e.g., "application/json", "text/plain")
	ContentType string `yaml:"content_type" json:"content_type"`
	// Reference to JSON Schema for validation
	SchemaRef string `yaml:"schema_ref,omitempty" json:"schema_ref,omitempty"`
	// Whether this input/output is required
	Required bool `yaml:"required" json:"required"`
	// Description
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// Capabilities defines what permissions the tool needs.
type Capabilities struct {
	// Filesystem mounts needed
	FSMounts []FSMount `yaml:"fs_mounts,omitempty" json:"fs_mounts,omitempty"`
	// Network access required
	Network bool `yaml:"network" json:"network"`
	// Secret names requested
	Secrets []string `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	// Whether the tool uses AI/ML
	UsesAI bool `yaml:"uses_ai" json:"uses_ai"`
	// Whether this is a response action
	ResponseAction bool `yaml:"response_action" json:"response_action"`
	// Additional capabilities
	Extra []string `yaml:"extra,omitempty" json:"extra,omitempty"`
}

// FSMount defines a filesystem mount.
type FSMount struct {
	// Path inside the container
	Path string `yaml:"path" json:"path"`
	// Whether the mount is read-only
	ReadOnly bool `yaml:"readonly" json:"readonly"`
	// Purpose description
	Purpose string `yaml:"purpose,omitempty" json:"purpose,omitempty"`
}

// Resources defines resource limits.
type Resources struct {
	// CPU limit (cores, e.g., 1.0)
	CPU float64 `yaml:"cpu" json:"cpu"`
	// Memory limit (e.g., "512Mi")
	Memory string `yaml:"memory" json:"memory"`
	// Execution timeout (e.g., "5m")
	Timeout string `yaml:"timeout" json:"timeout"`
	// Maximum PIDs
	MaxPids int64 `yaml:"max_pids" json:"max_pids"`
}

// LoadFromFile loads a tool manifest from a YAML file.
func LoadFromFile(path string) (*ToolManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest file: %w", err)
	}

	var manifest ToolManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest YAML: %w", err)
	}

	manifest.ManifestPath = path
	manifest.Digest = computeDigest(data)

	return &manifest, nil
}

// LoadFromDirectory finds and loads tool.yaml from a directory.
func LoadFromDirectory(dir string) (*ToolManifest, error) {
	manifestPath := filepath.Join(dir, "tool.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		// Try tool.yml as alternative
		manifestPath = filepath.Join(dir, "tool.yml")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("no tool.yaml or tool.yml found in %s", dir)
		}
	}

	return LoadFromFile(manifestPath)
}

// Validate checks the manifest for errors.
func (m *ToolManifest) Validate() error {
	var errs []string

	// Required fields
	if m.ID == "" {
		errs = append(errs, "id is required")
	} else if !isValidID(m.ID) {
		errs = append(errs, "id must be lowercase alphanumeric with hyphens")
	}

	if m.Name == "" {
		errs = append(errs, "name is required")
	}

	if m.Version == "" {
		errs = append(errs, "version is required")
	} else if !isValidVersion(m.Version) {
		errs = append(errs, "version must be valid semver (e.g., 1.0.0)")
	}

	if m.ToolAPIVersion == "" {
		errs = append(errs, "tool_api_version is required")
	}

	// Runtime validation
	switch m.Runtime {
	case RuntimeOCI:
		if m.Entrypoint.Image == "" {
			errs = append(errs, "entrypoint.image is required for OCI runtime")
		}
	case RuntimeWASI:
		// WASI is stubbed for MVP
		errs = append(errs, "WASI runtime is not yet supported")
	default:
		errs = append(errs, fmt.Sprintf("unsupported runtime: %s", m.Runtime))
	}

	// Input/output validation
	for i, input := range m.Inputs {
		if input.Name == "" {
			errs = append(errs, fmt.Sprintf("inputs[%d].name is required", i))
		}
		if input.ContentType == "" {
			errs = append(errs, fmt.Sprintf("inputs[%d].content_type is required", i))
		}
	}

	for i, output := range m.Outputs {
		if output.Name == "" {
			errs = append(errs, fmt.Sprintf("outputs[%d].name is required", i))
		}
		if output.ContentType == "" {
			errs = append(errs, fmt.Sprintf("outputs[%d].content_type is required", i))
		}
	}

	// Resource validation
	if m.Resources.CPU <= 0 {
		m.Resources.CPU = 1.0 // Default
	}
	if m.Resources.Memory == "" {
		m.Resources.Memory = "512Mi" // Default
	}
	if m.Resources.Timeout == "" {
		m.Resources.Timeout = "5m" // Default
	}

	if len(errs) > 0 {
		return fmt.Errorf("manifest validation errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// ValidateWithSchema validates the manifest against a JSON schema.
func (m *ToolManifest) ValidateWithSchema(schemaPath string) error {
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("reading schema file: %w", err)
	}

	manifestJSON, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling manifest to JSON: %w", err)
	}

	schemaLoader := gojsonschema.NewBytesLoader(schemaData)
	documentLoader := gojsonschema.NewBytesLoader(manifestJSON)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid() {
		var errMsgs []string
		for _, err := range result.Errors() {
			errMsgs = append(errMsgs, err.String())
		}
		return fmt.Errorf("schema validation failed: %s", strings.Join(errMsgs, "; "))
	}

	return nil
}

// ToJSON returns the manifest as JSON.
func (m *ToolManifest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// ToYAML returns the manifest as YAML.
func (m *ToolManifest) ToYAML() ([]byte, error) {
	return yaml.Marshal(m)
}

// HasDangerousCapabilities returns true if the tool has dangerous capabilities.
func (m *ToolManifest) HasDangerousCapabilities() bool {
	return m.Capabilities.Network ||
		m.Capabilities.UsesAI ||
		m.Capabilities.ResponseAction ||
		len(m.Capabilities.Secrets) > 0
}

// GetRequiredInputs returns the list of required input names.
func (m *ToolManifest) GetRequiredInputs() []string {
	var required []string
	for _, input := range m.Inputs {
		if input.Required {
			required = append(required, input.Name)
		}
	}
	return required
}

func computeDigest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func isValidID(id string) bool {
	matched, _ := regexp.MatchString(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`, id)
	return matched
}

func isValidVersion(version string) bool {
	// Simple semver validation
	matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?(\+[a-zA-Z0-9.]+)?$`, version)
	return matched
}

// DiscoverTools finds all tool manifests in the given directories.
func DiscoverTools(dirs []string) ([]*ToolManifest, error) {
	var tools []*ToolManifest
	seen := make(map[string]bool)

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // Skip non-existent directories
		}

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.Name() == "tool.yaml" || info.Name() == "tool.yml" {
				manifest, err := LoadFromFile(path)
				if err != nil {
					return fmt.Errorf("loading %s: %w", path, err)
				}

				// Deduplicate by ID + version
				key := fmt.Sprintf("%s@%s", manifest.ID, manifest.Version)
				if !seen[key] {
					seen[key] = true
					tools = append(tools, manifest)
				}
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("discovering tools in %s: %w", dir, err)
		}
	}

	return tools, nil
}
