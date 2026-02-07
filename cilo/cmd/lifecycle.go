package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sharedco/cilo/pkg/compose"
	"github.com/sharedco/cilo/pkg/dns"
	envpkg "github.com/sharedco/cilo/pkg/env"
	"github.com/sharedco/cilo/pkg/filesystem"
	"github.com/sharedco/cilo/pkg/models"
	"github.com/sharedco/cilo/pkg/runtime"
	"github.com/sharedco/cilo/pkg/runtime/docker"
	"github.com/sharedco/cilo/pkg/share"
	"github.com/sharedco/cilo/pkg/state"
	"github.com/spf13/cobra"
)

// getProjectAndEnv determines the project and environment from context
// Priority: 1) Explicit project flag, 2) Configured project, 3) Current directory name
func getProjectAndEnv(cmd *cobra.Command, args []string) (project, envName string, err error) {
	envName = state.NormalizeName(args[0])

	projectFlag, _ := cmd.Flags().GetString("project")
	if projectFlag != "" {
		return projectFlag, envName, nil
	}

	config, err := models.LoadProjectConfig()
	if err != nil {
		return "", "", fmt.Errorf("failed to load project config: %w", err)
	}
	if config != nil {
		return config.Project, envName, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to get current directory: %w", err)
	}
	return filepath.Base(cwd), envName, nil
}

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		originalName := args[0]
		name := state.NormalizeName(originalName)
		if name != originalName {
			fmt.Printf("Normalized: %s → %s\n", originalName, name)
		}
		from, _ := cmd.Flags().GetString("from")
		empty, _ := cmd.Flags().GetBool("empty")
		include, _ := cmd.Flags().GetString("include")
		projectFlag, _ := cmd.Flags().GetString("project")

		var source string
		var project string

		if from == "" {
			from = "."
		}

		absFrom, err := filepath.Abs(from)
		if err != nil {
			return fmt.Errorf("invalid source path: %w", err)
		}
		source = absFrom

		sourceConfig, err := models.LoadProjectConfigFromPath(source)
		if err != nil {
			return fmt.Errorf("failed to load project config: %w", err)
		}

		if projectFlag != "" {
			project = projectFlag
		} else if sourceConfig != nil && sourceConfig.Project != "" {
			project = sourceConfig.Project
		} else {
			project = filepath.Base(absFrom)
		}

		exists, err := state.EnvironmentExists(project, name)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("environment %q already exists in project %q (use a different name or destroy first)", name, project)
		}

		env, err := state.CreateEnvironment(name, source, project)
		if err != nil {
			return err
		}

		workspace := state.GetEnvStoragePath(project, name)
		if err := os.MkdirAll(workspace, 0755); err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}

		if empty {
			composePath := filepath.Join(workspace, "docker-compose.yml")
			if err := compose.CreateMinimal(env, composePath); err != nil {
				return err
			}

			envPath := filepath.Join(workspace, ".env")
			os.WriteFile(envPath, []byte("# Environment variables\n"), 0644)
		} else {
			copyOpts := CopyOptions{
				Include: include,
			}
			if sourceConfig != nil {
				copyOpts.CopyDotDirs = sourceConfig.CopyDotDirs
				copyOpts.IgnoreDotDirs = sourceConfig.IgnoreDotDirs
			}

			if err := copyProject(source, workspace, copyOpts); err != nil {
				return fmt.Errorf("failed to copy project: %w", err)
			}

			composeFiles, _, err := compose.ResolveComposeFiles(workspace, nil)
			if err == nil && sourceConfig != nil {
				composeFiles, _, err = compose.ResolveComposeFiles(workspace, sourceConfig.ComposeFiles)
			}
			if err != nil {
				return err
			}
			if len(composeFiles) == 0 {
				return fmt.Errorf("no compose files found in source directory")
			}
		}

		ciloDir := filepath.Join(workspace, ".cilo")
		if err := os.MkdirAll(ciloDir, 0755); err != nil {
			return fmt.Errorf("failed to create .cilo directory: %w", err)
		}

		metaPath := filepath.Join(ciloDir, "meta.json")
		metaData := fmt.Sprintf(`{
  "name": %q,
  "project": %q,
  "created_at": %q,
  "source": %q,
  "subnet": %q
}
`, env.Name, env.Project, env.CreatedAt.Format(time.RFC3339), env.Source, env.Subnet)
		os.WriteFile(metaPath, []byte(metaData), 0644)

		fmt.Printf("✓ Environment %q created in project %q\n", name, project)
		fmt.Printf("  Workspace: %s\n", workspace)

		return nil
	},
}

