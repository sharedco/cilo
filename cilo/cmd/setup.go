package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sharedco/cilo/pkg/models"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Initialize cilo for this project",
	Long: `Setup creates a .cilo/config.yml in the current directory.

This configures the project for cilo with settings like:
- Project name for DNS organization
- Docker compose file location(s)
- Build tool (docker/podman)
- Default environment name
- Hostname mappings

Examples:
  # Interactive setup
  cilo setup

  # Quick setup with defaults (detects project name from directory)
  cilo setup --name myproject

  # Setup with specific compose file
  cilo setup --compose ./docker/docker-compose.yml

  # Setup with multiple compose files
  cilo setup --compose docker-compose.yml --compose docker-compose.prod.yml
  
  # Setup with custom DNS suffix
  cilo setup --dns-suffix .localhost`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if already configured
		configPath := ".cilo/config.yml"
		if _, err := os.Stat(configPath); err == nil {
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				// Load existing config to show what project is configured
				existingData, err := os.ReadFile(configPath)
				if err == nil {
					var existing models.ProjectConfig
					if yaml.Unmarshal(existingData, &existing) == nil && existing.Project != "" {
						fmt.Printf("Project already configured: %s\n", existing.Project)
						fmt.Printf("Config: %s\n\n", configPath)
						fmt.Printf("To reconfigure this project (will overwrite existing config):\n")
						fmt.Printf("  cilo setup --force\n\n")
						fmt.Printf("To view current configuration:\n")
						fmt.Printf("  cat .cilo/config.yml\n")
						return nil
					}
				}
				return fmt.Errorf("project already configured (use --force to overwrite)")
			}
			fmt.Printf("⚠ Overwriting existing configuration at %s\n\n", configPath)
		}

		// Get flags
		name, _ := cmd.Flags().GetString("name")
		composeFiles, _ := cmd.Flags().GetStringArray("compose")
		envFiles, _ := cmd.Flags().GetStringArray("env-file")
		buildTool, _ := cmd.Flags().GetString("build-tool")
		defaultEnv, _ := cmd.Flags().GetString("default-env")
		dnsSuffix, _ := cmd.Flags().GetString("dns-suffix")

		// Auto-detect project name from directory if not provided
		if name == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
			name = filepath.Base(cwd)
		}

		// Auto-detect compose files if not provided
		if len(composeFiles) == 0 {
			possibleFiles := []string{
				"docker-compose.yml",
				"docker-compose.yaml",
				"compose.yml",
				"compose.yaml",
			}
			for _, f := range possibleFiles {
				if _, err := os.Stat(f); err == nil {
					composeFiles = append(composeFiles, f)
					break
				}
			}
			if len(composeFiles) == 0 {
				return fmt.Errorf("no docker-compose.yml found in current directory (use --compose to specify path)")
			}
		}

		// Validate compose files exist
		for _, f := range composeFiles {
			if _, err := os.Stat(f); err != nil {
				return fmt.Errorf("compose file not found: %s", f)
			}
		}

		// Create config
		config := models.ProjectConfig{
			Project:            name,
			BuildTool:          buildTool,
			ComposeFiles:       composeFiles,
			EnvFiles:           envFiles,
			DNSSuffix:          dnsSuffix,
			DefaultEnvironment: defaultEnv,
			Hostnames:          []string{},
			CopyDotDirs:        []string{".cilo"},
		}

		// Create .cilo directory
		if err := os.MkdirAll(".cilo", 0755); err != nil {
			return fmt.Errorf("failed to create .cilo directory: %w", err)
		}

		// Write config file
		configPath = filepath.Join(".cilo", "config.yml")
		data, err := yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Printf("✓ Project configured: %s\n", name)
		fmt.Printf("  Config: %s\n", configPath)
		fmt.Printf("  Compose files:\n")
		for _, f := range composeFiles {
			fmt.Printf("    - %s\n", f)
		}
		if len(envFiles) > 0 {
			fmt.Printf("  Env files:\n")
			for _, f := range envFiles {
				fmt.Printf("    - %s\n", f)
			}
		}
		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  cilo create <env>    # Create an environment\n")
		fmt.Printf("  cilo up <env>        # Start the environment\n")

		return nil
	},
}

func init() {
	setupCmd.Flags().String("name", "", "Project name (defaults to directory name)")
	setupCmd.Flags().StringArray("compose", []string{}, "Docker compose file path(s)")
	setupCmd.Flags().StringArray("env-file", []string{}, "Env file path(s) passed to docker compose")
	setupCmd.Flags().String("build-tool", "docker", "Build tool: docker, podman, nerdctl")
	setupCmd.Flags().String("default-env", "dev", "Default environment name")
	setupCmd.Flags().String("dns-suffix", "", "Override default DNS suffix (default: .test)")
	setupCmd.Flags().Bool("force", false, "Overwrite existing configuration")
}
