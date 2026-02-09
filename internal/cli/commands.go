// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/sharedco/cilo/internal/config"
	"github.com/sharedco/cilo/internal/models"
	"github.com/sharedco/cilo/internal/runtime"
	"github.com/sharedco/cilo/internal/runtime/docker"
	"github.com/sharedco/cilo/internal/state"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments",
	Long: `List environments for the current project or all environments.

By default, shows environments for the current project (if in a configured project directory).
Use --all to see all environments across all projects.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		allFlag, _ := cmd.Flags().GetBool("all")
		projectFilter, _ := cmd.Flags().GetString("project")

		envs, err := state.ListEnvironments()
		if err != nil {
			return err
		}

		if !allFlag {
			var currentProject string
			if projectFilter != "" {
				currentProject = projectFilter
			} else {
				config, _ := models.LoadProjectConfig()
				if config != nil {
					currentProject = config.Project
				} else {
					cwd, err := os.Getwd()
					if err == nil {
						currentProject = filepath.Base(cwd)
					}
				}
			}

			if currentProject != "" {
				projectEnvs := make([]*models.Environment, 0)
				for _, env := range envs {
					if env.Project == currentProject {
						projectEnvs = append(projectEnvs, env)
					}
				}
				envs = projectEnvs
				fmt.Printf("Environments for project: %s\n\n", currentProject)
			}
		}

		if len(envs) == 0 {
			if allFlag {
				fmt.Println("No environments found")
			} else {
				fmt.Println("No environments found for this project")
				fmt.Println("Use --all to see all environments")
			}
			return nil
		}

		switch format {
		case "json":
			return listJSON(envs, allFlag)
		case "quiet":
			return listQuiet(envs, allFlag)
		default:
			return listTable(envs, allFlag)
		}
	},
}

var statusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show environment status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, name, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}

		env, err := state.GetEnvironment(project, name)
		if err != nil {
			return err
		}

		fmt.Printf("Environment: %s\n", env.Name)
		fmt.Printf("Status: %s\n", env.Status)
		fmt.Printf("Created: %s\n", env.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Subnet: %s\n", env.Subnet)
		if env.Source != "" {
			fmt.Printf("Source: %s\n", env.Source)
		}

		dnsSuffix := ".test"
		workspace := config.GetEnvPath(project, name)
		projectConfig, _ := models.LoadProjectConfigFromPath(workspace)
		if projectConfig != nil && projectConfig.DNSSuffix != "" {
			dnsSuffix = projectConfig.DNSSuffix
		}

		fmt.Printf("\nServices:\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "NAME\tTYPE\tIP\tURL\t\n")
		fmt.Fprintf(w, "----\t----\t--\t---\t\n")

		for _, service := range env.Services {
			serviceType := "isolated"
			if contains(env.UsesSharedServices, service.Name) {
				serviceType = "shared"
			}
			url := fmt.Sprintf("http://%s.%s%s", service.Name, env.Name, dnsSuffix)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n", service.Name, serviceType, service.IP, url)
		}
		w.Flush()
		fmt.Printf("\nWorkspace: %s\n", workspace)

		return nil
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs <name> [service]",
	Short: "Show logs for an environment or service",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, name, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}
		var service string
		if len(args) > 1 {
			service = args[1]
		}

		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetInt("tail")

		if _, err := state.GetEnvironment(project, name); err != nil {
			return err
		}

		provider := docker.NewProvider()
		ctx := context.Background()
		return provider.Logs(ctx, project, name, service, runtime.LogOptions{
			Follow: follow,
			Tail:   tail,
		})
	},
}

var execCmd = &cobra.Command{
	Use:   "exec <name> <service> [command]",
	Short: "Execute a command in a service container",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, name, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}
		service := args[1]
		var command []string
		if len(args) > 2 {
			command = args[2:]
		} else {
			command = []string{"sh"}
		}

		interactive, _ := cmd.Flags().GetBool("interactive")
		tty, _ := cmd.Flags().GetBool("tty")

		if _, err := state.GetEnvironment(project, name); err != nil {
			return err
		}

		provider := docker.NewProvider()
		ctx := context.Background()
		return provider.Exec(ctx, project, name, service, command, runtime.ExecOptions{
			Interactive: interactive,
			TTY:         tty,
		})
	},
}

var pathCmd = &cobra.Command{
	Use:   "path <name>",
	Short: "Print the workspace path for an environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, name, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}

		if _, err := state.GetEnvironment(project, name); err != nil {
			return err
		}

		workspace := state.GetEnvStoragePath(project, name)
		fmt.Println(workspace)
		return nil
	},
}

var composeCmd = &cobra.Command{
	Use:   "compose <name> [args...]",
	Short: "Run docker compose commands for an environment",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, name, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}
		composeArgs := args[1:]
		if len(composeArgs) == 0 {
			composeArgs = []string{"ps"}
		}

		if _, err := state.GetEnvironment(project, name); err != nil {
			return err
		}

		provider := docker.NewProvider()
		ctx := context.Background()
		return provider.Compose(ctx, project, name, runtime.ComposeOptions{
			Args: composeArgs,
		})
	},
}

func init() {
	listCmd.Flags().String("format", "table", "Output format: table, json, quiet")
	listCmd.Flags().Bool("all", false, "Show all environments across all projects")
	listCmd.Flags().String("project", "", "Filter to specific project name")

	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logsCmd.Flags().Int("tail", 100, "Number of lines to show from end of logs")

	execCmd.Flags().BoolP("interactive", "i", true, "Keep STDIN open")
	execCmd.Flags().BoolP("tty", "t", true, "Allocate a pseudo-TTY")

	statusCmd.Flags().String("project", "", "Project name (defaults to configured project)")
	logsCmd.Flags().String("project", "", "Project name (defaults to configured project)")
	execCmd.Flags().String("project", "", "Project name (defaults to configured project)")
	pathCmd.Flags().String("project", "", "Project name (defaults to configured project)")
	composeCmd.Flags().String("project", "", "Project name (defaults to configured project)")
}

func listTable(envs []*models.Environment, all bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if all {
		fmt.Fprintf(w, "PROJECT\tNAME\tSTATUS\tSERVICES\tCREATED\t\n")
		fmt.Fprintf(w, "-------\t----\t------\t--------\t-------\t\n")
	} else {
		fmt.Fprintf(w, "NAME\tSTATUS\tSERVICES\tCREATED\t\n")
		fmt.Fprintf(w, "----\t------\t--------\t-------\t\n")
	}

	for _, env := range envs {
		services := make([]string, 0, len(env.Services))
		for name := range env.Services {
			services = append(services, name)
		}
		serviceList := strings.Join(services, ", ")
		if len(serviceList) > 30 {
			serviceList = serviceList[:27] + "..."
		}

		created := env.CreatedAt.Format("Jan 02 15:04")
		if all {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t\n", env.Project, env.Name, env.Status, serviceList, created)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n", env.Name, env.Status, serviceList, created)
		}
	}

	return w.Flush()
}

func listJSON(envs []*models.Environment, all bool) error {
	var output []map[string]interface{}

	for _, env := range envs {
		if !all && env.Status == "stopped" {
			continue
		}

		services := make([]map[string]string, 0, len(env.Services))
		for _, svc := range env.Services {
			services = append(services, map[string]string{
				"name": svc.Name,
				"url":  svc.URL,
				"ip":   svc.IP,
			})
		}

		output = append(output, map[string]interface{}{
			"name":       env.Name,
			"status":     env.Status,
			"created_at": env.CreatedAt,
			"subnet":     env.Subnet,
			"services":   services,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func listQuiet(envs []*models.Environment, all bool) error {
	for _, env := range envs {
		if !all && env.Status == "stopped" {
			continue
		}
		fmt.Println(env.Name)
	}
	return nil
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
