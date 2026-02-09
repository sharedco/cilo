-- Create wireguard_peers table for managing WireGuard peer connections
CREATE TABLE IF NOT EXISTS wireguard_peers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id TEXT NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    public_key TEXT UNIQUE NOT NULL,
    assigned_ip INET NOT NULL,
    connected_at TIMESTAMPTZ DEFAULT NOW(),
    last_seen TIMESTAMPTZ DEFAULT NOW()
);

-- Create indexes for efficient queries
CREATE INDEX idx_wireguard_peers_machine_id ON wireguard_peers(machine_id);
CREATE INDEX idx_wireguard_peers_environment_id ON wireguard_peers(environment_id);
CREATE INDEX idx_wireguard_peers_public_key ON wireguard_peers(public_key);
CREATE INDEX idx_wireguard_peers_user_id ON wireguard_peers(user_id);

-- Create unique constraint to prevent duplicate IPs per machine
CREATE UNIQUE INDEX idx_wireguard_peers_machine_ip ON wireguard_peers(machine_id, assigned_ip);
