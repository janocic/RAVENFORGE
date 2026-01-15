//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/ravenforge/ravenforge/core/test/harness"
)

func TestRavenForgeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	h := harness.NewTestHarness(t)
	h.Setup()
	defer h.Teardown()

	// Run all standard scenarios
	for _, scenario := range harness.StandardScenarios() {
		h.RunScenario(scenario)
	}
}

func TestToolRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	h := harness.NewTestHarness(t)
	h.Setup()
	defer h.Teardown()

	h.RunScenario(harness.ToolRegistrationScenario())
}

func TestArtifactLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	h := harness.NewTestHarness(t)
	h.Setup()
	defer h.Teardown()

	h.RunScenario(harness.ArtifactLifecycleScenario())
}

func TestJobExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	h := harness.NewTestHarness(t)
	h.Setup()
	defer h.Teardown()

	h.RunScenario(harness.JobExecutionScenario())
}

func TestPipelineExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	h := harness.NewTestHarness(t)
	h.Setup()
	defer h.Teardown()

	h.RunScenario(harness.PipelineExecutionScenario())
}

func TestAuditTrail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	h := harness.NewTestHarness(t)
	h.Setup()
	defer h.Teardown()

	h.RunScenario(harness.AuditTrailScenario())
}
