// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var cleanSlateCmd = &cobra.Command{
	Use:   "clean-slate",
	Short: "Remove all environments and reset to fresh state",
	Long: `Clean slate - destroys all environments and removes all state.

This command will:
  - Stop and remove all cilo Docker containers
  - Remove all cilo Docker networks
  - Stop the dnsmasq DNS daemon
  - Remove all environment state and workspace data
  - Preserve the DNS configuration (will be refreshed on next init)

After running this, you'll have a completely fresh cilo installation
ready for 'sudo cilo init'.

Use with caution - this destroys all your local cilo environments!`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🧹 Clean slate - removing all cilo environments and state...")
		fmt.Println()

		fmt.Println("  → Stopping all cilo containers...")
		stopContainers()

		fmt.Println("  → Removing all cilo containers...")
		rmContainers()

		fmt.Println("  → Removing all cilo networks...")
		rmNetworks()

		fmt.Println("  → Stopping dnsmasq...")
		stopDNS()

		fmt.Println("  → Removing cilo state...")
		rmState()

		fmt.Println()
		fmt.Println("✓ Clean slate complete! Ready for fresh start:")
		fmt.Println("   sudo cilo init")
		fmt.Println("   cd examples/basic && cilo create myenv && cilo up myenv")

		return nil
	},
}

func stopContainers() {
	cmd := exec.Command("docker", "ps", "-aq", "--filter", "name=cilo_")
	output, _ := cmd.Output()
	if len(output) > 0 {
		ids := strings.Fields(string(output))
		for _, id := range ids {
			exec.Command("docker", "stop", id).Run()
		}
	}
}

func rmContainers() {
	cmd := exec.Command("docker", "ps", "-aq", "--filter", "name=cilo_")
	output, _ := cmd.Output()
	if len(output) > 0 {
		ids := strings.Fields(string(output))
		for _, id := range ids {
			exec.Command("docker", "rm", id).Run()
		}
	}
}

func rmNetworks() {
	cmd := exec.Command("docker", "network", "ls", "--format", "{{.Name}}")
	output, _ := cmd.Output()
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cilo_") {
			exec.Command("docker", "network", "rm", line).Run()
		}
	}
}

func stopDNS() {
	exec.Command("sudo", "pkill", "-x", "dnsmasq").Run()
}

func rmState() {
	ciloHome := os.Getenv("HOME") + "/.cilo"
	os.RemoveAll(ciloHome + "/envs")
	os.Remove(ciloHome + "/state.json")
	os.Remove(ciloHome + "/state.json.lock")
}

func init() {
	rootCmd.AddCommand(cleanSlateCmd)
}
