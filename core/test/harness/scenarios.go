package harness

import (
	"net/http"
	"time"
)

// StandardScenarios returns a collection of standard test scenarios
func StandardScenarios() []TestScenario {
	return []TestScenario{
		ToolRegistrationScenario(),
		ArtifactLifecycleScenario(),
		JobExecutionScenario(),
		PipelineExecutionScenario(),
		AuditTrailScenario(),
	}
}

// ToolRegistrationScenario tests tool registration and discovery
func ToolRegistrationScenario() TestScenario {
	return TestScenario{
		Name:        "tool_registration",
		Description: "Tests tool registration, listing, and retrieval",
		Setup: func(h *TestHarness) {
			// Create test tools
			h.CreateTestTool("test-ingest", "ingest")
			h.CreateTestTool("test-detect", "detect")
			h.CreateTestTool("test-enrich", "enrich")
		},
		Run: func(h *TestHarness) {
			// Trigger discovery
			resp, err := h.POST("/v1/tools/discover", map[string]interface{}{
				"path": h.TempDir() + "/tools",
			})
			if err != nil {
				h.t.Fatalf("Discovery failed: %v", err)
			}
			h.AssertStatus(resp, http.StatusOK)
		},
		Verify: func(h *TestHarness) {
			// List tools
			resp, err := h.GET("/v1/tools")
			if err != nil {
				h.t.Fatalf("List failed: %v", err)
			}
			h.AssertStatus(resp, http.StatusOK)

			var tools []map[string]interface{}
			if err := h.ParseJSON(resp, &tools); err != nil {
				h.t.Fatalf("Parse failed: %v", err)
			}

			if len(tools) < 3 {
				h.t.Errorf("Expected at least 3 tools, got %d", len(tools))
			}

			// Get specific tool
			resp, err = h.GET("/v1/tools/test-ingest")
			if err != nil {
				h.t.Fatalf("Get tool failed: %v", err)
			}
			h.AssertStatus(resp, http.StatusOK)
		},
	}
}

// ArtifactLifecycleScenario tests artifact CRUD operations
func ArtifactLifecycleScenario() TestScenario {
	var artifactID string

	return TestScenario{
		Name:        "artifact_lifecycle",
		Description: "Tests artifact creation, retrieval, and deletion",
		Setup: func(h *TestHarness) {
			// Create test data file
			h.CreateTestArtifact("test-data.jsonl", `{"event":"test1"}
{"event":"test2"}
{"event":"test3"}
`)
		},
		Run: func(h *TestHarness) {
			// Create artifact
			resp, err := h.POST("/v1/artifacts", map[string]interface{}{
				"name":     "test-artifact",
				"type":     "events",
				"source":   "test",
				"filepath": h.TempDir() + "/test-artifacts/test-data.jsonl",
			})
			if err != nil {
				h.t.Fatalf("Create artifact failed: %v", err)
			}
			h.AssertStatus(resp, http.StatusCreated)

			var result map[string]interface{}
			if err := h.ParseJSON(resp, &result); err != nil {
				h.t.Fatalf("Parse failed: %v", err)
			}
			artifactID = result["id"].(string)
		},
		Verify: func(h *TestHarness) {
			// Get artifact
			resp, err := h.GET("/v1/artifacts/" + artifactID)
			if err != nil {
				h.t.Fatalf("Get artifact failed: %v", err)
			}
			h.AssertStatus(resp, http.StatusOK)

			// List artifacts
			resp, err = h.GET("/v1/artifacts")
			if err != nil {
				h.t.Fatalf("List artifacts failed: %v", err)
			}
			h.AssertStatus(resp, http.StatusOK)

			var artifacts []map[string]interface{}
			if err := h.ParseJSON(resp, &artifacts); err != nil {
				h.t.Fatalf("Parse failed: %v", err)
			}

			found := false
			for _, a := range artifacts {
				if a["id"] == artifactID {
					found = true
					break
				}
			}
			if !found {
				h.t.Error("Created artifact not found in list")
			}
		},
		Cleanup: func(h *TestHarness) {
			// Delete artifact
			if artifactID != "" {
				resp, err := h.DELETE("/v1/artifacts/" + artifactID)
				if err != nil {
					h.t.Logf("Cleanup failed: %v", err)
				} else {
					h.AssertStatus(resp, http.StatusNoContent)
				}
			}
		},
	}
}