var upCmd = &cobra.Command{
	Use:   "up <name>",
	Short: "Start an environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, name, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}

		build, _ := cmd.Flags().GetBool("build")
		recreate, _ := cmd.Flags().GetBool("recreate")
		sharedFlag, _ := cmd.Flags().GetStringSlice("shared")
		isolateFlag, _ := cmd.Flags().GetStringSlice("isolate")

		env, err := state.GetEnvironment(project, name)
		if err != nil {
			return err
		}

		workspace := state.GetEnvStoragePath(project, name)
		projectConfig, err := models.LoadProjectConfigFromPath(workspace)
		if err != nil {
			return fmt.Errorf("failed to load project config: %w", err)
		}

		dnsSuffix := ".test"
		if projectConfig != nil && projectConfig.DNSSuffix != "" {
			dnsSuffix = projectConfig.DNSSuffix
		}
		env.DNSSuffix = dnsSuffix

		if err := envpkg.ApplyConfig(workspace, projectConfig, envpkg.RenderContext{
			Project:   project,
			Env:       name,
			DNSSuffix: dnsSuffix,
		}); err != nil {
			return fmt.Errorf("failed to apply env config: %w", err)
		}

		composeFiles, _, err := compose.ResolveComposeFiles(workspace, nil)
		if err == nil && projectConfig != nil {
			composeFiles, _, err = compose.ResolveComposeFiles(workspace, projectConfig.ComposeFiles)
		}
		if err != nil {
			return err
		}

		if err := compose.Validate(composeFiles); err != nil {
			return fmt.Errorf("invalid compose file: %w", err)
		}

		// Determine which services should be shared
		// 1. Start with services labeled cilo.share: "true"
		sharedServices, err := compose.GetServicesWithLabel(composeFiles, "cilo.share", "true")
		if err != nil {
			return fmt.Errorf("failed to get shared services: %w", err)
		}

		// 2. Add from --shared flag
		for _, svc := range sharedFlag {
			svc = strings.TrimSpace(svc)
			if svc != "" && !contains(sharedServices, svc) {
				sharedServices = append(sharedServices, svc)
			}
		}

		// 3. Remove from --isolate flag
		sharedServices = filterOut(sharedServices, isolateFlag)

		// Create network first
		ctx := context.Background()
		provider := docker.NewProvider()
		if err := provider.CreateNetwork(ctx, env); err != nil {
			return fmt.Errorf("failed to create network: %w", err)
		}

		// Handle shared services
		if len(sharedServices) > 0 {
			fmt.Printf("Managing shared services: %s\n", strings.Join(sharedServices, ", "))
			shareMgr := share.NewManager(provider, ctx)

			for _, svc := range sharedServices {
				// Ensure shared service is running
				containerName, ip, err := shareMgr.EnsureSharedService(svc, project, composeFiles)
				if err != nil {
					return fmt.Errorf("failed to ensure shared service %s: %w", svc, err)
				}

				// Register in state
				if err := shareMgr.RegisterSharedService(svc, project, containerName, ip, composeFiles); err != nil {
					return fmt.Errorf("failed to register shared service %s: %w", svc, err)
				}

				// Connect to environment network
				if err := shareMgr.ConnectSharedServiceToEnvironment(svc, project, name); err != nil {
					return fmt.Errorf("failed to connect shared service %s: %w", svc, err)
				}

				// Get IP for this specific network
				ip, err = shareMgr.GetSharedServiceIP(svc, project, name)
				if err != nil {
					return fmt.Errorf("failed to get shared service IP: %w", err)
				}

				// Add reference
				if err := shareMgr.AddEnvironmentReference(svc, project, project, name); err != nil {
					return fmt.Errorf("failed to add environment reference: %w", err)
				}

				// Add to environment's service list
				if env.Services == nil {
					env.Services = make(map[string]*models.Service)
				}
				env.Services[svc] = &models.Service{
					Name:      svc,
					IP:        ip,
					Container: containerName,
				}

				fmt.Printf("  ✓ Shared service %s connected (IP: %s)\n", svc, ip)
			}

			// Store shared services list in environment
			env.UsesSharedServices = sharedServices
		}

		fmt.Printf("Generating cilo override...\n")
		overridePath := filepath.Join(workspace, ".cilo", "override.yml")
		if err := compose.TransformWithShared(env, composeFiles, overridePath, dnsSuffix, sharedServices); err != nil {
			return fmt.Errorf("failed to generate override file: %w", err)
		}

		fmt.Printf("Starting containers...\n")
		if err := provider.Up(ctx, env, runtime.UpOptions{
			Build:    build,
			Recreate: recreate,
		}); err != nil {
			return err
		}

		if err := dns.UpdateDNS(env); err != nil {
			fmt.Printf("Warning: failed to update DNS: %v\n", err)
		}

		if err := state.UpdateEnvironment(env); err != nil {
			return err
		}

		fmt.Printf("✓ Environment %s is running\n", name)
		fmt.Printf("  Project: %s\n", project)

		ingressService := getIngressService(env)
		if ingressService != nil && len(ingressService.Hostnames) > 0 {
			fmt.Printf("\n🌐 Access URLs:\n")
			for _, hostname := range ingressService.Hostnames {
				fmt.Printf("  http://%s.%s.%s%s -> %s\n", hostname, project, name, dnsSuffix, ingressService.Name)
			}
			fmt.Printf("  http://%s.%s%s -> %s (apex)\n", project, name, dnsSuffix, ingressService.Name)
		} else if ingressService != nil {
			fmt.Printf("\n🌐 Access URL:\n")
			fmt.Printf("  http://%s.%s%s -> %s\n", ingressService.Name, name, dnsSuffix, ingressService.Name)
		}

		// Show other running services
		var otherServices []*models.Service
		for _, service := range env.Services {
			if !service.IsIngress {
				otherServices = append(otherServices, service)
			}
		}
		if len(otherServices) > 0 {
			fmt.Printf("\n📦 Services:\n")
			for _, service := range otherServices {
				serviceType := "isolated"
				if contains(env.UsesSharedServices, service.Name) {
					serviceType = "shared"
				}
				fmt.Printf("  %s: %s (%s)\n", service.Name, service.IP, serviceType)
			}
		}

		return nil
	},
}

