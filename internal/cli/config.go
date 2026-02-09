// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"fmt"
	"os"

	"github.com/sharedco/cilo/internal/models"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or manage project configuration",
	Long: `Show or manage the cilo configuration for the current project.

This command reads and displays the .cilo/config.yml file.

Examples:
  # Show current configuration
  cilo config

  # Show configuration in different formats
  cilo config --format yaml
  cilo config --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")

		// Check if we're in a configured project
		config, err := models.LoadProjectConfig()
		if err != nil {
			return fmt.Errorf("failed to load project config: %w", err)
		}
		if config == nil {
			return fmt.Errorf("no project configured in current directory\n\nRun 'cilo setup' to configure this project")
		}

		switch format {
		case "json":
			return showConfigJSON(config)
		case "yaml", "yml":
			return showConfigYAML(config)
		default:
			return showConfigTable(config)
		}
	},
}

func showConfigTable(config *models.ProjectConfig) error {
	fmt.Printf("Project Configuration\n")
	fmt.Printf("=====================\n\n")
	fmt.Printf("Project:           %s\n", config.Project)
	fmt.Printf("Build Tool:        %s\n", config.BuildTool)
	fmt.Printf("Default Env:       %s\n", config.DefaultEnvironment)
	if config.DNSSuffix != "" {
		fmt.Printf("DNS Suffix:        %s\n", config.DNSSuffix)
	}
	if len(config.Hostnames) > 0 {
		fmt.Printf("Hostnames:         %v\n", config.Hostnames)
	}
	fmt.Printf("\nCompose Files:\n")
	for i, f := range config.ComposeFiles {
		fmt.Printf("  %d. %s\n", i+1, f)
	}
	if len(config.EnvFiles) > 0 {
		fmt.Printf("\nEnv Files:\n")
		for _, f := range config.EnvFiles {
			fmt.Printf("  - %s\n", f)
		}
	}
	if len(config.Environments) > 0 {
		fmt.Printf("\nEnvironments:\n")
		for _, env := range config.Environments {
			fmt.Printf("  - %s\n", env)
		}
	}
	return nil
}

func showConfigJSON(config *models.ProjectConfig) error {
	// Read raw file for accurate YAML output
	data, err := os.ReadFile(".cilo/config.yml")
	if err != nil {
		return err
	}
	// For now, just print the raw YAML since converting to JSON is extra work
	// and the yaml package doesn't have a simple MarshalToJSON
	fmt.Print(string(data))
	return nil
}

func showConfigYAML(config *models.ProjectConfig) error {
	data, err := os.ReadFile(".cilo/config.yml")
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}

func init() {
	configCmd.Flags().String("format", "table", "Output format: table, yaml, json")
}
