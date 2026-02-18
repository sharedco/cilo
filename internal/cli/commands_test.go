// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/sharedco/cilo/internal/cilod"
	"github.com/sharedco/cilo/internal/models"
	"github.com/sharedco/cilo/internal/state"
	"github.com/spf13/cobra"
)

type mockCilodClient struct {
	environments []cilod.Environment
	err          error
}

func (m *mockCilodClient) ListEnvironments() ([]cilod.Environment, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.environments, nil
}

// testListEnvironments is a helper to run the list command and capture output
func testListEnvironments(t *testing.T, args []string) (string, error) {
	// Create a new command for testing
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			allFlag, _ := cmd.Flags().GetBool("all")
			projectFilter, _ := cmd.Flags().GetString("project")

			envs, err := state.ListEnvironments()
			if err != nil {
				return err
			}

			// Get connected machines and their environments
			machines, err := ListConnectedMachines()
			if err != nil {
				return err
			}

			var unifiedEnvs []envWithMachine

			// Add local environments
			for _, env := range envs {
				unifiedEnvs = append(unifiedEnvs, envWithMachine{
					Environment: env,
					Machine:     "local",
				})
			}

			// Add remote environments from each machine
			for _, machine := range machines {
				client := cilod.NewClient(machine.WGAssignedIP, machine.Token)
				client.SetTimeout(100 * time.Millisecond)
				remoteEnvs, err := client.ListEnvironments()
				if err != nil {
					unifiedEnvs = append(unifiedEnvs, envWithMachine{
						Environment: &models.Environment{
							Name:    "(unreachable)",
							Project: "",
							Status:  "unreachable",
						},
						Machine: machine.Host,
					})
					continue
				}

				for _, remoteEnv := range remoteEnvs {
					// Convert cilod.Environment to models.Environment
					services := make(map[string]*models.Service)
					for _, svcName := range remoteEnv.Services {
						services[svcName] = &models.Service{Name: svcName}
					}
					unifiedEnvs = append(unifiedEnvs, envWithMachine{
						Environment: &models.Environment{
							Name:      remoteEnv.Name,
							Project:   "", // Will be determined from context
							Status:    remoteEnv.Status,
							CreatedAt: remoteEnv.CreatedAt,
							Services:  services,
						},
						Machine: machine.Host,
					})
				}
			}

			// Filter by project if not --all
			if !allFlag {
				var currentProject string
				if projectFilter != "" {
					currentProject = projectFilter
				} else {
					config, _ := models.LoadProjectConfig()
					if config != nil {
						currentProject = config.Project
					}
				}

				if currentProject != "" {
					filtered := make([]envWithMachine, 0)
					for _, ewm := range unifiedEnvs {
						if ewm.Environment.Project == currentProject || ewm.Environment.Name == "(unreachable)" {
							filtered = append(filtered, ewm)
						}
					}
					unifiedEnvs = filtered
					cmd.Printf("Environments for project: %s\n\n", currentProject)
				}
			}

			if len(unifiedEnvs) == 0 {
				if allFlag {
					cmd.Println("No environments found")
				} else {
					cmd.Println("No environments found for this project")
					cmd.Println("Use --all to see all environments")
				}
				return nil
			}

			// Output based on format
			switch format {
			case "json":
				return outputListJSON(cmd, unifiedEnvs, allFlag)
			case "quiet":
				return outputListQuiet(cmd, unifiedEnvs, allFlag)
			default:
				return outputListTable(cmd, unifiedEnvs, allFlag)
			}
		},
	}

	cmd.Flags().String("format", "table", "Output format")
	cmd.Flags().Bool("all", false, "Show all environments")
	cmd.Flags().String("project", "", "Filter to specific project")

	cmd.SetArgs(args)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	return buf.String(), err
}

func outputListTable(cmd *cobra.Command, envs []envWithMachine, all bool) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	if all {
		fmt.Fprintln(w, "ENVIRONMENT\tPROJECT\tSTATUS\tMACHINE\tSERVICES\t")
		fmt.Fprintln(w, "-----------\t-------\t------\t-------\t--------\t")
	} else {
		fmt.Fprintln(w, "ENVIRONMENT\tSTATUS\tMACHINE\tSERVICES\t")
		fmt.Fprintln(w, "-----------\t------\t-------\t--------\t")
	}

	for _, ewm := range envs {
		services := make([]string, 0, len(ewm.Services))
		for name := range ewm.Services {
			services = append(services, name)
		}
		serviceList := strings.Join(services, ", ")
		if len(serviceList) > 30 {
			serviceList = serviceList[:27] + "..."
		}

		if all {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t\n",
				ewm.Name, ewm.Project, ewm.Status, ewm.Machine, serviceList)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n",
				ewm.Name, ewm.Status, ewm.Machine, serviceList)
		}
	}

	return w.Flush()
}

