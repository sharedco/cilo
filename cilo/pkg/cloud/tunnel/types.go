package tunnel

import "time"

// PeerStats contains statistics for a WireGuard peer
type PeerStats struct {
	PublicKey     string
	Endpoint      string
	AllowedIPs    []string
	LastHandshake time.Time
	RxBytes       int64
	TxBytes       int64
}
