package cloud

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// setupTestHome creates a temporary directory and sets it as HOME
// Returns cleanup function to restore original state
func setupTestHome(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "cilo-auth-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Save original HOME
	originalHome := os.Getenv("HOME")
	if runtime.GOOS == "windows" {
		originalHome = os.Getenv("USERPROFILE")
	}

	// Set HOME to temp directory
	os.Setenv("HOME", tempDir)
	if runtime.GOOS == "windows" {
		os.Setenv("USERPROFILE", tempDir)
	}

	// Return cleanup function
	cleanup := func() {
		os.Setenv("HOME", originalHome)
		if runtime.GOOS == "windows" {
			os.Setenv("USERPROFILE", originalHome)
		}
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// getAuthFilePath returns the expected auth file path for the test
func getAuthFilePath(homeDir string) string {
	return filepath.Join(homeDir, ".cilo", "cloud-auth.json")
}

func TestSaveAuth(t *testing.T) {
	t.Run("saves auth to file with correct permissions", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		auth := &Auth{
			Server: "https://api.example.com",
			APIKey: "test-api-key-12345",
			TeamID: "team-abc",
		}

		err := SaveAuth(auth)
		if err != nil {
			t.Fatalf("SaveAuth failed: %v", err)
		}

		// Verify file exists
		authPath := getAuthFilePath(tempDir)
		if _, err := os.Stat(authPath); os.IsNotExist(err) {
			t.Fatal("Auth file was not created")
		}

		// Verify file permissions (Unix only)
		if runtime.GOOS != "windows" {
			info, err := os.Stat(authPath)
			if err != nil {
				t.Fatalf("Failed to stat auth file: %v", err)
			}
			mode := info.Mode().Perm()
			if mode != 0600 {
				t.Errorf("Expected file permissions 0600, got %04o", mode)
			}
		}

		// Verify directory permissions (Unix only)
		dirPath := filepath.Dir(authPath)
		if runtime.GOOS != "windows" {
			info, err := os.Stat(dirPath)
			if err != nil {
				t.Fatalf("Failed to stat auth directory: %v", err)
			}
			mode := info.Mode().Perm()
			if mode != 0700 {
				t.Errorf("Expected directory permissions 0700, got %04o", mode)
			}
		}

		// Verify content
		data, err := os.ReadFile(authPath)
		if err != nil {
			t.Fatalf("Failed to read auth file: %v", err)
		}

		var savedAuth Auth
		if err := json.Unmarshal(data, &savedAuth); err != nil {
			t.Fatalf("Failed to unmarshal saved auth: %v", err)
		}

		if savedAuth.Server != auth.Server {
			t.Errorf("Server mismatch: got %q, want %q", savedAuth.Server, auth.Server)
		}
		if savedAuth.APIKey != auth.APIKey {
			t.Errorf("APIKey mismatch: got %q, want %q", savedAuth.APIKey, auth.APIKey)
		}
		if savedAuth.TeamID != auth.TeamID {
			t.Errorf("TeamID mismatch: got %q, want %q", savedAuth.TeamID, auth.TeamID)
		}
	})

	t.Run("saves auth without team_id", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		auth := &Auth{
			Server: "https://api.example.com",
			APIKey: "test-api-key-12345",
		}

		err := SaveAuth(auth)
		if err != nil {
			t.Fatalf("SaveAuth failed: %v", err)
		}

		authPath := getAuthFilePath(tempDir)
		data, err := os.ReadFile(authPath)
		if err != nil {
			t.Fatalf("Failed to read auth file: %v", err)
		}

		// Verify team_id is not present in JSON (omitempty)
		var rawMap map[string]interface{}
		if err := json.Unmarshal(data, &rawMap); err != nil {
			t.Fatalf("Failed to unmarshal raw JSON: %v", err)
		}

		if _, exists := rawMap["team_id"]; exists {
			t.Error("team_id should not be present when empty")
		}
	})

	t.Run("overwrites existing auth file", func(t *testing.T) {
		_, cleanup := setupTestHome(t)
		defer cleanup()

		// Create initial auth
		auth1 := &Auth{
			Server: "https://old.example.com",
			APIKey: "old-key",
		}
		if err := SaveAuth(auth1); err != nil {
			t.Fatalf("First SaveAuth failed: %v", err)
		}

		// Overwrite with new auth
		auth2 := &Auth{
			Server: "https://new.example.com",
			APIKey: "new-key",
			TeamID: "new-team",
		}
		if err := SaveAuth(auth2); err != nil {
			t.Fatalf("Second SaveAuth failed: %v", err)
		}

		// Verify new content
		loaded, err := LoadAuth()
		if err != nil {
			t.Fatalf("LoadAuth failed: %v", err)
		}

		if loaded.Server != auth2.Server {
			t.Errorf("Server not updated: got %q, want %q", loaded.Server, auth2.Server)
		}
		if loaded.APIKey != auth2.APIKey {
			t.Errorf("APIKey not updated: got %q, want %q", loaded.APIKey, auth2.APIKey)
		}
	})
}

