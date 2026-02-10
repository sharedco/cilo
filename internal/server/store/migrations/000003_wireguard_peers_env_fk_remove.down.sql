-- Restore the wireguard_peers -> environments FK (legacy behavior)

ALTER TABLE wireguard_peers
  ADD CONSTRAINT wireguard_peers_environment_id_fkey
  FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE;