var downCmd = &cobra.Command{
	Use:   "down <name>",
	Short: "Stop an environment",
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

		ctx := context.Background()
		provider := docker.NewProvider()

		// Disconnect shared services before stopping environment
		if len(env.UsesSharedServices) > 0 {
			fmt.Printf("Disconnecting shared services...\n")
			shareMgr := share.NewManager(provider, ctx)

			for _, svc := range env.UsesSharedServices {
				if err := shareMgr.DisconnectSharedServiceFromEnvironment(svc, project, name); err != nil {
					fmt.Printf("Warning: failed to disconnect shared service %s: %v\n", svc, err)
				}

				if err := shareMgr.RemoveEnvironmentReference(svc, project, project, name); err != nil {
					fmt.Printf("Warning: failed to remove environment reference: %v\n", err)
				}

				// Check if service should be stopped (grace period handled internally)
				if err := shareMgr.StopSharedServiceIfUnused(svc, project); err != nil {
					fmt.Printf("Warning: failed to stop shared service: %v\n", err)
				}
			}
		}

		if err := provider.Down(ctx, env); err != nil {
			return err
		}

		if err := state.UpdateEnvironment(env); err != nil {
			return err
		}

		fmt.Printf("✓ Environment %s stopped\n", name)
		return nil
	},
}

