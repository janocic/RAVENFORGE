// Package pipeline implements the DAG-based pipeline executor.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Pipeline represents a pipeline definition.
type Pipeline struct {
	// Pipeline metadata
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Version     string            `yaml:"version" json:"version"`
	Metadata    map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`

	// Pipeline inputs
	Inputs []PipelineInput `yaml:"inputs" json:"inputs"`

	// Pipeline nodes (tool invocations)
	Nodes []Node `yaml:"nodes" json:"nodes"`

	// Pipeline outputs
	Outputs []PipelineOutput `yaml:"outputs" json:"outputs"`
}

// PipelineInput defines a pipeline-level input.
type PipelineInput struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool   `yaml:"required" json:"required"`
}

// PipelineOutput defines a pipeline-level output.
type PipelineOutput struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	FromNode    string `yaml:"from_node" json:"from_node"`
	FromOutput  string `yaml:"from_output" json:"from_output"`
}

// Node represents a tool invocation in the pipeline.
type Node struct {
	// Node ID (unique within pipeline)
	ID string `yaml:"id" json:"id"`
	// Tool to execute
	ToolID      string `yaml:"tool_id" json:"tool_id"`
	ToolVersion string `yaml:"tool_version,omitempty" json:"tool_version,omitempty"`
	// Node inputs (from other nodes or pipeline inputs)
	Inputs []NodeInput `yaml:"inputs" json:"inputs"`
	// Parameters to pass to the tool
	Params map[string]interface{} `yaml:"params,omitempty" json:"params,omitempty"`
	// Condition for execution
	Condition string `yaml:"condition,omitempty" json:"condition,omitempty"`
	// Continue on error
	ContinueOnError bool `yaml:"continue_on_error,omitempty" json:"continue_on_error,omitempty"`
}

// NodeInput defines an input to a node.
type NodeInput struct {
	Name string `yaml:"name" json:"name"`
	// Source: either a node output or a pipeline input
	FromNode   string `yaml:"from_node,omitempty" json:"from_node,omitempty"`
	FromOutput string `yaml:"from_output,omitempty" json:"from_output,omitempty"`
	FromInput  string `yaml:"from_input,omitempty" json:"from_input,omitempty"` // Pipeline input
}

// LoadFromFile loads a pipeline from a YAML file.
func LoadFromFile(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading pipeline file: %w", err)
	}

	var pipeline Pipeline
	if err := yaml.Unmarshal(data, &pipeline); err != nil {
		return nil, fmt.Errorf("parsing pipeline YAML: %w", err)
	}

	return &pipeline, nil
}

// Validate checks the pipeline for errors.
func (p *Pipeline) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("pipeline id is required")
	}
	if p.Name == "" {
		return fmt.Errorf("pipeline name is required")
	}
	if len(p.Nodes) == 0 {
		return fmt.Errorf("pipeline must have at least one node")
	}

	// Check for unique node IDs
	nodeIDs := make(map[string]bool)
	for _, node := range p.Nodes {
		if node.ID == "" {
			return fmt.Errorf("node id is required")
		}
		if nodeIDs[node.ID] {
			return fmt.Errorf("duplicate node id: %s", node.ID)
		}
		nodeIDs[node.ID] = true
	}

	// Check for valid dependencies
	for _, node := range p.Nodes {
		for _, input := range node.Inputs {
			if input.FromNode != "" && !nodeIDs[input.FromNode] {
				return fmt.Errorf("node %s references unknown node %s", node.ID, input.FromNode)
			}
		}
	}

	// Check for cycles
	if hasCycle(p.Nodes) {
		return fmt.Errorf("pipeline has circular dependencies")
	}

	return nil
}

// BuildDAG creates a directed acyclic graph from the pipeline nodes.
func (p *Pipeline) BuildDAG() (*DAG, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	dag := &DAG{
		Nodes:    make(map[string]*DAGNode),
		Edges:    make(map[string][]string),
		RevEdges: make(map[string][]string),
	}

	// Create DAG nodes
	for _, node := range p.Nodes {
		dag.Nodes[node.ID] = &DAGNode{
			ID:   node.ID,
			Node: node,
		}
	}

	// Create edges
	for _, node := range p.Nodes {
		for _, input := range node.Inputs {
			if input.FromNode != "" {
				dag.Edges[input.FromNode] = append(dag.Edges[input.FromNode], node.ID)
				dag.RevEdges[node.ID] = append(dag.RevEdges[node.ID], input.FromNode)
			}
		}
	}

	return dag, nil
}

// DAG represents the pipeline as a directed acyclic graph.
type DAG struct {
	Nodes    map[string]*DAGNode
	Edges    map[string][]string // Node -> downstream nodes
	RevEdges map[string][]string // Node -> upstream nodes (dependencies)
}

// DAGNode represents a node in the DAG.
type DAGNode struct {
	ID   string
	Node Node
}

// GetReadyNodes returns nodes with all dependencies satisfied.
func (d *DAG) GetReadyNodes(completed map[string]bool) []string {
	var ready []string

	for id := range d.Nodes {
		if completed[id] {
			continue
		}

		deps := d.RevEdges[id]
		allSatisfied := true
		for _, dep := range deps {
			if !completed[dep] {
				allSatisfied = false
				break
			}
		}

		if allSatisfied {
			ready = append(ready, id)
		}
	}

	return ready
}

// TopologicalSort returns nodes in topological order.
func (d *DAG) TopologicalSort() ([]string, error) {
	var order []string
	visited := make(map[string]bool)
	temp := make(map[string]bool)

	var visit func(string) error
	visit = func(id string) error {
		if temp[id] {
			return fmt.Errorf("cycle detected at node %s", id)
		}
		if visited[id] {
			return nil
		}

		temp[id] = true
		for _, dep := range d.RevEdges[id] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		temp[id] = false
		visited[id] = true
		order = append(order, id)
		return nil
	}

	for id := range d.Nodes {
		if !visited[id] {
			if err := visit(id); err != nil {
				return nil, err
			}
		}
	}

	return order, nil
}

func hasCycle(nodes []Node) bool {
	// Build adjacency list
	adj := make(map[string][]string)
	for _, node := range nodes {
		for _, input := range node.Inputs {
			if input.FromNode != "" {
				adj[input.FromNode] = append(adj[input.FromNode], node.ID)
			}
		}
	}

	// DFS for cycle detection
	white := make(map[string]bool)
	gray := make(map[string]bool)
	black := make(map[string]bool)

	for _, node := range nodes {
		white[node.ID] = true
	}

	var dfs func(string) bool
	dfs = func(u string) bool {
		white[u] = false
		gray[u] = true

		for _, v := range adj[u] {
			if white[v] && dfs(v) {
				return true
			}
			if gray[v] {
				return true
			}
		}

		gray[u] = false
		black[u] = true
		return false
	}

	for _, node := range nodes {
		if white[node.ID] {
			if dfs(node.ID) {
				return true
			}
		}
	}

	return false
}

// ExecutionPlan represents a plan for executing a pipeline.
type ExecutionPlan struct {
	PipelineID string
	RunID      string
	StartTime  time.Time
	Nodes      []*PlannedNode
}

// PlannedNode represents a node in the execution plan.
type PlannedNode struct {
	NodeID   string
	ToolID   string
	Inputs   map[string]string // Input name -> artifact ID
	Params   map[string]interface{}
	Status   string
	Error    string
	Outputs  map[string]string // Output name -> artifact ID
}

// Executor executes pipelines.
type Executor struct {
	logger *zap.Logger

	// Callbacks
	OnNodeStart    func(pipelineID, nodeID string)
	OnNodeComplete func(pipelineID, nodeID string, outputs map[string]string)
	OnNodeFailed   func(pipelineID, nodeID string, err error)
	
	// Tool executor
	ExecuteTool func(ctx context.Context, toolID, toolVersion string, inputs map[string]string, params map[string]interface{}) (map[string]string, error)
}

// NewExecutor creates a new pipeline executor.
func NewExecutor(logger *zap.Logger) *Executor {
	return &Executor{
		logger: logger,
	}
}

// PipelineRun represents an execution of a pipeline.
type PipelineRun struct {
	ID          string                 `json:"id"`
	PipelineID  string                 `json:"pipeline_id"`
	PipelineName string                `json:"pipeline_name"`
	Status      string                 `json:"status"`
	Inputs      map[string]string      `json:"inputs"`
	Outputs     map[string]string      `json:"outputs,omitempty"`
	NodeRuns    map[string]*NodeRun    `json:"node_runs"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// NodeRun represents the execution of a single node.
type NodeRun struct {
	NodeID    string            `json:"node_id"`
	ToolID    string            `json:"tool_id"`
	Status    string            `json:"status"`
	Inputs    map[string]string `json:"inputs"`
	Outputs   map[string]string `json:"outputs,omitempty"`
	StartTime *time.Time        `json:"start_time,omitempty"`
	EndTime   *time.Time        `json:"end_time,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// Execute runs a pipeline.
func (e *Executor) Execute(ctx context.Context, pipeline *Pipeline, inputs map[string]string) (*PipelineRun, error) {
	// Validate inputs
	for _, input := range pipeline.Inputs {
		if input.Required {
			if _, ok := inputs[input.Name]; !ok {
				return nil, fmt.Errorf("required input %s not provided", input.Name)
			}
		}
	}

	// Build DAG
	dag, err := pipeline.BuildDAG()
	if err != nil {
		return nil, err
	}

	// Initialize run
	run := &PipelineRun{
		ID:           uuid.New().String(),
		PipelineID:   pipeline.ID,
		PipelineName: pipeline.Name,
		Status:       "RUNNING",
		Inputs:       inputs,
		NodeRuns:     make(map[string]*NodeRun),
		StartTime:    time.Now().UTC(),
	}

	for _, node := range pipeline.Nodes {
		run.NodeRuns[node.ID] = &NodeRun{
			NodeID: node.ID,
			ToolID: node.ToolID,
			Status: "PENDING",
		}
	}

	e.logger.Info("starting pipeline execution",
		zap.String("run_id", run.ID),
		zap.String("pipeline_id", pipeline.ID),
	)

	// Execute nodes in parallel where possible
	completed := make(map[string]bool)
	outputs := make(map[string]map[string]string) // nodeID -> outputName -> artifactID
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(dag.Nodes))

	for {
		select {
		case <-ctx.Done():
			run.Status = "CANCELED"
			now := time.Now().UTC()
			run.EndTime = &now
			return run, ctx.Err()
		default:
		}

		mu.Lock()
		readyNodes := dag.GetReadyNodes(completed)
		mu.Unlock()

		if len(readyNodes) == 0 {
			// Check if all nodes are completed
			mu.Lock()
			allDone := len(completed) == len(dag.Nodes)
			mu.Unlock()
			if allDone {
				break
			}
			// Wait a bit for running nodes to complete
			time.Sleep(100 * time.Millisecond)
			continue
		}

		for _, nodeID := range readyNodes {
			nodeID := nodeID
			node := dag.Nodes[nodeID].Node

			// Mark as in-progress to prevent re-scheduling
			mu.Lock()
			if completed[nodeID] {
				mu.Unlock()
				continue
			}
			completed[nodeID] = true // Temporarily mark to prevent double scheduling
			mu.Unlock()

			wg.Add(1)
			go func() {
				defer wg.Done()

				// Build node inputs
				nodeInputs := make(map[string]string)
				for _, input := range node.Inputs {
					if input.FromInput != "" {
						if artifactID, ok := inputs[input.FromInput]; ok {
							nodeInputs[input.Name] = artifactID
						}
					} else if input.FromNode != "" {
						mu.Lock()
						if nodeOutputs, ok := outputs[input.FromNode]; ok {
							if artifactID, ok := nodeOutputs[input.FromOutput]; ok {
								nodeInputs[input.Name] = artifactID
							}
						}
						mu.Unlock()
					}
				}

				// Update node run status
				mu.Lock()
				now := time.Now().UTC()
				run.NodeRuns[nodeID].Status = "RUNNING"
				run.NodeRuns[nodeID].StartTime = &now
				run.NodeRuns[nodeID].Inputs = nodeInputs
				mu.Unlock()

				if e.OnNodeStart != nil {
					e.OnNodeStart(run.ID, nodeID)
				}

				e.logger.Info("executing node",
					zap.String("run_id", run.ID),
					zap.String("node_id", nodeID),
					zap.String("tool_id", node.ToolID),
				)

				// Execute tool
				var nodeOutputs map[string]string
				var execErr error

				if e.ExecuteTool != nil {
					nodeOutputs, execErr = e.ExecuteTool(ctx, node.ToolID, node.ToolVersion, nodeInputs, node.Params)
				}

				now = time.Now().UTC()
				mu.Lock()
				run.NodeRuns[nodeID].EndTime = &now
				if execErr != nil {
					run.NodeRuns[nodeID].Status = "FAILED"
					run.NodeRuns[nodeID].Error = execErr.Error()
					if !node.ContinueOnError {
						errCh <- fmt.Errorf("node %s failed: %w", nodeID, execErr)
					}
					if e.OnNodeFailed != nil {
						e.OnNodeFailed(run.ID, nodeID, execErr)
					}
				} else {
					run.NodeRuns[nodeID].Status = "SUCCEEDED"
					run.NodeRuns[nodeID].Outputs = nodeOutputs
					outputs[nodeID] = nodeOutputs
					if e.OnNodeComplete != nil {
						e.OnNodeComplete(run.ID, nodeID, nodeOutputs)
					}
				}
				mu.Unlock()

				e.logger.Info("node completed",
					zap.String("run_id", run.ID),
					zap.String("node_id", nodeID),
					zap.String("status", run.NodeRuns[nodeID].Status),
				)
			}()
		}

		// Check for errors
		select {
		case err := <-errCh:
			wg.Wait()
			run.Status = "FAILED"
			run.Error = err.Error()
			now := time.Now().UTC()
			run.EndTime = &now
			return run, err
		default:
		}
	}

	wg.Wait()

	// Collect pipeline outputs
	run.Outputs = make(map[string]string)
	for _, output := range pipeline.Outputs {
		if nodeOutputs, ok := outputs[output.FromNode]; ok {
			if artifactID, ok := nodeOutputs[output.FromOutput]; ok {
				run.Outputs[output.Name] = artifactID
			}
		}
	}

	run.Status = "SUCCEEDED"
	now := time.Now().UTC()
	run.EndTime = &now

	e.logger.Info("pipeline completed",
		zap.String("run_id", run.ID),
		zap.String("pipeline_id", pipeline.ID),
	)

	return run, nil
}

// ToJSON returns the pipeline as JSON.
func (p *Pipeline) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}