func outputListJSON(cmd *cobra.Command, envs []envWithMachine, all bool) error {
	var output []map[string]interface{}

	for _, ewm := range envs {
		if !all && ewm.Status == "stopped" {
			continue
		}

		services := make([]map[string]string, 0, len(ewm.Services))
		for _, svc := range ewm.Services {
			services = append(services, map[string]string{
				"name": svc.Name,
				"url":  svc.URL,
				"ip":   svc.IP,
			})
		}

		output = append(output, map[string]interface{}{
			"name":       ewm.Name,
			"project":    ewm.Project,
			"status":     ewm.Status,
			"machine":    ewm.Machine,
			"created_at": ewm.CreatedAt,
			"subnet":     ewm.Subnet,
			"services":   services,
		})
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputListQuiet(cmd *cobra.Command, envs []envWithMachine, all bool) error {
	for _, ewm := range envs {
		if !all && ewm.Status == "stopped" {
			continue
		}
		fmt.Fprintln(cmd.OutOrStdout(), ewm.Name)
	}
	return nil
}

// newTabWriter creates a tabwriter for formatted output
func newTabWriter(out interface{}) *tabWriter {
	return &tabWriter{out: out}
}

type tabWriter struct {
	out interface{}
}

func (w *tabWriter) Flush() error {
	return nil
}

func setupTestState(t *testing.T) func() {
	tempDir, err := os.MkdirTemp("", "cilo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	ciloHome := filepath.Join(tempDir, ".cilo")
	if err := os.MkdirAll(ciloHome, 0755); err != nil {
		t.Fatalf("Failed to create .cilo dir: %v", err)
	}

	oldCiloUserHome := os.Getenv("CILO_USER_HOME")
	os.Setenv("CILO_USER_HOME", tempDir)

	if err := state.InitializeState("10.224.", 5354); err != nil {
		t.Fatalf("Failed to initialize state: %v", err)
	}

	return func() {
		os.Setenv("CILO_USER_HOME", oldCiloUserHome)
		os.RemoveAll(tempDir)
	}
}

// createTestEnvironment creates a test environment in state
func createTestEnvironment(t *testing.T, project, name, status string) *models.Environment {
	env, err := state.CreateEnvironment(name, "/tmp/test", project)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}
	env.Status = status
	env.Services = map[string]*models.Service{
		"api":   {Name: "api", IP: "10.224.1.10"},
		"redis": {Name: "redis", IP: "10.224.1.11"},
	}
	if err := state.UpdateEnvironment(env); err != nil {
		t.Fatalf("Failed to update environment: %v", err)
	}
	return env
}

// createTestMachine creates a test machine connection
func createTestMachine(t *testing.T, host string) *Machine {
	machine := &Machine{
		Host:              host,
		Token:             "test-token",
		WGPrivateKey:      "test-private-key",
		WGPublicKey:       "test-public-key",
		WGServerPublicKey: "test-server-public-key",
		WGAssignedIP:      "10.225.1.2",
		WGEndpoint:        "192.168.1.100:51820",
		WGAllowedIPs:      []string{"10.225.0.0/16"},
		ConnectedAt:       time.Now(),
		Status:            "connected",
		Version:           1,
	}
	if err := SaveMachine(machine); err != nil {
		t.Fatalf("Failed to save machine: %v", err)
	}
	return machine
}

func TestListLocalOnly(t *testing.T) {
	cleanup := setupTestState(t)
	defer cleanup()

	createTestEnvironment(t, "myapp", "feat-auth", "running")
	createTestEnvironment(t, "myapp", "feat-ui", "stopped")
	createTestEnvironment(t, "other", "test-env", "running")

	output, err := testListEnvironments(t, []string{"--project", "myapp"})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !strings.Contains(output, "feat-auth") {
		t.Errorf("Expected output to contain 'feat-auth', got:\n%s", output)
	}
	if !strings.Contains(output, "feat-ui") {
		t.Errorf("Expected output to contain 'feat-ui', got:\n%s", output)
	}

	if strings.Contains(output, "test-env") {
		t.Errorf("Expected output NOT to contain 'test-env' (different project), got:\n%s", output)
	}

	if !strings.Contains(output, "local") {
		t.Errorf("Expected output to contain 'local' machine, got:\n%s", output)
	}
}

func TestListWithRemote(t *testing.T) {
	cleanup := setupTestState(t)
	defer cleanup()

	createTestEnvironment(t, "myapp", "local-env", "running")
	createTestMachine(t, "big-box")

	output, err := testListEnvironments(t, []string{"--all"})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !strings.Contains(output, "local-env") {
		t.Errorf("Expected output to contain 'local-env', got:\n%s", output)
	}

	if !strings.Contains(output, "MACHINE") {
		t.Errorf("Expected output to contain MACHINE column header, got:\n%s", output)
	}

	if !strings.Contains(output, "local") {
		t.Errorf("Expected output to contain 'local' machine, got:\n%s", output)
	}
}

func TestListAllFlag(t *testing.T) {
	cleanup := setupTestState(t)
	defer cleanup()

	createTestEnvironment(t, "myapp", "feat-auth", "running")
	createTestEnvironment(t, "other", "other-env", "running")
	createTestEnvironment(t, "third", "third-env", "stopped")

	output, err := testListEnvironments(t, []string{})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	t.Logf("Output without --all:\n%s", output)

	outputAll, err := testListEnvironments(t, []string{"--all"})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !strings.Contains(outputAll, "feat-auth") {
		t.Errorf("Expected --all output to contain 'feat-auth', got:\n%s", outputAll)
	}
	if !strings.Contains(outputAll, "other-env") {
		t.Errorf("Expected --all output to contain 'other-env', got:\n%s", outputAll)
	}
	if !strings.Contains(outputAll, "third-env") {
		t.Errorf("Expected --all output to contain 'third-env', got:\n%s", outputAll)
	}

	if !strings.Contains(outputAll, "PROJECT") {
		t.Errorf("Expected --all output to contain PROJECT column, got:\n%s", outputAll)
	}
}

func TestListSameEnvNameDifferentMachines(t *testing.T) {
	cleanup := setupTestState(t)
	defer cleanup()

	createTestEnvironment(t, "myapp", "shared-env", "running")
	createTestMachine(t, "big-box")
	createTestMachine(t, "gpu-server")

	output, err := testListEnvironments(t, []string{"--all"})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !strings.Contains(output, "local") {
		t.Errorf("Expected output to contain 'local' machine, got:\n%s", output)
	}

	lines := strings.Split(output, "\n")
	var headerLine string
	for _, line := range lines {
		if strings.Contains(line, "ENVIRONMENT") {
			headerLine = line
			break
		}
	}

	if headerLine == "" {
		t.Errorf("Expected to find header line with ENVIRONMENT, got:\n%s", output)
	}

	if !strings.Contains(headerLine, "MACHINE") {
		t.Errorf("Expected header to contain MACHINE column, got: %s", headerLine)
	}
}

func TestListJSONFormat(t *testing.T) {
	cleanup := setupTestState(t)
	defer cleanup()

	createTestEnvironment(t, "myapp", "json-test", "running")

	output, err := testListEnvironments(t, []string{"--format", "json", "--all"})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Expected valid JSON output, got error: %v\nOutput: %s", err, output)
	}

	if len(result) > 0 {
		if _, ok := result[0]["machine"]; !ok {
			t.Errorf("Expected JSON to contain 'machine' field, got: %v", result[0])
		}
	}
}

func TestListQuietFormat(t *testing.T) {
	cleanup := setupTestState(t)
	defer cleanup()

	createTestEnvironment(t, "myapp", "quiet-test", "running")
	createTestEnvironment(t, "myapp", "quiet-test2", "stopped")

	output, err := testListEnvironments(t, []string{"--format", "quiet"})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 1 {
		t.Errorf("Expected at least one line in quiet output, got: %s", output)
	}

	found := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "quiet-test" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected quiet output to contain 'quiet-test', got: %s", output)
	}
}

