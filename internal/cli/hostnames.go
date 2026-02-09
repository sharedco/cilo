// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/sharedco/cilo/internal/dns"
	"github.com/sharedco/cilo/internal/models"
	"github.com/sharedco/cilo/internal/state"
	"github.com/spf13/cobra"
)

var hostnamesCmd = &cobra.Command{
	Use:   "hostnames",
	Short: "Manage custom hostnames for an environment",
	Long: `Manage custom DNS hostnames that map to your environment's ingress service.

Hostnames are added in addition to the default service names.

Examples:
  # List all hostnames for an environment
  cilo hostnames list myapp-dev

  # Add a hostname
  cilo hostnames add myapp-dev api.myapp

  # Add multiple hostnames
  cilo hostnames add myapp-dev admin.myapp,stats.myapp

  # Remove a hostname
  cilo hostnames remove myapp-dev api.myapp

  # Set hostnames from a file
  cilo hostnames set myapp-dev --file hostnames.txt
`,
}

var hostnamesListCmd = &cobra.Command{
	Use:   "list <env>",
	Short: "List all hostnames for an environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, envName, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}

		env, err := state.GetEnvironment(project, envName)
		if err != nil {
			return err
		}

		dnsSuffix := getEnvDNSSuffix(env)

		fmt.Printf("Hostnames for environment %q:\n\n", envName)

		if env.Project != "" {
			fmt.Printf("Project: %s\n", env.Project)
			fmt.Printf("Wildcard DNS: *.%s.%s%s -> ingress IP\n", env.Project, envName, dnsSuffix)
			fmt.Printf("Apex DNS: %s.%s%s -> ingress IP\n\n", env.Project, envName, dnsSuffix)
		}

		ingressService := getIngressService(env)
		if ingressService == nil {
			fmt.Println("No ingress service configured.")
			fmt.Println("Add 'cilo.ingress: true' label to a service in docker-compose.yml")
			return nil
		}

		fmt.Printf("Ingress service: %s (IP: %s)\n\n", ingressService.Name, ingressService.IP)

		if len(ingressService.Hostnames) == 0 {
			fmt.Println("No custom hostnames configured.")
			fmt.Printf("Default: %s.%s%s\n", ingressService.Name, envName, dnsSuffix)
		} else {
			fmt.Println("Custom hostnames:")
			for _, h := range ingressService.Hostnames {
				fmt.Printf("  %s.%s%s -> %s\n", h, envName, dnsSuffix, ingressService.IP)
			}
		}

		return nil
	},
}

var hostnamesAddCmd = &cobra.Command{
	Use:   "add <env> <hostname>...",
	Short: "Add custom hostnames to an environment",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, envName, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}
		hostnameArgs := args[1:]

		env, err := state.GetEnvironment(project, envName)
		if err != nil {
			return err
		}

		dnsSuffix := getEnvDNSSuffix(env)

		ingressService := getIngressService(env)
		if ingressService == nil {
			return fmt.Errorf("no ingress service found in environment %q\nAdd 'cilo.ingress: true' label to a service", envName)
		}

		var newHostnames []string
		for _, arg := range hostnameArgs {
			parts := strings.Split(arg, ",")
			for _, part := range parts {
				h := strings.TrimSpace(part)
				if h != "" {
					newHostnames = append(newHostnames, h)
				}
			}
		}

		existing := make(map[string]bool)
		for _, h := range ingressService.Hostnames {
			existing[h] = true
		}

		added := 0
		for _, h := range newHostnames {
			if !existing[h] {
				ingressService.Hostnames = append(ingressService.Hostnames, h)
				existing[h] = true
				added++
				fmt.Printf("✓ Added hostname: %s.%s%s\n", h, envName, dnsSuffix)
			} else {
				fmt.Printf("⚠ Hostname already exists: %s.%s%s\n", h, envName, dnsSuffix)
			}
		}

		if added > 0 {
			if err := state.UpdateEnvironment(env); err != nil {
				return fmt.Errorf("failed to update environment: %w", err)
			}

			if err := dns.UpdateDNS(env); err != nil {
				return fmt.Errorf("failed to update DNS: %w", err)
			}

			fmt.Printf("\n✓ DNS updated. Test with: dig %s.%s%s\n", newHostnames[0], envName, dnsSuffix)
		}

		return nil
	},
}

