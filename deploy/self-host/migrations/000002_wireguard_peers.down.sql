-- Drop wireguard_peers table and all associated indexes
DROP INDEX IF EXISTS idx_wireguard_peers_machine_ip;
DROP INDEX IF EXISTS idx_wireguard_peers_user_id;
DROP INDEX IF EXISTS idx_wireguard_peers_public_key;
DROP INDEX IF EXISTS idx_wireguard_peers_environment_id;
DROP INDEX IF EXISTS idx_wireguard_peers_machine_id;
DROP TABLE IF EXISTS wireguard_peers;
