// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Repo represents a git repository in the project
type Repo struct {
	Name string
	Path string // Relative to project root
}

// FindRepos finds all git repositories in a directory tree
func FindRepos(root string) ([]Repo, error) {
	var repos []Repo
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == ".git" {
			repoPath := filepath.Dir(path)
			rel, err := filepath.Rel(root, repoPath)
			if err != nil {
				return err
			}
			name := rel
			if rel == "." {
				name = "root"
			}
			repos = append(repos, Repo{
				Name: name,
				Path: rel,
			})
			return filepath.SkipDir
		}
		return nil
	})
	return repos, err
}

// Diff compares the host repo with the environment repo
func Diff(hostRoot, envRoot string, repo Repo) (string, error) {
	hostPath := filepath.Join(hostRoot, repo.Path)
	envPath := filepath.Join(envRoot, repo.Path)

	// Fetch from env to host as a temp branch
	branchName := "cilo-diff-tmp"
	// Ensure we are working with absolute paths for git fetch
	absEnvPath, _ := filepath.Abs(envPath)

	fetchCmd := exec.Command("git", "fetch", absEnvPath, "HEAD:"+branchName)
	fetchCmd.Dir = hostPath
	if err := fetchCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to fetch from env: %w", err)
	}
	defer func() {
		exec.Command("git", "branch", "-D", branchName).Run()
	}()

	// Generate diff
	diffCmd := exec.Command("git", "diff", "HEAD.."+branchName)
	diffCmd.Dir = hostPath
	output, err := diffCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate diff: %w", err)
	}

	return string(output), nil
}

// Merge brings changes from env to host
func Merge(hostRoot, envRoot string, repo Repo) error {
	hostPath := filepath.Join(hostRoot, repo.Path)
	envPath := filepath.Join(envRoot, repo.Path)
	absEnvPath, _ := filepath.Abs(envPath)

	// Pull from env
	cmd := exec.Command("git", "pull", absEnvPath, "HEAD")
	cmd.Dir = hostPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to merge from env: %w", err)
	}

	return nil
}