func TestListUnreachableMachine(t *testing.T) {
	cleanup := setupTestState(t)
	defer cleanup()

	createTestEnvironment(t, "myapp", "local-env", "running")

	machine := &Machine{
		Host:              "unreachable-server",
		Token:             "test-token",
		WGPrivateKey:      "test-private-key",
		WGPublicKey:       "test-public-key",
		WGServerPublicKey: "test-server-public-key",
		WGAssignedIP:      "192.0.2.1",
		WGEndpoint:        "192.0.2.1:51820",
		WGAllowedIPs:      []string{"10.225.0.0/16"},
		ConnectedAt:       time.Now(),
		Status:            "connected",
		Version:           1,
	}
	if err := SaveMachine(machine); err != nil {
		t.Fatalf("Failed to save machine: %v", err)
	}

	output, err := testListEnvironments(t, []string{"--all"})
	if err != nil {
		t.Fatalf("Command should not fail with unreachable machine: %v", err)
	}

	if !strings.Contains(output, "local-env") {
		t.Errorf("Expected output to contain 'local-env' despite unreachable machine, got:\n%s", output)
	}
}

func TestListProjectFilter(t *testing.T) {
	cleanup := setupTestState(t)
	defer cleanup()

	createTestEnvironment(t, "project-a", "env-a1", "running")
	createTestEnvironment(t, "project-a", "env-a2", "running")
	createTestEnvironment(t, "project-b", "env-b1", "running")

	output, err := testListEnvironments(t, []string{"--project", "project-a"})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !strings.Contains(output, "env-a1") {
		t.Errorf("Expected output to contain 'env-a1', got:\n%s", output)
	}
	if !strings.Contains(output, "env-a2") {
		t.Errorf("Expected output to contain 'env-a2', got:\n%s", output)
	}
	if strings.Contains(output, "env-b1") {
		t.Errorf("Expected output NOT to contain 'env-b1', got:\n%s", output)
	}
}

// Helper to get tabwriter
func getTabWriter(out interface{}) *tabWriter {
	return &tabWriter{out: out}
}