func TestLoadAuth(t *testing.T) {
	t.Run("loads valid auth file", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		// Create auth file manually
		auth := &Auth{
			Server: "https://api.example.com",
			APIKey: "test-api-key-12345",
			TeamID: "team-abc",
		}
		data, _ := json.Marshal(auth)

		authPath := getAuthFilePath(tempDir)
		os.MkdirAll(filepath.Dir(authPath), 0700)
		os.WriteFile(authPath, data, 0600)

		loaded, err := LoadAuth()
		if err != nil {
			t.Fatalf("LoadAuth failed: %v", err)
		}

		if loaded.Server != auth.Server {
			t.Errorf("Server mismatch: got %q, want %q", loaded.Server, auth.Server)
		}
		if loaded.APIKey != auth.APIKey {
			t.Errorf("APIKey mismatch: got %q, want %q", loaded.APIKey, auth.APIKey)
		}
		if loaded.TeamID != auth.TeamID {
			t.Errorf("TeamID mismatch: got %q, want %q", loaded.TeamID, auth.TeamID)
		}
	})

	t.Run("returns error when file does not exist", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		_, err := LoadAuth()
		if err == nil {
			t.Fatal("Expected error for missing file, got nil")
		}

		expectedMsg := "not logged in"
		if err.Error() != expectedMsg {
			t.Errorf("Error message mismatch: got %q, want %q", err.Error(), expectedMsg)
		}

		// Use tempDir to verify the file doesn't exist there
		authPath := getAuthFilePath(tempDir)
		if _, err := os.Stat(authPath); !os.IsNotExist(err) {
			t.Error("Auth file should not exist")
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		// Create invalid JSON file
		authPath := getAuthFilePath(tempDir)
		os.MkdirAll(filepath.Dir(authPath), 0700)
		os.WriteFile(authPath, []byte("not valid json"), 0600)

		_, err := LoadAuth()
		if err == nil {
			t.Fatal("Expected error for invalid JSON, got nil")
		}

		if !strings.Contains(err.Error(), "parse auth file") {
			t.Errorf("Expected 'parse auth file' error, got: %v", err)
		}
	})

	t.Run("returns error when server is missing", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		// Create auth file with missing server
		auth := map[string]string{
			"api_key": "test-key",
		}
		data, _ := json.Marshal(auth)

		authPath := getAuthFilePath(tempDir)
		os.MkdirAll(filepath.Dir(authPath), 0700)
		os.WriteFile(authPath, data, 0600)

		_, err := LoadAuth()
		if err == nil {
			t.Fatal("Expected error for missing server, got nil")
		}

		expectedMsg := "invalid auth file: missing server or api_key"
		if err.Error() != expectedMsg {
			t.Errorf("Error message mismatch: got %q, want %q", err.Error(), expectedMsg)
		}
	})

	t.Run("returns error when api_key is missing", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		// Create auth file with missing api_key
		auth := map[string]string{
			"server": "https://api.example.com",
		}
		data, _ := json.Marshal(auth)

		authPath := getAuthFilePath(tempDir)
		os.MkdirAll(filepath.Dir(authPath), 0700)
		os.WriteFile(authPath, data, 0600)

		_, err := LoadAuth()
		if err == nil {
			t.Fatal("Expected error for missing api_key, got nil")
		}

		expectedMsg := "invalid auth file: missing server or api_key"
		if err.Error() != expectedMsg {
			t.Errorf("Error message mismatch: got %q, want %q", err.Error(), expectedMsg)
		}
	})

	t.Run("returns error when both server and api_key are missing", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		// Create empty auth file
		auth := map[string]string{}
		data, _ := json.Marshal(auth)

		authPath := getAuthFilePath(tempDir)
		os.MkdirAll(filepath.Dir(authPath), 0700)
		os.WriteFile(authPath, data, 0600)

		_, err := LoadAuth()
		if err == nil {
			t.Fatal("Expected error for empty auth, got nil")
		}

		expectedMsg := "invalid auth file: missing server or api_key"
		if err.Error() != expectedMsg {
			t.Errorf("Error message mismatch: got %q, want %q", err.Error(), expectedMsg)
		}
	})

	t.Run("handles empty server string", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		auth := map[string]string{
			"server":  "",
			"api_key": "test-key",
		}
		data, _ := json.Marshal(auth)

		authPath := getAuthFilePath(tempDir)
		os.MkdirAll(filepath.Dir(authPath), 0700)
		os.WriteFile(authPath, data, 0600)

		_, err := LoadAuth()
		if err == nil {
			t.Fatal("Expected error for empty server, got nil")
		}
	})

	t.Run("handles empty api_key string", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		auth := map[string]string{
			"server":  "https://api.example.com",
			"api_key": "",
		}
		data, _ := json.Marshal(auth)

		authPath := getAuthFilePath(tempDir)
		os.MkdirAll(filepath.Dir(authPath), 0700)
		os.WriteFile(authPath, data, 0600)

		_, err := LoadAuth()
		if err == nil {
			t.Fatal("Expected error for empty api_key, got nil")
		}
	})
}

