// Ravenforge is the CLI client for the Ravenforge platform.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ravenforge/ravenforge/core/internal/manifest"
	"github.com/ravenforge/ravenforge/core/internal/pipeline"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	serverURL string
	timeout   time.Duration
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ravenforge",
		Short: "Ravenforge CLI - Cybersecurity platform client",
		Long: `Ravenforge is an open-source, tool-based cybersecurity platform.
This CLI provides commands to interact with the Ravenforge daemon,
manage tools, execute pipelines, and retrieve artifacts.`,
	}

	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://127.0.0.1:7433", "Ravenforge daemon URL")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout")

	// Add subcommands
	rootCmd.AddCommand(toolCmd())
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(pipelineCmd())
	rootCmd.AddCommand(jobCmd())
	rootCmd.AddCommand(artifactCmd())
	rootCmd.AddCommand(auditCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// Tool commands

func toolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Manage tools",
	}

	cmd.AddCommand(toolListCmd())
	cmd.AddCommand(toolInfoCmd())
	cmd.AddCommand(toolValidateCmd())
	cmd.AddCommand(toolScaffoldCmd())

	return cmd
}

func toolListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/v1/tools")
			if err != nil {
				return err
			}

			var tools []map[string]interface{}
			if err := json.Unmarshal(resp, &tools); err != nil {
				return err
			}

			fmt.Printf("%-30s %-15s %-50s\n", "ID", "VERSION", "NAME")
			fmt.Println(strings.Repeat("-", 95))
			for _, tool := range tools {
				fmt.Printf("%-30s %-15s %-50s\n",
					tool["id"],
					tool["version"],
					tool["name"],
				)
			}

			return nil
		},
	}
}

func toolInfoCmd() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "info <tool-id>",
		Short: "Show tool details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolID := args[0]
			path := fmt.Sprintf("/v1/tools/%s", toolID)
			if version != "" {
				path += "?version=" + version
			}

			resp, err := apiGet(path)
			if err != nil {
				return err
			}

			var tool map[string]interface{}
			if err := json.Unmarshal(resp, &tool); err != nil {
				return err
			}

			output, _ := json.MarshalIndent(tool, "", "  ")
			fmt.Println(string(output))

			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "Specific tool version")

	return cmd
}

func toolValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate a tool manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			m, err := manifest.LoadFromDirectory(path)
			if err != nil {
				m, err = manifest.LoadFromFile(path)
				if err != nil {
					return fmt.Errorf("loading manifest: %w", err)
				}
			}

			if err := m.Validate(); err != nil {
				fmt.Printf("❌ Validation failed: %s\n", err)
				return err
			}

			fmt.Printf("✅ Manifest valid: %s@%s\n", m.ID, m.Version)
			return nil
		},
	}
}

