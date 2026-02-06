package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"

	"github.com/cilo/cilo/pkg/config"
	"github.com/cilo/cilo/pkg/dns"
	"github.com/cilo/cilo/pkg/state"
	"github.com/spf13/cobra"
)

var (
	version = "0.1.20"
	commit  = "unknown"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "cilo",
	Short: "cilo - isolated workspace environments for AI agents",
	Long: `cilo creates isolated workspace environments from docker-compose projects.
Each environment has its own mutable copy of the compose file, a unique DNS namespace,
and runs on an isolated Docker network accessible via DNS names rather than ports.`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(pathCmd)
	rootCmd.AddCommand(composeCmd)
	rootCmd.AddCommand(dnsCmd)
	rootCmd.AddCommand(hostnamesCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize cilo (requires sudo)",
	Long: `Initialize cilo (one-time setup).

This command requires sudo because it configures system DNS settings.
It will automatically prompt for your password if needed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getuid() != 0 {
			// Get the absolute path of this executable
			exe, err := os.Executable()
			if err != nil {
				fmt.Println("cilo init requires sudo privileges to configure system DNS.")
				fmt.Println()
				fmt.Println("Could not determine executable path. Please run manually:")
				fmt.Println("  sudo /path/to/cilo init")
				return fmt.Errorf("could not determine executable path: %w", err)
			}

			fmt.Println("cilo init requires sudo privileges. Requesting elevation...")
			fmt.Println()

			// Re-invoke with sudo using absolute path, preserving user's HOME
			homeDir := os.Getenv("HOME")
			sudoCmd := exec.Command("sudo", "CILO_USER_HOME="+homeDir, exe, "init")
			sudoCmd.Stdin = os.Stdin
			sudoCmd.Stdout = os.Stdout
			sudoCmd.Stderr = os.Stderr

			if err := sudoCmd.Run(); err != nil {
				// If sudo failed, provide manual instructions
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
					return nil // User cancelled sudo, exit cleanly
				}
				fmt.Println()
				fmt.Println("Sudo elevation failed. You can also run manually:")
				fmt.Printf("  sudo %s init\n", exe)
				fmt.Println()
				fmt.Println("Or for manual DNS setup instructions:")
				fmt.Println("  cilo dns setup --print-manual")
				return err
			}
			return nil
		}

		fmt.Println("Initializing cilo...")

		ciloDir := config.GetCiloHome()
		dirs := []string{
			ciloDir,
			config.GetEnvsDir(),
			config.GetDNSDir(),
		}
		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}

		if err := state.InitializeState(); err != nil {
			return fmt.Errorf("failed to initialize state: %w", err)
		}

		fmt.Println("✓ cilo state initialized")

		if err := dns.SetupDNS(); err != nil {
			return fmt.Errorf("DNS setup failed: %w", err)
		}
		fmt.Println("✓ DNS daemon started")

		if err := dns.SetupSystemResolver(); err != nil {
			return fmt.Errorf("system resolver setup failed: %w", err)
		}
		fmt.Println("✓ System DNS configured")

		if err := fixOwnership(ciloDir); err != nil {
			return fmt.Errorf("failed to fix ownership: %w", err)
		}

		fmt.Println("\n✓ cilo initialized successfully")
		return nil
	},
}

func fixOwnership(ciloDir string) error {
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		return nil
	}

	u, err := user.Lookup(sudoUser)
	if err != nil {
		return err
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	return chownRecursive(ciloDir, uid, gid)
}

func chownRecursive(path string, uid, gid int) error {
	return exec.Command("chown", "-R", fmt.Sprintf("%d:%d", uid, gid), path).Run()
}