func TestClearAuth(t *testing.T) {
	t.Run("removes existing auth file", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		// Create auth file
		auth := &Auth{
			Server: "https://api.example.com",
			APIKey: "test-key",
		}
		if err := SaveAuth(auth); err != nil {
			t.Fatalf("SaveAuth failed: %v", err)
		}

		authPath := getAuthFilePath(tempDir)
		if _, err := os.Stat(authPath); os.IsNotExist(err) {
			t.Fatal("Auth file should exist before ClearAuth")
		}

		// Clear auth
		err := ClearAuth()
		if err != nil {
			t.Fatalf("ClearAuth failed: %v", err)
		}

		// Verify file is removed
		if _, err := os.Stat(authPath); !os.IsNotExist(err) {
			t.Error("Auth file should be removed after ClearAuth")
		}
	})

	t.Run("succeeds when file does not exist", func(t *testing.T) {
		_, cleanup := setupTestHome(t)
		defer cleanup()

		// Don't create any file
		err := ClearAuth()
		if err != nil {
			t.Fatalf("ClearAuth should succeed when file doesn't exist: %v", err)
		}
	})

	t.Run("returns error on permission denied", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping permission test on Windows")
		}

		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		// Create auth file
		auth := &Auth{
			Server: "https://api.example.com",
			APIKey: "test-key",
		}
		if err := SaveAuth(auth); err != nil {
			t.Fatalf("SaveAuth failed: %v", err)
		}

		authPath := getAuthFilePath(tempDir)

		// Remove read/write permissions from parent directory
		dirPath := filepath.Dir(authPath)
		os.Chmod(dirPath, 0000)
		defer os.Chmod(dirPath, 0700) // Restore for cleanup

		err := ClearAuth()
		if err == nil {
			t.Fatal("Expected error when directory is not accessible")
		}
	})
}

func TestIsLoggedIn(t *testing.T) {
	t.Run("returns true when auth file exists and is valid", func(t *testing.T) {
		_, cleanup := setupTestHome(t)
		defer cleanup()

		auth := &Auth{
			Server: "https://api.example.com",
			APIKey: "test-key",
		}
		if err := SaveAuth(auth); err != nil {
			t.Fatalf("SaveAuth failed: %v", err)
		}

		if !IsLoggedIn() {
			t.Error("IsLoggedIn should return true for valid auth")
		}
	})

	t.Run("returns false when auth file does not exist", func(t *testing.T) {
		_, cleanup := setupTestHome(t)
		defer cleanup()

		if IsLoggedIn() {
			t.Error("IsLoggedIn should return false when no auth file")
		}
	})

	t.Run("returns false when auth file is invalid", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		// Create invalid auth file
		authPath := getAuthFilePath(tempDir)
		os.MkdirAll(filepath.Dir(authPath), 0700)
		os.WriteFile(authPath, []byte("invalid json"), 0600)

		if IsLoggedIn() {
			t.Error("IsLoggedIn should return false for invalid auth")
		}
	})

	t.Run("returns false when auth file is missing required fields", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		// Create auth file with missing fields
		auth := map[string]string{
			"server": "https://api.example.com",
			// missing api_key
		}
		data, _ := json.Marshal(auth)

		authPath := getAuthFilePath(tempDir)
		os.MkdirAll(filepath.Dir(authPath), 0700)
		os.WriteFile(authPath, data, 0600)

		if IsLoggedIn() {
			t.Error("IsLoggedIn should return false for incomplete auth")
		}
	})
}