func toolScaffoldCmd() *cobra.Command {
	var name, runtime, outputDir string

	cmd := &cobra.Command{
		Use:   "scaffold",
		Short: "Create a new tool scaffold",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			targetDir := filepath.Join(outputDir, name)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				return err
			}

			// Create tool.yaml
			manifest := map[string]interface{}{
				"id":               name,
				"name":             strings.Title(strings.ReplaceAll(name, "-", " ")),
				"version":          "1.0.0",
				"description":      "TODO: Add description",
				"tool_api_version": "1.0",
				"runtime":          runtime,
				"entrypoint": map[string]interface{}{
					"image":   fmt.Sprintf("ravenforge/%s:latest", name),
					"command": []string{"/app/run"},
				},
				"inputs": []map[string]interface{}{
					{
						"name":         "input",
						"content_type": "application/json",
						"required":     true,
					},
				},
				"outputs": []map[string]interface{}{
					{
						"name":         "output",
						"content_type": "application/json",
					},
				},
				"capabilities": map[string]interface{}{
					"network":         false,
					"uses_ai":         false,
					"response_action": false,
				},
				"resources": map[string]interface{}{
					"cpu":     1.0,
					"memory":  "512Mi",
					"timeout": "5m",
				},
			}

			manifestYAML, _ := yaml.Marshal(manifest)
			if err := os.WriteFile(filepath.Join(targetDir, "tool.yaml"), manifestYAML, 0644); err != nil {
				return err
			}

			// Create Dockerfile
			dockerfile := `FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

ENTRYPOINT ["python", "main.py"]
`
			if err := os.WriteFile(filepath.Join(targetDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
				return err
			}

			// Create main.py
			mainPy := `#!/usr/bin/env python3
"""` + name + ` tool implementation."""

import json
import os
import sys
from pathlib import Path

INPUT_DIR = Path(os.environ.get("RF_INPUT_DIR", "/rf/in"))
OUTPUT_DIR = Path(os.environ.get("RF_OUTPUT_DIR", "/rf/out"))


def main():
    """Main entry point."""
    # Read inputs
    for input_file in INPUT_DIR.iterdir():
        print(json.dumps({"level": "info", "msg": f"Processing {input_file.name}"}))
        
        with open(input_file) as f:
            data = json.load(f)
        
        # TODO: Process data
        result = {"processed": True, "input": input_file.name}
        
        # Write output
        output_file = OUTPUT_DIR / f"{input_file.stem}_output.json"
        with open(output_file, "w") as f:
            json.dump(result, f, indent=2)
        
        print(json.dumps({"level": "info", "msg": f"Wrote {output_file.name}"}))

    # Write result metadata
    result_meta = {
        "status": "success",
        "outputs": [str(f.name) for f in OUTPUT_DIR.iterdir()]
    }
    with open(OUTPUT_DIR / "result.json", "w") as f:
        json.dump(result_meta, f, indent=2)

    return 0


if __name__ == "__main__":
    sys.exit(main())
`
			if err := os.WriteFile(filepath.Join(targetDir, "main.py"), []byte(mainPy), 0644); err != nil {
				return err
			}

			// Create requirements.txt
			if err := os.WriteFile(filepath.Join(targetDir, "requirements.txt"), []byte("# Add dependencies here\n"), 0644); err != nil {
				return err
			}

			// Create README.md
			readme := fmt.Sprintf(`# %s

## Description

TODO: Add description

## Inputs

| Name | Type | Required | Description |
|------|------|----------|-------------|
| input | application/json | Yes | Input data |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| output | application/json | Processed output |

## Usage

~~~bash
ravenforge run %s --input <artifact-id>
~~~

## Development

~~~bash
# Build
docker build -t ravenforge/%s:latest .

# Test locally
docker run -v $(pwd)/test-input:/rf/in -v $(pwd)/test-output:/rf/out ravenforge/%s:latest
~~~
`, name, name, name, name)
			if err := os.WriteFile(filepath.Join(targetDir, "README.md"), []byte(readme), 0644); err != nil {
				return err
			}

			fmt.Printf("✅ Created tool scaffold at %s\n", targetDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Tool name (required)")
	cmd.Flags().StringVar(&runtime, "runtime", "oci", "Runtime type (oci)")
	cmd.Flags().StringVar(&outputDir, "output", ".", "Output directory")

	return cmd
}

// Run commands

func runCmd() *cobra.Command {
	var inputFile string
	var inputArtifact string
	var params string

	cmd := &cobra.Command{
		Use:   "run <tool-id>",
		Short: "Execute a tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolID := args[0]

			request := map[string]interface{}{
				"tool_id": toolID,
				"inputs":  map[string]string{},
				"params":  map[string]interface{}{},
			}

			if inputArtifact != "" {
				request["inputs"] = map[string]string{
					"input": inputArtifact,
				}
			}

			if params != "" {
				var p map[string]interface{}
				if err := json.Unmarshal([]byte(params), &p); err != nil {
					return fmt.Errorf("parsing params JSON: %w", err)
				}
				request["params"] = p
			}

			body, _ := json.Marshal(request)
			resp, err := apiPost("/v1/runs", body)
			if err != nil {
				return err
			}

			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}

			fmt.Printf("✅ Run submitted\n")
			fmt.Printf("   Run ID: %s\n", result["run_id"])
			fmt.Printf("   Job ID: %s\n", result["job_id"])

			return nil
		},
	}

	cmd.Flags().StringVarP(&inputFile, "input-file", "f", "", "Input file path")
	cmd.Flags().StringVarP(&inputArtifact, "input", "i", "", "Input artifact ID")
	cmd.Flags().StringVarP(&params, "params", "p", "", "Parameters as JSON")

	return cmd
}

// Pipeline commands

func pipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Manage pipelines",
	}

	cmd.AddCommand(pipelineRunCmd())
	cmd.AddCommand(pipelineValidateCmd())

	return cmd
}

func pipelineRunCmd() *cobra.Command {
	var inputsJSON string

	cmd := &cobra.Command{
		Use:   "run <pipeline.yaml>",
		Short: "Execute a pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pipelinePath := args[0]

			p, err := pipeline.LoadFromFile(pipelinePath)
			if err != nil {
				return fmt.Errorf("loading pipeline: %w", err)
			}

			inputs := make(map[string]string)
			if inputsJSON != "" {
				if err := json.Unmarshal([]byte(inputsJSON), &inputs); err != nil {
					return fmt.Errorf("parsing inputs JSON: %w", err)
				}
			}

			request := map[string]interface{}{
				"pipeline": p,
				"inputs":   inputs,
			}

			body, _ := json.Marshal(request)
			resp, err := apiPost("/v1/pipelines", body)
			if err != nil {
				return err
			}

			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}

			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(output))

			return nil
		},
	}

	cmd.Flags().StringVar(&inputsJSON, "inputs", "", "Pipeline inputs as JSON (artifact IDs)")

	return cmd
}

func pipelineValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <pipeline.yaml>",
		Short: "Validate a pipeline definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pipelinePath := args[0]

			p, err := pipeline.LoadFromFile(pipelinePath)
			if err != nil {
				return fmt.Errorf("loading pipeline: %w", err)
			}

			if err := p.Validate(); err != nil {
				fmt.Printf("❌ Validation failed: %s\n", err)
				return err
			}

			fmt.Printf("✅ Pipeline valid: %s (%d nodes)\n", p.Name, len(p.Nodes))
			return nil
		},
	}
}

// Job commands

func jobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Manage jobs",
	}

	cmd.AddCommand(jobStatusCmd())
	cmd.AddCommand(jobListCmd())
	cmd.AddCommand(jobCancelCmd())

	return cmd
}

func jobStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <job-id>",
		Short: "Get job status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]

			resp, err := apiGet(fmt.Sprintf("/v1/jobs/%s", jobID))
			if err != nil {
				return err
			}

			var job map[string]interface{}
			if err := json.Unmarshal(resp, &job); err != nil {
				return err
			}

			output, _ := json.MarshalIndent(job, "", "  ")
			fmt.Println(string(output))

			return nil
		},
	}
}

func jobListCmd() *cobra.Command {
	var status string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/v1/jobs"
			params := []string{}
			if status != "" {
				params = append(params, "status="+status)
			}
			if limit > 0 {
				params = append(params, fmt.Sprintf("limit=%d", limit))
			}
			if len(params) > 0 {
				path += "?" + strings.Join(params, "&")
			}

			resp, err := apiGet(path)
			if err != nil {
				return err
			}

			var jobs []map[string]interface{}
			if err := json.Unmarshal(resp, &jobs); err != nil {
				return err
			}

			fmt.Printf("%-36s %-20s %-15s %-20s\n", "JOB ID", "TOOL", "STATUS", "CREATED")
			fmt.Println(strings.Repeat("-", 95))
			for _, job := range jobs {
				fmt.Printf("%-36s %-20s %-15s %-20s\n",
					job["id"],
					job["tool_id"],
					job["status"],
					job["created_at"],
				)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of results")

	return cmd
}

func jobCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <job-id>",
		Short: "Cancel a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]

			_, err := apiPost(fmt.Sprintf("/v1/jobs/%s/cancel", jobID), nil)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Job %s canceled\n", jobID)
			return nil
		},
	}
}

// Artifact commands

func artifactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifact",
		Short: "Manage artifacts",
	}

	cmd.AddCommand(artifactGetCmd())
	cmd.AddCommand(artifactDownloadCmd())

	return cmd
}

func artifactGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <artifact-id>",
		Short: "Get artifact metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			artifactID := args[0]

			resp, err := apiGet(fmt.Sprintf("/v1/artifacts/%s", artifactID))
			if err != nil {
				return err
			}

			var artifact map[string]interface{}
			if err := json.Unmarshal(resp, &artifact); err != nil {
				return err
			}

			output, _ := json.MarshalIndent(artifact, "", "  ")
			fmt.Println(string(output))

			return nil
		},
	}
}

func artifactDownloadCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "download <artifact-id>",
		Short: "Download artifact data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			artifactID := args[0]

			resp, err := http.Get(fmt.Sprintf("%s/v1/artifacts/%s/data", serverURL, artifactID))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server error: %s", string(body))
			}

			var out io.Writer = os.Stdout
			if outputPath != "" {
				f, err := os.Create(outputPath)
				if err != nil {
					return err
				}
				defer f.Close()
				out = f
			}

			_, err = io.Copy(out, resp.Body)
			return err
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path")

	return cmd
}

// Audit commands

func auditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "View audit logs",
	}

	cmd.AddCommand(auditTailCmd())

	return cmd
}

func auditTailCmd() *cobra.Command {
	var limit int
	var since uint64

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail audit log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/v1/audit/tail"
			params := []string{}
			if limit > 0 {
				params = append(params, fmt.Sprintf("limit=%d", limit))
			}
			if since > 0 {
				params = append(params, fmt.Sprintf("since=%d", since))
			}
			if len(params) > 0 {
				path += "?" + strings.Join(params, "&")
			}

			resp, err := apiGet(path)
			if err != nil {
				return err
			}

			var entries []map[string]interface{}
			if err := json.Unmarshal(resp, &entries); err != nil {
				return err
			}

			for _, entry := range entries {
				output, _ := json.Marshal(entry)
				fmt.Println(string(output))
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Number of entries")
	cmd.Flags().Uint64Var(&since, "since", 0, "Entries since sequence number")

	return cmd
}

// Version command

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Ravenforge CLI v1.0.0")
			fmt.Println("Tool API Version: 1.0")
		},
	}
}

// API helpers

func apiGet(path string) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(serverURL + path)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func apiPost(path string, data []byte) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	
	var body io.Reader
	if data != nil {
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest("POST", serverURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
