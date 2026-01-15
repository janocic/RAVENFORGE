// Package policy implements the policy engine for tool execution authorization.
package policy

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ravenforge/ravenforge/core/internal/manifest"
	"gopkg.in/yaml.v3"
)

// Engine evaluates policies for tool execution.
type Engine struct {
	policy  *Policy
	version string
	mu      sync.RWMutex
}

// Policy defines the complete policy configuration.
type Policy struct {
	// Policy version for auditing
	Version string `yaml:"version"`
	// Default action when no rules match
	DefaultAction Action `yaml:"default_action"`
	// Tool-specific rules
	Rules []Rule `yaml:"rules"`
	// Capability gates
	Gates []Gate `yaml:"gates"`
	// Global resource limits
	Limits ResourcePolicy `yaml:"limits"`
}

// Action defines a policy action.
type Action string

const (
	ActionAllow  Action = "allow"
	ActionDeny   Action = "deny"
	ActionAudit  Action = "audit" // Allow but log prominently
)

// Rule defines a policy rule.
type Rule struct {
	// Rule ID for auditing
	ID string `yaml:"id"`
	// Description
	Description string `yaml:"description,omitempty"`
	// Match criteria
	Match RuleMatch `yaml:"match"`
	// Action to take
	Action Action `yaml:"action"`
	// Conditions that must be met
	Conditions []Condition `yaml:"conditions,omitempty"`
}

// RuleMatch defines what a rule matches against.
type RuleMatch struct {
	// Tool IDs (supports wildcards)
	Tools []string `yaml:"tools,omitempty"`
	// Tool versions
	Versions []string `yaml:"versions,omitempty"`
	// Capabilities requested
	Capabilities []string `yaml:"capabilities,omitempty"`
	// All conditions must match if true, any if false
	MatchAll bool `yaml:"match_all"`
}

// Condition defines an additional condition for a rule.
type Condition struct {
	Type  string `yaml:"type"` // "time_range", "actor_type", etc.
	Value string `yaml:"value"`
}

// Gate defines a capability gate requiring approval.
type Gate struct {
	// Gate ID
	ID string `yaml:"id"`
	// Capabilities that trigger this gate
	Capabilities []string `yaml:"capabilities"`
	// Type of approval required
	ApprovalType string `yaml:"approval_type"` // "manual", "auto", "deny"
	// Description
	Description string `yaml:"description,omitempty"`
}

// ResourcePolicy defines global resource limits.
type ResourcePolicy struct {
	// Maximum CPU per tool
	MaxCPU float64 `yaml:"max_cpu"`
	// Maximum memory per tool (bytes)
	MaxMemory int64 `yaml:"max_memory"`
	// Maximum execution time
	MaxTimeout string `yaml:"max_timeout"`
	// Maximum concurrent runs per tool
	MaxConcurrent int `yaml:"max_concurrent"`
}

// Decision represents a policy evaluation result.
type Decision struct {
	Allowed       bool
	Action        Action
	MatchedRules  []string
	RequiredGates []string
	Reason        string
	PolicyVersion string
	EvaluatedAt   time.Time
}

// New creates a new policy engine.
func New() *Engine {
	return &Engine{
		policy: DefaultPolicy(),
	}
}

// DefaultPolicy returns a secure default policy.
func DefaultPolicy() *Policy {
	return &Policy{
		Version:       "1.0.0",
		DefaultAction: ActionDeny,
		Rules: []Rule{
			{
				ID:          "allow-registered",
				Description: "Allow all registered tools by default",
				Match: RuleMatch{
					Tools:    []string{"*"},
					MatchAll: false,
				},
				Action: ActionAllow,
			},
		},
		Gates: []Gate{
			{
				ID:           "network-gate",
				Capabilities: []string{"network"},
				ApprovalType: "manual",
				Description:  "Tools requesting network access require approval",
			},
			{
				ID:           "ai-gate",
				Capabilities: []string{"uses_ai"},
				ApprovalType: "manual",
				Description:  "AI/ML tools require approval",
			},
			{
				ID:           "response-gate",
				Capabilities: []string{"response_action"},
				ApprovalType: "deny",
				Description:  "Response actions are denied by default",
			},
			{
				ID:           "secrets-gate",
				Capabilities: []string{"secrets"},
				ApprovalType: "manual",
				Description:  "Secret access requires approval",
			},
		},
		Limits: ResourcePolicy{
			MaxCPU:        2.0,
			MaxMemory:     1 << 30, // 1GB
			MaxTimeout:    "30m",
			MaxConcurrent: 10,
		},
	}
}

// LoadFromFile loads a policy from a YAML file.
func (e *Engine) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Use default policy
		}
		return fmt.Errorf("reading policy file: %w", err)
	}

	var policy Policy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return fmt.Errorf("parsing policy: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.policy = &policy
	e.version = policy.Version

	return nil
}