var destroyCmd = &cobra.Command{
	Use:   "destroy <name>",
	Short: "Destroy an environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, name, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}

		keepWorkspace, _ := cmd.Flags().GetBool("keep-workspace")
		force, _ := cmd.Flags().GetBool("force")

		env, err := state.GetEnvironment(project, name)
		if err != nil {
			return err
		}

		if !force {
			fmt.Printf("Are you sure you want to destroy %s in project %s? [y/N] ", name, project)
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
				fmt.Println("Cancelled")
				return nil
			}
		}

		provider := docker.NewProvider()
		ctx := context.Background()
		if err := provider.Destroy(ctx, env); err != nil {
			return err
		}

		if err := dns.RemoveDNS(name); err != nil {
			fmt.Printf("Warning: failed to remove DNS entries: %v\n", err)
		}

		if !keepWorkspace {
			workspace := state.GetEnvStoragePath(project, name)
			if err := os.RemoveAll(workspace); err != nil {
				return fmt.Errorf("failed to remove workspace: %w", err)
			}
		}

		if err := state.DeleteEnvironment(project, name); err != nil {
			return err
		}

		fmt.Printf("✓ Environment %s destroyed from project %s\n", name, project)
		return nil
	},
}

func init() {
	createCmd.Flags().String("from", "", "Copy from existing project directory")
	createCmd.Flags().Bool("empty", false, "Create with no docker-compose.yml")
	createCmd.Flags().String("include", "", "Only copy matching files (glob pattern)")
	createCmd.Flags().String("project", "", "Project name (defaults to configured project or directory name)")

	upCmd.Flags().Bool("build", false, "Build images before starting")
	upCmd.Flags().Bool("recreate", false, "Force recreate containers")
	upCmd.Flags().String("project", "", "Project name (defaults to configured project)")
	upCmd.Flags().StringSlice("shared", []string{}, "Services to share across environments")
	upCmd.Flags().StringSlice("isolate", []string{}, "Services to isolate (override labels)")

	downCmd.Flags().String("project", "", "Project name (defaults to configured project)")

	destroyCmd.Flags().Bool("keep-workspace", false, "Don't delete the workspace directory")
	destroyCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	destroyCmd.Flags().String("project", "", "Project name (defaults to configured project)")
}

type CopyOptions struct {
	Include       string
	CopyDotDirs   []string
	IgnoreDotDirs []string
}

func copyProject(src, dst string, opts CopyOptions) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			if shouldSkipDotDir(info.Name(), opts) {
				return filepath.SkipDir
			}
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		if opts.Include != "" {
			matched, _ := filepath.Match(opts.Include, info.Name())
			if !matched {
				return nil
			}
		}

		return filesystem.CopyFile(path, dstPath)
	})
}

func shouldSkipDotDir(name string, opts CopyOptions) bool {
	if name == ".cilo" || name == ".git" {
		return false
	}
	for _, ignore := range opts.IgnoreDotDirs {
		if ignore == name {
			return true
		}
	}

	if len(opts.CopyDotDirs) == 0 {
		return true
	}

	for _, allowed := range opts.CopyDotDirs {
		if allowed == name {
			return false
		}
	}

	return true
}

// filterOut removes items from slice that are in the filter list
func filterOut(slice []string, filter []string) []string {
	result := []string{}
	for _, item := range slice {
		shouldFilter := false
		for _, f := range filter {
			if strings.TrimSpace(f) == item {
				shouldFilter = true
				break
			}
		}
		if !shouldFilter {
			result = append(result, item)
		}
	}
	return result
}
