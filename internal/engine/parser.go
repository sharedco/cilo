// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Parser converts a project directory into an EnvironmentSpec.
// Implementations detect and parse different project formats (Compose, Devcontainer, Procfile).
type Parser interface {
	// Name returns the parser identifier (e.g., "compose", "devcontainer", "procfile")
	Name() string

	// Detect checks if this parser can handle the given project directory.
	// Returns true if the parser recognizes the project structure.
	Detect(projectPath string) bool

	// Parse converts the project directory into a universal EnvironmentSpec.
	// Returns an error if parsing fails.
	Parse(projectPath string) (*EnvironmentSpec, error)
}

// ParserRegistry holds all registered parsers in priority order.
// Parsers are tried in registration order during auto-detection.
type ParserRegistry struct {
	mu      sync.RWMutex
	parsers []Parser
}

// NewParserRegistry creates a new parser registry.
func NewParserRegistry() *ParserRegistry {
	return &ParserRegistry{
		parsers: make([]Parser, 0),
	}
}

// Register adds a parser to the registry.
// Parsers are tried in registration order, so register more specific parsers first.
func (r *ParserRegistry) Register(parser Parser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.parsers = append(r.parsers, parser)
}

// DetectAndParse automatically detects the project type and parses it.
// Returns the first parser that successfully detects and parses the project.
func (r *ParserRegistry) DetectAndParse(projectPath string) (*EnvironmentSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Validate project path exists
	if _, err := os.Stat(projectPath); err != nil {
		return nil, fmt.Errorf("project path does not exist: %w", err)
	}

	// Try each parser in order
	for _, parser := range r.parsers {
		if parser.Detect(projectPath) {
			spec, err := parser.Parse(projectPath)
			if err != nil {
				return nil, fmt.Errorf("parser %s detected project but failed to parse: %w", parser.Name(), err)
			}
			return spec, nil
		}
	}

	return nil, fmt.Errorf("no parser detected a valid project at %s", projectPath)
}

// Detect checks if any registered parser can handle the project.
// Returns the name of the first matching parser, or empty string if none match.
func (r *ParserRegistry) Detect(projectPath string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, parser := range r.parsers {
		if parser.Detect(projectPath) {
			return parser.Name()
		}
	}

	return ""
}

// Get returns a parser by name, or nil if not found.
func (r *ParserRegistry) Get(name string) Parser {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, parser := range r.parsers {
		if parser.Name() == name {
			return parser
		}
	}

	return nil
}

// List returns the names of all registered parsers.
func (r *ParserRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.parsers))
	for i, parser := range r.parsers {
		names[i] = parser.Name()
	}
	return names
}

// DefaultRegistry is the global parser registry.
var DefaultRegistry = NewParserRegistry()

// Register adds a parser to the default registry.
func Register(parser Parser) {
	DefaultRegistry.Register(parser)
}

// DetectAndParse uses the default registry to detect and parse a project.
func DetectAndParse(projectPath string) (*EnvironmentSpec, error) {
	return DefaultRegistry.DetectAndParse(projectPath)
}

// Detect uses the default registry to detect a project type.
func Detect(projectPath string) string {
	return DefaultRegistry.Detect(projectPath)
}

// Get retrieves a parser by name from the default registry.
func Get(name string) Parser {
	return DefaultRegistry.Get(name)
}

// List returns all parser names from the default registry.
func List() []string {
	return DefaultRegistry.List()
}

// FileExists checks if a file exists at the given path.
// This is a helper function for parser implementations.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// FindFile searches for a file in the project directory and its parents.
// Returns the absolute path to the file if found, or empty string if not found.
// This is useful for finding configuration files like docker-compose.yml.
func FindFile(projectPath string, filename string) string {
	current := projectPath
	for {
		candidate := filepath.Join(current, filename)
		if FileExists(candidate) {
			return candidate
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			break
		}
		current = parent
	}
	return ""
}

// FindFiles searches for multiple possible filenames in the project directory.
// Returns the absolute path to the first file found, or empty string if none found.
func FindFiles(projectPath string, filenames []string) string {
	for _, filename := range filenames {
		if path := FindFile(projectPath, filename); path != "" {
			return path
		}
	}
	return ""
}