// Evaluate evaluates the policy for a tool manifest.
func (e *Engine) Evaluate(m *manifest.ToolManifest) Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()

	decision := Decision{
		PolicyVersion: e.policy.Version,
		EvaluatedAt:   time.Now().UTC(),
	}

	// Check gates first
	requiredGates := e.checkGates(m)
	if len(requiredGates) > 0 {
		decision.RequiredGates = requiredGates
		
		// Check if any gate denies
		for _, gateID := range requiredGates {
			for _, gate := range e.policy.Gates {
				if gate.ID == gateID && gate.ApprovalType == "deny" {
					decision.Allowed = false
					decision.Action = ActionDeny
					decision.Reason = fmt.Sprintf("gate %s denies capability", gateID)
					return decision
				}
			}
		}
	}

	// Evaluate rules
	var matchedRules []string
	var finalAction Action = e.policy.DefaultAction

	for _, rule := range e.policy.Rules {
		if e.ruleMatches(&rule, m) {
			matchedRules = append(matchedRules, rule.ID)
			finalAction = rule.Action
			// Last matching rule wins
		}
	}

	decision.MatchedRules = matchedRules
	decision.Action = finalAction
	decision.Allowed = (finalAction == ActionAllow || finalAction == ActionAudit)

	if !decision.Allowed {
		decision.Reason = "denied by policy"
	}

	return decision
}

func (e *Engine) checkGates(m *manifest.ToolManifest) []string {
	var required []string

	capabilities := e.extractCapabilities(m)

	for _, gate := range e.policy.Gates {
		for _, gateCap := range gate.Capabilities {
			for _, toolCap := range capabilities {
				if gateCap == toolCap {
					required = append(required, gate.ID)
					break
				}
			}
		}
	}

	return required
}

func (e *Engine) extractCapabilities(m *manifest.ToolManifest) []string {
	var caps []string

	if m.Capabilities.Network {
		caps = append(caps, "network")
	}
	if m.Capabilities.UsesAI {
		caps = append(caps, "uses_ai")
	}
	if m.Capabilities.ResponseAction {
		caps = append(caps, "response_action")
	}
	if len(m.Capabilities.Secrets) > 0 {
		caps = append(caps, "secrets")
	}

	caps = append(caps, m.Capabilities.Extra...)

	return caps
}

func (e *Engine) ruleMatches(rule *Rule, m *manifest.ToolManifest) bool {
	// Check tool ID match
	toolMatches := len(rule.Match.Tools) == 0 // Empty means match all
	for _, pattern := range rule.Match.Tools {
		if matchPattern(pattern, m.ID) {
			toolMatches = true
			break
		}
	}

	if !toolMatches {
		return false
	}

	// Check version match
	if len(rule.Match.Versions) > 0 {
		versionMatches := false
		for _, pattern := range rule.Match.Versions {
			if matchPattern(pattern, m.Version) {
				versionMatches = true
				break
			}
		}
		if !versionMatches {
			return false
		}
	}

	// Check capability match
	if len(rule.Match.Capabilities) > 0 {
		toolCaps := e.extractCapabilities(m)
		capMatches := false
		for _, reqCap := range rule.Match.Capabilities {
			for _, toolCap := range toolCaps {
				if reqCap == toolCap {
					capMatches = true
					break
				}
			}
			if capMatches {
				break
			}
		}
		if !capMatches {
			return false
		}
	}

	return true
}

// matchPattern matches a string against a glob-like pattern.
func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	// Simple prefix/suffix matching
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(value) >= len(prefix) && value[:len(prefix)] == prefix
	}
	if len(pattern) > 0 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(value) >= len(suffix) && value[len(value)-len(suffix):] == suffix
	}
	return pattern == value
}

// ValidateResources checks if resource requests are within policy limits.
func (e *Engine) ValidateResources(m *manifest.ToolManifest) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if m.Resources.CPU > e.policy.Limits.MaxCPU {
		return fmt.Errorf("CPU request %.2f exceeds limit %.2f", m.Resources.CPU, e.policy.Limits.MaxCPU)
	}

	// Memory parsing would go here
	// Timeout parsing would go here

	return nil
}

// GetPolicy returns the current policy (for auditing).
func (e *Engine) GetPolicy() *Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.policy
}

// GetVersion returns the policy version.
func (e *Engine) GetVersion() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.version
}

// SaveToFile saves the current policy to a file.
func (e *Engine) SaveToFile(path string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	data, err := yaml.Marshal(e.policy)
	if err != nil {
		return fmt.Errorf("marshaling policy: %w", err)
	}

	return os.WriteFile(path, data, 0640)
}
