package engine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sharedco/cilo/pkg/engine"
	_ "github.com/sharedco/cilo/pkg/parsers" // Register parsers
)

func TestDetectAndParse_ComposeProject(t *testing.T) {
	// Create temp directory with compose file
	dir := t.TempDir()

	composeContent := `
services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
  db:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: secret
`

	err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(composeContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test detection and parsing
	spec, err := engine.DetectAndParse(dir)
	if err != nil {
		t.Fatalf("DetectAndParse failed: %v", err)
	}

	if spec.Source != "compose" {
		t.Errorf("expected source 'compose', got '%s'", spec.Source)
	}

	if len(spec.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(spec.Services))
	}

	// Verify services were parsed correctly
	serviceNames := make(map[string]bool)
	for _, svc := range spec.Services {
		serviceNames[svc.Name] = true
	}

	if !serviceNames["web"] {
		t.Error("missing 'web' service")
	}
	if !serviceNames["db"] {
		t.Error("missing 'db' service")
	}
}

func TestDetectAndParse_NoProjectFound(t *testing.T) {
	dir := t.TempDir()

	_, err := engine.DetectAndParse(dir)
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestDetectAndParse_ComposeYml(t *testing.T) {
	// Test compose.yml (newer naming)
	dir := t.TempDir()

	composeContent := `
services:
  api:
    build: .
    ports:
      - "3000:3000"
`

	err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(composeContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	spec, err := engine.DetectAndParse(dir)
	if err != nil {
		t.Fatalf("DetectAndParse failed: %v", err)
	}

	if len(spec.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(spec.Services))
	}
}

func TestDetect(t *testing.T) {
	// Test detection without parsing
	dir := t.TempDir()

	composeContent := `
services:
  test:
    image: alpine
`

	err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(composeContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	parserName := engine.Detect(dir)
	if parserName != "compose" {
		t.Errorf("expected parser 'compose', got '%s'", parserName)
	}
}

func TestDetect_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	parserName := engine.Detect(dir)
	if parserName != "" {
		t.Errorf("expected empty string for empty directory, got '%s'", parserName)
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(dir, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test file that exists
	if !engine.FileExists(testFile) {
		t.Error("FileExists should return true for existing file")
	}

	// Test file that doesn't exist
	if engine.FileExists(filepath.Join(dir, "nonexistent.txt")) {
		t.Error("FileExists should return false for nonexistent file")
	}
}

func TestFindFile(t *testing.T) {
	// Create nested directory structure
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	err := os.Mkdir(subdir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create file in parent directory
	testFile := filepath.Join(dir, "config.yml")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Search from subdirectory - should find file in parent
	found := engine.FindFile(subdir, "config.yml")
	if found == "" {
		t.Error("FindFile should find file in parent directory")
	}

	// Verify it found the correct file
	if found != testFile {
		t.Errorf("FindFile returned wrong path: got %s, want %s", found, testFile)
	}

	// Search for non-existent file
	found = engine.FindFile(subdir, "nonexistent.yml")
	if found != "" {
		t.Errorf("FindFile should return empty string for nonexistent file, got %s", found)
	}
}

func TestFindFiles(t *testing.T) {
	dir := t.TempDir()

	// Create one of multiple possible files
	testFile := filepath.Join(dir, "compose.yml")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Search for multiple possible filenames
	filenames := []string{"docker-compose.yml", "compose.yml", "compose.yaml"}
	found := engine.FindFiles(dir, filenames)
	if found == "" {
		t.Error("FindFiles should find one of the possible files")
	}

	// Verify it found the correct file
	if found != testFile {
		t.Errorf("FindFiles returned wrong path: got %s, want %s", found, testFile)
	}

	// Search for files that don't exist
	notFound := engine.FindFiles(dir, []string{"nonexistent1.yml", "nonexistent2.yml"})
	if notFound != "" {
		t.Errorf("FindFiles should return empty string when no files found, got %s", notFound)
	}
}