func TestGetServerURL(t *testing.T) {
	t.Run("returns server URL when logged in", func(t *testing.T) {
		_, cleanup := setupTestHome(t)
		defer cleanup()

		expectedURL := "https://api.example.com"
		auth := &Auth{
			Server: expectedURL,
			APIKey: "test-key",
		}
		if err := SaveAuth(auth); err != nil {
			t.Fatalf("SaveAuth failed: %v", err)
		}

		url, err := GetServerURL()
		if err != nil {
			t.Fatalf("GetServerURL failed: %v", err)
		}

		if url != expectedURL {
			t.Errorf("URL mismatch: got %q, want %q", url, expectedURL)
		}
	})

	t.Run("returns error when not logged in", func(t *testing.T) {
		_, cleanup := setupTestHome(t)
		defer cleanup()

		_, err := GetServerURL()
		if err == nil {
			t.Fatal("Expected error when not logged in")
		}

		if err.Error() != "not logged in" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("returns error when auth is invalid", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		// Create invalid auth file
		authPath := getAuthFilePath(tempDir)
		os.MkdirAll(filepath.Dir(authPath), 0700)
		os.WriteFile(authPath, []byte("invalid"), 0600)

		_, err := GetServerURL()
		if err == nil {
			t.Fatal("Expected error for invalid auth")
		}
	})
}

func TestAuthFilePath(t *testing.T) {
	t.Run("returns correct path", func(t *testing.T) {
		tempDir, cleanup := setupTestHome(t)
		defer cleanup()

		path, err := authFilePath()
		if err != nil {
			t.Fatalf("authFilePath failed: %v", err)
		}

		expectedPath := filepath.Join(tempDir, ".cilo", "cloud-auth.json")
		if path != expectedPath {
			t.Errorf("Path mismatch: got %q, want %q", path, expectedPath)
		}
	})

	t.Run("handles home directory error", func(t *testing.T) {
		// Save original HOME
		originalHome := os.Getenv("HOME")
		defer os.Setenv("HOME", originalHome)

		// Set HOME to empty to simulate error condition
		os.Setenv("HOME", "")

		_, err := authFilePath()
		if err == nil {
			t.Fatal("Expected error when HOME is empty")
		}
	})
}

// Benchmark tests
func BenchmarkSaveAuth(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "cilo-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("HOME", tempDir)
	defer os.Unsetenv("HOME")

	auth := &Auth{
		Server: "https://api.example.com",
		APIKey: "test-api-key",
		TeamID: "team-123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := SaveAuth(auth); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadAuth(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "cilo-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("HOME", tempDir)
	defer os.Unsetenv("HOME")

	// Create auth file first
	auth := &Auth{
		Server: "https://api.example.com",
		APIKey: "test-api-key",
		TeamID: "team-123",
	}
	if err := SaveAuth(auth); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := LoadAuth(); err != nil {
			b.Fatal(err)
		}
	}
}

// Integration test
func TestAuthIntegration(t *testing.T) {
	_, cleanup := setupTestHome(t)
	defer cleanup()

	// Full workflow: Save -> IsLoggedIn -> GetServerURL -> Load -> Clear -> IsLoggedIn
	auth := &Auth{
		Server: "https://integration.example.com",
		APIKey: "integration-key",
		TeamID: "integration-team",
	}

	// Save
	if err := SaveAuth(auth); err != nil {
		t.Fatalf("SaveAuth failed: %v", err)
	}

	// Check logged in
	if !IsLoggedIn() {
		t.Error("Should be logged in after SaveAuth")
	}

	// Get server URL
	url, err := GetServerURL()
	if err != nil {
		t.Fatalf("GetServerURL failed: %v", err)
	}
	if url != auth.Server {
		t.Errorf("URL mismatch: got %q, want %q", url, auth.Server)
	}

	// Load and verify
	loaded, err := LoadAuth()
	if err != nil {
		t.Fatalf("LoadAuth failed: %v", err)
	}
	if loaded.Server != auth.Server || loaded.APIKey != auth.APIKey || loaded.TeamID != auth.TeamID {
		t.Error("Loaded auth doesn't match saved auth")
	}

	// Clear
	if err := ClearAuth(); err != nil {
		t.Fatalf("ClearAuth failed: %v", err)
	}

	// Check not logged in
	if IsLoggedIn() {
		t.Error("Should not be logged in after ClearAuth")
	}

	// GetServerURL should fail
	_, err = GetServerURL()
	if err == nil {
		t.Error("GetServerURL should fail after ClearAuth")
	}

	// LoadAuth should fail
	_, err = LoadAuth()
	if err == nil {
		t.Error("LoadAuth should fail after ClearAuth")
	}
}

// Test concurrent access
func TestConcurrentSave(t *testing.T) {
	_, cleanup := setupTestHome(t)
	defer cleanup()

	// Test that concurrent saves don't corrupt the file
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			auth := &Auth{
				Server: fmt.Sprintf("https://server%d.example.com", idx),
				APIKey: fmt.Sprintf("key-%d", idx),
			}
			SaveAuth(auth)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify file is valid (last write wins)
	loaded, err := LoadAuth()
	if err != nil {
		t.Fatalf("LoadAuth after concurrent saves failed: %v", err)
	}

	// Just verify it has valid structure
	if loaded.Server == "" || loaded.APIKey == "" {
		t.Error("Concurrent save resulted in invalid auth")
	}
}
