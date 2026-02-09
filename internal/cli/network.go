// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/sharedco/cilo/internal/dns"
	"github.com/sharedco/cilo/internal/network"
	"github.com/sharedco/cilo/internal/state"
	"github.com/spf13/cobra"
)

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Network management commands",
}

var networkStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show network configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := state.LoadState()
		if err != nil {
			return err
		}

		fmt.Printf("Base Subnet: %s\n", st.BaseSubnet)
		fmt.Printf("DNS Port:    %d\n", st.DNSPort)
		fmt.Printf("Next Subnet: %s%d.0/24\n", st.BaseSubnet, st.SubnetCounter+1)

		return nil
	},
}

var networkMigrateCmd = &cobra.Command{
	Use:   "migrate [new-subnet]",
	Short: "Migrate environments to a new base subnet",
	Long: `Migrate all environments to a new base subnet. 
If no subnet is provided, it will automatically find an available one.
NOTE: This will destroy all current environments and recreate them (without losing data in workspaces).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := state.LoadState()
		if err != nil {
			return err
		}

		newSubnet := ""
		if len(args) > 0 {
			newSubnet = args[0]
		} else {
			fmt.Println("🔍 Scanning for available subnet...")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			newSubnet, err = network.FindAvailableBaseSubnet(ctx, 224)
			if err != nil {
				return err
			}
		}

		if newSubnet == st.BaseSubnet {
			fmt.Printf("Already using subnet %s\n", newSubnet)
			return nil
		}

		fmt.Printf("Migrating from %s to %s...\n", st.BaseSubnet, newSubnet)
		fmt.Println("⚠️  This will restart all environments.")
		fmt.Print("Continue? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}

		// Update state
		st.BaseSubnet = newSubnet
		st.SubnetCounter = 0

		// Re-calculate subnets for all environments
		for _, host := range st.Hosts {
			for _, env := range host.Environments {
				st.SubnetCounter++
				env.Subnet = fmt.Sprintf("%s%d.0/24", st.BaseSubnet, st.SubnetCounter)
				fmt.Printf("  - %s/%s -> %s\n", env.Project, env.Name, env.Subnet)
			}
		}

		if err := state.SaveState(st); err != nil {
			return err
		}

		fmt.Println("✓ State updated. Please run 'cilo doctor --fix' to reconcile environments and DNS.")
		fmt.Println("Or run 'cilo up' for each environment.")

		// Automatically trigger DNS update if possible
		if err := dns.UpdateDNSFromState(st); err != nil {
			fmt.Printf("Warning: DNS update failed: %v\n", err)
		}

		return nil
	},
}

func init() {
	networkCmd.AddCommand(networkStatusCmd)
	networkCmd.AddCommand(networkMigrateCmd)
	rootCmd.AddCommand(networkCmd)
}