// JobExecutionScenario tests job submission and execution
func JobExecutionScenario() TestScenario {
	var jobID string

	return TestScenario{
		Name:        "job_execution",
		Description: "Tests job submission, status tracking, and completion",
		Setup: func(h *TestHarness) {
			// Create test tool
			h.CreateTestTool("test-processor", "process")

			// Trigger discovery
			h.POST("/v1/tools/discover", map[string]interface{}{
				"path": h.TempDir() + "/tools",
			})

			// Create input artifact
			events := SampleEvents()
			h.WriteJSONLFile(h.TempDir()+"/test-input.jsonl", events)
		},
		Run: func(h *TestHarness) {
			// Submit job
			resp, err := h.POST("/v1/runs", map[string]interface{}{
				"tool": "test-processor",
				"inputs": map[string]interface{}{
					"data": h.TempDir() + "/test-input.jsonl",
				},
				"parameters": map[string]interface{}{
					"option": "test-value",
				},
			})
			if err != nil {
				h.t.Fatalf("Submit job failed: %v", err)
			}
			h.AssertStatus(resp, http.StatusAccepted)

			var result map[string]interface{}
			if err := h.ParseJSON(resp, &result); err != nil {
				h.t.Fatalf("Parse failed: %v", err)
			}
			jobID = result["job_id"].(string)
		},
		Verify: func(h *TestHarness) {
			// Wait for job completion
			job, err := h.WaitForJob(jobID, 60*time.Second)
			if err != nil {
				h.t.Fatalf("Wait for job failed: %v", err)
			}

			status := job["status"].(string)
			if status != "completed" && status != "failed" {
				h.t.Errorf("Unexpected job status: %s", status)
			}

			// Check job in list
			resp, err := h.GET("/v1/jobs")
			if err != nil {
				h.t.Fatalf("List jobs failed: %v", err)
			}
			h.AssertStatus(resp, http.StatusOK)
		},
	}
}

// PipelineExecutionScenario tests pipeline DAG execution
func PipelineExecutionScenario() TestScenario {
	var pipelineID string

	return TestScenario{
		Name:        "pipeline_execution",
		Description: "Tests pipeline creation and DAG execution",
		Setup: func(h *TestHarness) {
			// Create test tools
			h.CreateTestTool("step1", "process")
			h.CreateTestTool("step2", "process")
			h.CreateTestTool("step3", "process")

			// Trigger discovery
			h.POST("/v1/tools/discover", map[string]interface{}{
				"path": h.TempDir() + "/tools",
			})
		},
		Run: func(h *TestHarness) {
			// Create and run pipeline
			resp, err := h.POST("/v1/pipelines", map[string]interface{}{
				"name":        "test-pipeline",
				"description": "Test pipeline with DAG",
				"stages": []map[string]interface{}{
					{
						"name": "ingest",
						"tool": "step1",
						"inputs": map[string]interface{}{
							"data": "$INPUT.events",
						},
					},
					{
						"name":       "process",
						"tool":       "step2",
						"depends_on": []string{"ingest"},
						"inputs": map[string]interface{}{
							"data": "$STAGE.ingest.results",
						},
					},
					{
						"name":       "output",
						"tool":       "step3",
						"depends_on": []string{"process"},
						"inputs": map[string]interface{}{
							"data": "$STAGE.process.results",
						},
					},
				},
			})
			if err != nil {
				h.t.Fatalf("Create pipeline failed: %v", err)
			}
			h.AssertStatus(resp, http.StatusCreated)

			var result map[string]interface{}
			if err := h.ParseJSON(resp, &result); err != nil {
				h.t.Fatalf("Parse failed: %v", err)
			}
			pipelineID = result["id"].(string)
		},
		Verify: func(h *TestHarness) {
			// Get pipeline
			resp, err := h.GET("/v1/pipelines/" + pipelineID)
			if err != nil {
				h.t.Fatalf("Get pipeline failed: %v", err)
			}
			h.AssertStatus(resp, http.StatusOK)

			var pipeline map[string]interface{}
			if err := h.ParseJSON(resp, &pipeline); err != nil {
				h.t.Fatalf("Parse failed: %v", err)
			}

			// Verify stages
			stages, ok := pipeline["stages"].([]interface{})
			if !ok || len(stages) != 3 {
				h.t.Errorf("Expected 3 stages, got %v", len(stages))
			}
		},
	}
}

// AuditTrailScenario tests audit logging
func AuditTrailScenario() TestScenario {
	return TestScenario{
		Name:        "audit_trail",
		Description: "Tests audit log creation and retrieval",
		Setup: func(h *TestHarness) {
			// Perform some auditable actions
			h.CreateTestTool("audit-test", "test")
			h.POST("/v1/tools/discover", map[string]interface{}{
				"path": h.TempDir() + "/tools",
			})
		},
		Run: func(h *TestHarness) {
			// Actions already performed in setup
		},
		Verify: func(h *TestHarness) {
			// Get audit logs
			resp, err := h.GET("/v1/audit?limit=100")
			if err != nil {
				h.t.Fatalf("Get audit failed: %v", err)
			}
			h.AssertStatus(resp, http.StatusOK)

			var entries []map[string]interface{}
			if err := h.ParseJSON(resp, &entries); err != nil {
				h.t.Fatalf("Parse failed: %v", err)
			}

			if len(entries) == 0 {
				h.t.Error("Expected audit entries, got none")
			}

			// Verify audit entries have required fields
			for _, entry := range entries {
				if _, ok := entry["timestamp"]; !ok {
					h.t.Error("Audit entry missing timestamp")
				}
				if _, ok := entry["action"]; !ok {
					h.t.Error("Audit entry missing action")
				}
				if _, ok := entry["hash"]; !ok {
					h.t.Error("Audit entry missing hash")
				}
			}
		},
	}
}
