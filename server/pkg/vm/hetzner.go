package vm

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/hetznercloud/hcloud-go/v2/hcloud/exp/actionutil"
)

// HetznerProvider implements Provider for Hetzner Cloud
type HetznerProvider struct {
	client     *hcloud.Client
	sshKeyIDs  []int64
	firewallID int64
}

// HetznerConfig contains configuration for the Hetzner provider
type HetznerConfig struct {
	Token      string
	SSHKeyIDs  []int64
	FirewallID int64
}

// NewHetznerProvider creates a new Hetzner provider
func NewHetznerProvider(cfg HetznerConfig) (*HetznerProvider, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("Hetzner API token is required")
	}

	client := hcloud.NewClient(
		hcloud.WithToken(cfg.Token),
		hcloud.WithApplication("cilo-server", "1.0.0"),
	)

	return &HetznerProvider{
		client:     client,
		sshKeyIDs:  cfg.SSHKeyIDs,
		firewallID: cfg.FirewallID,
	}, nil
}

// Name returns the provider name
func (h *HetznerProvider) Name() string {
	return "hetzner"
}

// Provision creates a new VM on Hetzner
func (h *HetznerProvider) Provision(ctx context.Context, config ProvisionConfig) (*Machine, error) {
	// Build SSH keys
	sshKeys := make([]*hcloud.SSHKey, len(h.sshKeyIDs))
	for i, id := range h.sshKeyIDs {
		sshKeys[i] = &hcloud.SSHKey{ID: id}
	}

	// Build firewalls
	var firewalls []*hcloud.ServerCreateFirewall
	if h.firewallID > 0 {
		firewalls = []*hcloud.ServerCreateFirewall{
			{Firewall: hcloud.Firewall{ID: h.firewallID}},
		}
	}

	// Build labels
	labels := config.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["managed-by"] = "cilo"
	labels["created"] = time.Now().Format(time.RFC3339)

	// Create server options
	opts := hcloud.ServerCreateOpts{
		Name:             config.Name,
		ServerType:       &hcloud.ServerType{Name: config.Size},
		Location:         &hcloud.Location{Name: config.Region},
		SSHKeys:          sshKeys,
		Firewalls:        firewalls,
		Labels:           labels,
		StartAfterCreate: hcloud.Ptr(true),
	}

	// Set image
	if config.ImageID != "" {
		imageID, err := strconv.ParseInt(config.ImageID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid image ID: %w", err)
		}
		opts.Image = &hcloud.Image{ID: imageID}
	} else {
		opts.Image = &hcloud.Image{Name: "ubuntu-24.04"}
	}

	// Set user data if provided
	if config.UserData != "" {
		opts.UserData = config.UserData
	}

	// Create the server
	result, _, err := h.client.Server.Create(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}

	// Wait for server to be ready
	err = h.client.Action.WaitFor(ctx, actionutil.AppendNext(result.Action, result.NextActions)...)
	if err != nil {
		// Try to clean up
		h.client.Server.Delete(ctx, result.Server)
		return nil, fmt.Errorf("wait for server: %w", err)
	}

	// Get the full server info
	server, _, err := h.client.Server.GetByID(ctx, result.Server.ID)
	if err != nil {
		return nil, fmt.Errorf("get server: %w", err)
	}

	return &Machine{
		ID:           fmt.Sprintf("hetzner-%d", server.ID),
		ProviderID:   strconv.FormatInt(server.ID, 10),
		ProviderType: "hetzner",
		PublicIP:     server.PublicNet.IPv4.IP.String(),
		Status:       MachineStatusProvisioning,
		Region:       config.Region,
		Size:         config.Size,
		CreatedAt:    time.Now(),
	}, nil
}

// Destroy removes a VM from Hetzner
func (h *HetznerProvider) Destroy(ctx context.Context, providerID string) error {
	serverID, err := strconv.ParseInt(providerID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid server ID: %w", err)
	}

	result, _, err := h.client.Server.DeleteWithResult(ctx, &hcloud.Server{ID: serverID})
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}

	// Wait for deletion
	if result.Action != nil {
		if err := h.client.Action.WaitFor(ctx, result.Action); err != nil {
			return fmt.Errorf("wait for deletion: %w", err)
		}
	}

	return nil
}

// List returns all VMs managed by Cilo on Hetzner
func (h *HetznerProvider) List(ctx context.Context) ([]*Machine, error) {
	servers, err := h.client.Server.AllWithOpts(ctx, hcloud.ServerListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: "managed-by=cilo",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}

	machines := make([]*Machine, len(servers))
	for i, server := range servers {
		machines[i] = &Machine{
			ID:           fmt.Sprintf("hetzner-%d", server.ID),
			ProviderID:   strconv.FormatInt(server.ID, 10),
			ProviderType: "hetzner",
			PublicIP:     server.PublicNet.IPv4.IP.String(),
			Status:       mapHetznerStatus(server.Status),
			Region:       server.Datacenter.Location.Name,
			Size:         server.ServerType.Name,
			CreatedAt:    server.Created,
		}
	}

	return machines, nil
}

// HealthCheck checks if a Hetzner server is healthy
func (h *HetznerProvider) HealthCheck(ctx context.Context, providerID string) (bool, error) {
	serverID, err := strconv.ParseInt(providerID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid server ID: %w", err)
	}

	server, _, err := h.client.Server.GetByID(ctx, serverID)
	if err != nil {
		return false, fmt.Errorf("get server: %w", err)
	}

	if server == nil {
		return false, fmt.Errorf("server not found")
	}

	return server.Status == hcloud.ServerStatusRunning, nil
}

// GetMachine returns a single machine by provider ID
func (h *HetznerProvider) GetMachine(ctx context.Context, providerID string) (*Machine, error) {
	serverID, err := strconv.ParseInt(providerID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid server ID: %w", err)
	}

	server, _, err := h.client.Server.GetByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("get server: %w", err)
	}

	if server == nil {
		return nil, fmt.Errorf("server not found")
	}

	return &Machine{
		ID:           fmt.Sprintf("hetzner-%d", server.ID),
		ProviderID:   strconv.FormatInt(server.ID, 10),
		ProviderType: "hetzner",
		PublicIP:     server.PublicNet.IPv4.IP.String(),
		Status:       mapHetznerStatus(server.Status),
		Region:       server.Datacenter.Location.Name,
		Size:         server.ServerType.Name,
		CreatedAt:    server.Created,
	}, nil
}

func mapHetznerStatus(status hcloud.ServerStatus) string {
	switch status {
	case hcloud.ServerStatusRunning:
		return MachineStatusReady
	case hcloud.ServerStatusInitializing:
		return MachineStatusProvisioning
	case hcloud.ServerStatusStarting:
		return MachineStatusProvisioning
	case hcloud.ServerStatusStopping:
		return MachineStatusDraining
	case hcloud.ServerStatusOff:
		return MachineStatusDraining
	case hcloud.ServerStatusDeleting:
		return MachineStatusDestroying
	default:
		return MachineStatusFailed
	}
}