var hostnamesRemoveCmd = &cobra.Command{
	Use:   "remove <env> <hostname>...",
	Short: "Remove custom hostnames from an environment",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, envName, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}
		hostnameArgs := args[1:]

		env, err := state.GetEnvironment(project, envName)
		if err != nil {
			return err
		}

		dnsSuffix := getEnvDNSSuffix(env)

		ingressService := getIngressService(env)
		if ingressService == nil {
			return fmt.Errorf("no ingress service found")
		}

		var toRemove []string
		for _, arg := range hostnameArgs {
			parts := strings.Split(arg, ",")
			for _, part := range parts {
				h := strings.TrimSpace(part)
				if h != "" {
					toRemove = append(toRemove, h)
				}
			}
		}

		removeMap := make(map[string]bool)
		for _, h := range toRemove {
			removeMap[h] = true
		}

		var newHostnames []string
		removed := 0
		for _, h := range ingressService.Hostnames {
			if removeMap[h] {
				removed++
				fmt.Printf("✓ Removed hostname: %s.%s%s\n", h, envName, dnsSuffix)
			} else {
				newHostnames = append(newHostnames, h)
			}
		}

		ingressService.Hostnames = newHostnames

		if removed > 0 {
			if err := state.UpdateEnvironment(env); err != nil {
				return fmt.Errorf("failed to update environment: %w", err)
			}

			if err := dns.UpdateDNS(env); err != nil {
				return fmt.Errorf("failed to update DNS: %w", err)
			}

			fmt.Printf("\n✓ DNS updated\n")
		} else {
			fmt.Println("No matching hostnames found")
		}

		return nil
	},
}

var hostnamesSetCmd = &cobra.Command{
	Use:   "set <env> --file <file>",
	Short: "Set hostnames from a file (replaces existing)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, envName, err := getProjectAndEnv(cmd, args)
		if err != nil {
			return err
		}
		filePath, _ := cmd.Flags().GetString("file")

		if filePath == "" {
			return fmt.Errorf("--file flag required")
		}

		env, err := state.GetEnvironment(project, envName)
		if err != nil {
			return err
		}

		dnsSuffix := getEnvDNSSuffix(env)

		ingressService := getIngressService(env)
		if ingressService == nil {
			return fmt.Errorf("no ingress service found")
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		var hostnames []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			hostnames = append(hostnames, line)
		}

		ingressService.Hostnames = hostnames

		if err := state.UpdateEnvironment(env); err != nil {
			return fmt.Errorf("failed to update environment: %w", err)
		}

		if err := dns.UpdateDNS(env); err != nil {
			return fmt.Errorf("failed to update DNS: %w", err)
		}

		fmt.Printf("✓ Set %d hostnames for environment %q\n", len(hostnames), envName)
		for _, h := range hostnames {
			fmt.Printf("  %s.%s%s\n", h, envName, dnsSuffix)
		}

		return nil
	},
}

func getIngressService(env *models.Environment) *models.Service {
	for _, svc := range env.Services {
		if svc.IsIngress {
			return svc
		}
	}
	return nil
}

func getEnvDNSSuffix(env *models.Environment) string {
	if env == nil || env.DNSSuffix == "" {
		return ".test"
	}
	return env.DNSSuffix
}

func init() {
	hostnamesCmd.AddCommand(hostnamesListCmd)
	hostnamesCmd.AddCommand(hostnamesAddCmd)
	hostnamesCmd.AddCommand(hostnamesRemoveCmd)
	hostnamesCmd.AddCommand(hostnamesSetCmd)

	hostnamesSetCmd.Flags().String("file", "", "File containing hostnames (one per line)")
}
