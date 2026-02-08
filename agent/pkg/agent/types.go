package agent

// UpRequest is the request body for POST /environment/up
type UpRequest struct {
	WorkspacePath string `json:"workspace_path"`
	EnvName       string `json:"env_name"`
	Subnet        string `json:"subnet"`
	Build         bool   `json:"build,omitempty"`
	Recreate      bool   `json:"recreate,omitempty"`
}

// UpResponse is the response for POST /environment/up
type UpResponse struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"` // service name -> IP
}

// AddPeerRequest is the request body for POST /wireguard/add-peer
type AddPeerRequest struct {
	PublicKey  string `json:"public_key"`
	AllowedIPs string `json:"allowed_ips"`
}

// WGStatusResponse shows WireGuard status
type WGStatusResponse struct {
	Interface string       `json:"interface"`
	PublicKey string       `json:"public_key"`
	Peers     []PeerStatus `json:"peers"`
}

// PeerStatus represents the status of a single WireGuard peer
type PeerStatus struct {
	PublicKey     string `json:"public_key"`
	Endpoint      string `json:"endpoint,omitempty"`
	AllowedIPs    string `json:"allowed_ips"`
	LastHandshake string `json:"last_handshake,omitempty"`
	RxBytes       int64  `json:"rx_bytes"`
	TxBytes       int64  `json:"tx_bytes"`
}

// ServiceStatus represents the status of a Docker Compose service
type ServiceStatus struct {
	Service string `json:"service"`
	State   string `json:"state"`  // running, exited, etc.
	Status  string `json:"status"` // Up 2 hours, Exited (0), etc.
	Health  string `json:"health"` // healthy, unhealthy, etc.
}
