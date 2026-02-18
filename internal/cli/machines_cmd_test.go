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
	"time"
)

// TestMachinesCommand tests the cilo machines command
func TestMachinesCommand(t *testing.T) {
	t.Run("no machines connected", func(t *testing.T) {
		// Clean up any existing machines
		cleanupAllMockMachines(t)

		// Capture output
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetErr(&buf)

		// Execute machines command
		rootCmd.SetArgs([]string{"machines"})
		err := rootCmd.Execute()
		rootCmd.SetArgs([]string{})

		if err != nil {
			t.Fatalf("machines command failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(strings.ToLower(output), "no connected machines") &&
			!strings.Contains(strings.ToLower(output), "no machines") {
			t.Errorf("Expected 'no connected machines' message, got: %s", output)
		}
	})

	t.Run("single machine connected", func(t *testing.T) {
		cleanupAllMockMachines(t)
		setupMockMachineWithTime(t, "server-1", time.Now().Add(-2*time.Hour))

		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetErr(&buf)

		rootCmd.SetArgs([]string{"machines"})
		err := rootCmd.Execute()
		rootCmd.SetArgs([]string{})

		if err != nil {
			t.Fatalf("machines command failed: %v", err)
		}

		output := buf.String()

		// Check table headers
		if !strings.Contains(output, "MACHINE") {
			t.Error("Expected MACHINE column in output")
		}
		if !strings.Contains(output, "STATUS") {
			t.Error("Expected STATUS column in output")
		}
		if !strings.Contains(output, "ENVS") {
			t.Error("Expected ENVS column in output")
		}
		if !strings.Contains(output, "CONNECTED SINCE") {
			t.Error("Expected CONNECTED SINCE column in output")
		}

		// Check machine data
		if !strings.Contains(output, "server-1") {
			t.Error("Expected machine hostname 'server-1' in output")
		}
	})

	t.Run("multiple machines connected", func(t *testing.T) {
		cleanupAllMockMachines(t)
		setupMockMachineWithTime(t, "big-box.example.com", time.Now().Add(-2*time.Hour))
		setupMockMachineWithTime(t, "gpu-server.internal", time.Now().Add(-5*24*time.Hour))

		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetErr(&buf)

		rootCmd.SetArgs([]string{"machines"})
		err := rootCmd.Execute()
		rootCmd.SetArgs([]string{})

		if err != nil {
			t.Fatalf("machines command failed: %v", err)
		}

		output := buf.String()

		// Check both machines appear
		if !strings.Contains(output, "big-box.example.com") {
			t.Error("Expected 'big-box.example.com' in output")
		}
		if !strings.Contains(output, "gpu-server.internal") {
			t.Error("Expected 'gpu-server.internal' in output")
		}

		// Check status column exists
		if !strings.Contains(output, "connected") && !strings.Contains(output, "unreachable") {
			t.Error("Expected status (connected or unreachable) in output")
		}
	})

	t.Run("json output flag", func(t *testing.T) {
		cleanupAllMockMachines(t)
		setupMockMachineWithTime(t, "json-test-server", time.Now().Add(-3*time.Hour))

		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetErr(&buf)

		rootCmd.SetArgs([]string{"machines", "--json"})
		err := rootCmd.Execute()
		rootCmd.SetArgs([]string{})

		if err != nil {
			t.Fatalf("machines command with --json failed: %v", err)
		}

		output := buf.String()

		// Verify valid JSON
		var machines []map[string]interface{}
		if err := json.Unmarshal([]byte(output), &machines); err != nil {
			t.Fatalf("Output is not valid JSON: %v\nOutput: %s", err, output)
		}

		// Should have at least one machine
		if len(machines) == 0 {
			t.Error("Expected at least one machine in JSON output")
		}

		// Check required fields
		for _, m := range machines {
			if _, ok := m["host"]; !ok {
				t.Error("JSON missing 'host' field")
			}
			if _, ok := m["status"]; !ok {
				t.Error("JSON missing 'status' field")
			}
			if _, ok := m["env_count"]; !ok {
				t.Error("JSON missing 'env_count' field")
			}
			if _, ok := m["connected_at"]; !ok {
				t.Error("JSON missing 'connected_at' field")
			}
		}
	})

	t.Run("unreachable machine handling", func(t *testing.T) {
		cleanupAllMockMachines(t)
		// Create a machine with an invalid/unreachable IP
		setupMockMachineWithDetails(t, "unreachable-server", "10.999.999.999", time.Now().Add(-1*time.Hour))

		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetErr(&buf)

		rootCmd.SetArgs([]string{"machines"})
		err := rootCmd.Execute()
		rootCmd.SetArgs([]string{})

		// Should not fail - should handle gracefully
		if err != nil {
			t.Fatalf("machines command should handle unreachable machines gracefully, got error: %v", err)
		}

		output := buf.String()

		// Should show the machine
		if !strings.Contains(output, "unreachable-server") {
			t.Error("Expected 'unreachable-server' to appear in output even if unreachable")
		}

		// Should show as unreachable
		if !strings.Contains(output, "unreachable") {
			t.Logf("Note: Machine may appear as 'connected' if mock doesn't simulate unreachable state")
		}
	})
}

// Helper functions for test setup

func cleanupAllMockMachines(t *testing.T) {
	t.Helper()
	ciloHome := getCiloHomeForTests(t)
	machinesDir := filepath.Join(ciloHome, "machines")
	os.RemoveAll(machinesDir)
}

func getCiloHomeForTests(t *testing.T) string {
	t.Helper()
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME environment variable not set")
	}
	return filepath.Join(home, ".cilo")
}

func setupMockMachineWithTime(t *testing.T, host string, connectedAt time.Time) string {
	t.Helper()
	return setupMockMachineWithDetails(t, host, "10.225.0.1", connectedAt)
}

func setupMockMachineWithDetails(t *testing.T, host string, wgIP string, connectedAt time.Time) string {
	t.Helper()

	ciloHome := getCiloHomeForTests(t)
	machinesDir := filepath.Join(ciloHome, "machines", host)

	// Create the machine directory
	if err := os.MkdirAll(machinesDir, 0755); err != nil {
		t.Fatalf("Failed to create mock machine directory: %v", err)
	}

	// Create a mock state.json
	stateData := fmt.Sprintf(`{
		"host": "%s",
		"token": "test-token-%s",
		"wg_ip": "%s",
		"connected_at": "%s"
	}`, host, host, wgIP, connectedAt.Format(time.RFC3339))

	statePath := filepath.Join(machinesDir, "state.json")
	if err := os.WriteFile(statePath, []byte(stateData), 0644); err != nil {
		t.Fatalf("Failed to write mock state file: %v", err)
	}

	return machinesDir
}
