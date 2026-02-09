// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestCloudAuthFlow tests the complete cloud authentication flow
// This test verifies:
// 1. Server is running and healthy
// 2. API key can be created via admin command
// 3. API key format is correct (cilo_ prefix)
// 4. Auth validation endpoint returns correct team info
// 5. Full authentication middleware works
func TestCloudAuthFlow(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true)")
	}

	serverURL := os.Getenv("CILO_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// Test 1: Server health check
	t.Run("ServerHealth", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/health")
		if err != nil {
			t.Fatalf("Server not reachable: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Server health check failed: %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "healthy") {
			t.Fatalf("Server not healthy: %s", string(body))
		}
		t.Log("✓ Server is healthy")
	})

	// Test 2: Create API key via admin command
	var apiKey string
	t.Run("CreateAPIKey", func(t *testing.T) {
		cmd := exec.Command("docker", "exec", "self-host-server-1", "cilo-server", "admin", "create-key",
			"--team", "team-default",
			"--scope", "admin",
			"--name", "e2e-test-key")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to create API key: %v\nOutput: %s", err, output)
		}

		// Extract API key from output
		outputStr := string(output)
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "cilo_") {
				apiKey = strings.TrimSpace(line)
				break
			}
		}

		if apiKey == "" {
			t.Fatalf("Could not extract API key from output:\n%s", outputStr)
		}

		// Verify key format
		if !strings.HasPrefix(apiKey, "cilo_") {
			t.Fatalf("API key has wrong format (should start with cilo_): %s", apiKey)
		}

		t.Logf("✓ API key created: %s...", apiKey[:20])
	})

	// Test 3: Auth validation endpoint
	t.Run("AuthValidation", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("Skipping - no API key from previous test")
		}

		req, err := http.NewRequest("GET", serverURL+"/v1/auth/validate", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Auth validation request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Auth validation failed: %d - %s", resp.StatusCode, string(body))
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		if !strings.Contains(bodyStr, "team_id") {
			t.Fatalf("Auth response missing team_id: %s", bodyStr)
		}

		t.Logf("✓ Auth validation successful: %s", bodyStr)
	})

	// Test 4: Protected endpoint (environments)
	t.Run("ProtectedEndpoint", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("Skipping - no API key from previous test")
		}

		req, err := http.NewRequest("GET", serverURL+"/v1/environments", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Environments request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should return 200 OK (even if empty) or 501 Not Implemented
		// but NOT 401 Unauthorized
		if resp.StatusCode == http.StatusUnauthorized {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Authentication failed on protected endpoint: %s", string(body))
		}

		t.Logf("✓ Protected endpoint accessible (status: %d)", resp.StatusCode)
	})

	// Test 5: Invalid API key rejected
	t.Run("InvalidAPIKey", func(t *testing.T) {
		req, err := http.NewRequest("GET", serverURL+"/v1/auth/validate", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer invalid_key_format")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("Expected 401 for invalid key, got: %d", resp.StatusCode)
		}

		t.Log("✓ Invalid API key correctly rejected")
	})

	// Test 6: Missing Authorization header
	t.Run("MissingAuth", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/v1/environments")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("Expected 401 for missing auth, got: %d", resp.StatusCode)
		}

		t.Log("✓ Missing auth header correctly rejected")
	})
}

// TestCloudLoginCommand tests the CLI cloud login command
func TestCloudLoginCommand(t *testing.T) {
	if os.Getenv("CILO_E2E_ENABLED") != "true" {
		t.Skip("E2E tests disabled (set CILO_E2E_ENABLED=true)")
	}

	ciloBinary := os.Getenv("CILO_BINARY")
	if ciloBinary == "" {
		ciloBinary = "cilo"
	}

	serverURL := os.Getenv("CILO_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// Get API key from server
	var apiKey string
	t.Run("GetAPIKey", func(t *testing.T) {
		cmd := exec.Command("docker", "exec", "self-host-server-1", "cilo-server", "admin", "create-key",
			"--team", "team-default",
			"--scope", "admin",
			"--name", "e2e-login-test")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to create API key: %v\nOutput: %s", err, output)
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "cilo_") {
				apiKey = strings.TrimSpace(line)
				break
			}
		}

		if apiKey == "" {
			t.Fatalf("Could not extract API key")
		}
		t.Logf("✓ Got API key for login test")
	})

	// Test cloud login
	t.Run("CloudLogin", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("No API key available")
		}

		// Create command with stdin
		cmd := exec.Command(ciloBinary, "cloud", "login", "--server", serverURL)
		cmd.Stdin = bytes.NewBufferString(apiKey + "\n")

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Cloud login failed: %v\nOutput: %s", err, output)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "Logged in") {
			t.Fatalf("Login did not succeed: %s", outputStr)
		}

		t.Logf("✓ Cloud login successful: %s", outputStr)
	})
}
