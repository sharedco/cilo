-- Decouple wireguard_peers from environments so a single WireGuard tunnel
-- (peer) can be reused across multiple environments on the same machine.

ALTER TABLE wireguard_peers
  DROP CONSTRAINT IF EXISTS wireguard_peers_environment_id_fkey;
