package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/cilo/cilo/pkg/compose"
	"github.com/cilo/cilo/pkg/config"
	"github.com/cilo/cilo/pkg/models"
	"github.com/cilo/cilo/pkg/state"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <command> <env>",
	Short: "Run a command in an environment workspace",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runRun,
}

func init() {
	runCmd.Flags().String("from", "", "Project source path (default: current directory)")
	runCmd.Flags().String("project", "", "Project name (default: directory basename)")
	runCmd.Flags().Bool("no-up", false, "Don't start the environment")
	runCmd.Flags().Bool("no-create", false, "Don't create if missing")
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	command := args[0]
	envName := state.NormalizeName(args[1])
	cmdArgs := args[2:]

	fromPath, _ := cmd.Flags().GetString("from")
	projectFlag, _ := cmd.Flags().GetString("project")
	noUp, _ := cmd.Flags().GetBool("no-up")
	noCreate, _ := cmd.Flags().GetBool("no-create")

	if !isInitialized() {
		return fmt.Errorf(`cilo is not initialized. DNS resolution for *.test domains won't work.

Run:
  sudo cilo init

Then retry:
  cilo run %s %s`, command, envName)
	}

	if fromPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		fromPath = cwd
	}
	fromPath, _ = filepath.Abs(fromPath)

	project := projectFlag
	if project == "" {
		sourceConfig, err := models.LoadProjectConfigFromPath(fromPath)
		if err != nil {
			return fmt.Errorf("failed to load project config: %w", err)
		}
		if sourceConfig != nil && sourceConfig.Project != "" {
			project = sourceConfig.Project
		} else {
			project = filepath.Base(fromPath)
		}
	}
	project = state.NormalizeName(project)

	env, err := state.GetEnvironment(project, envName)
	if err != nil {
		if noCreate {
			return fmt.Errorf("environment %s/%s does not exist (use 'cilo create' first, or remove --no-create)", project, envName)
		}

		fmt.Printf("Creating environment: %s/%s\n", project, envName)

		env, err = state.CreateEnvironment(envName, fromPath, project)
		if err != nil {
			return fmt.Errorf("failed to create environment: %w", err)
		}

		workspace := state.GetEnvStoragePath(project, envName)
		if err := os.MkdirAll(workspace, 0755); err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}

		sourceConfig, err := models.LoadProjectConfigFromPath(fromPath)
		if err != nil {
			return fmt.Errorf("failed to load project config: %w", err)
		}

		copyOpts := CopyOptions{}
		if sourceConfig != nil {
			copyOpts.CopyDotDirs = sourceConfig.CopyDotDirs
			copyOpts.IgnoreDotDirs = sourceConfig.IgnoreDotDirs
		}

		if err := copyProject(fromPath, workspace, copyOpts); err != nil {
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

		ciloDir := filepath.Join(workspace, ".cilo")
		os.MkdirAll(ciloDir, 0755)

		fmt.Printf("✓ Created environment: %s/%s\n", project, envName)
	}

	if !noUp && env.Status != "running" {
		fmt.Printf("Starting environment: %s/%s\n", project, envName)

		upArgs := []string{envName}
		upCommand := upCmd
		upCommand.Flags().Set("project", project)
		if err := upCommand.RunE(upCommand, upArgs); err != nil {
			return fmt.Errorf("failed to start environment: %w", err)
		}
	}

	workspace := state.GetEnvStoragePath(project, envName)
	projectConfig, err := models.LoadProjectConfigFromPath(workspace)
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	dnsSuffix := ".test"
	if projectConfig != nil && projectConfig.DNSSuffix != "" {
		dnsSuffix = projectConfig.DNSSuffix
	}

	cmdPath, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("command not found: %s", command)
	}

	environ := os.Environ()
	environ = append(environ,
		fmt.Sprintf("CILO_ENV=%s", envName),
		fmt.Sprintf("CILO_PROJECT=%s", project),
		fmt.Sprintf("CILO_WORKSPACE=%s", workspace),
		fmt.Sprintf("CILO_BASE_URL=http://%s.%s%s", project, envName, dnsSuffix),
		fmt.Sprintf("CILO_DNS_SUFFIX=%s", dnsSuffix),
	)

	if err := os.Chdir(workspace); err != nil {
		return fmt.Errorf("failed to change to workspace: %w", err)
	}

	fmt.Printf("\nLaunching %s in %s\n\n", command, workspace)

	execArgs := append([]string{command}, cmdArgs...)

	return syscall.Exec(cmdPath, execArgs, environ)
}

func isInitialized() bool {
	statePath := config.GetStatePath()
	_, err := os.Stat(statePath)
	return err == nil
}
