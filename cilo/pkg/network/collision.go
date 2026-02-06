package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// dockerNetwork represents the structure of docker network inspect output
type dockerNetwork struct {
	Name string `json:"Name"`
	IPAM struct {
		Config []struct {
			Subnet string `json:"Subnet"`
		} `json:"Config"`
	} `json:"IPAM"`
}

// GetDockerNetworkSubnets returns a map of Docker network names to their subnets
func GetDockerNetworkSubnets(ctx context.Context) (map[string]string, error) {
	// Get all network IDs
	cmd := exec.CommandContext(ctx, "docker", "network", "ls", "--format", "{{.ID}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list docker networks: %w", err)
	}

	networkIDs := strings.Fields(string(output))
	if len(networkIDs) == 0 {
		return make(map[string]string), nil
	}

	// Inspect all networks at once
	inspectArgs := append([]string{"network", "inspect"}, networkIDs...)
	cmd = exec.CommandContext(ctx, "docker", inspectArgs...)
	inspectOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect docker networks: %w", err)
	}

	// Parse JSON array of networks
	var networks []dockerNetwork
	if err := json.Unmarshal(inspectOutput, &networks); err != nil {
		return nil, fmt.Errorf("failed to parse docker network inspect output: %w", err)
	}

	// Build map of network name -> subnet
	result := make(map[string]string)
	for _, network := range networks {
		if len(network.IPAM.Config) > 0 && network.IPAM.Config[0].Subnet != "" {
			result[network.Name] = network.IPAM.Config[0].Subnet
		}
	}

	return result, nil
}

// CheckSubnetCollision checks if a given subnet collides with any existing Docker network subnets
// Returns (hasCollision, collidingNetworkName, error)
func CheckSubnetCollision(ctx context.Context, subnet string) (bool, string, error) {
	// Parse the input subnet
	_, inputNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return false, "", fmt.Errorf("invalid subnet format: %w", err)
	}

	// Get all existing Docker network subnets
	networks, err := GetDockerNetworkSubnets(ctx)
	if err != nil {
		return false, "", err
	}

	// Check for collisions
	for networkName, existingSubnet := range networks {
		_, existingNet, err := net.ParseCIDR(existingSubnet)
		if err != nil {
			// Skip networks with invalid subnets
			continue
		}

		if subnetsOverlap(inputNet, existingNet) {
			return true, networkName, nil
		}
	}

	return false, "", nil
}

// subnetsOverlap checks if two subnets overlap
func subnetsOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}
