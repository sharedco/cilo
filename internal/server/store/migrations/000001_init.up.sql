-- Create teams table
CREATE TABLE IF NOT EXISTS teams (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create api_keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    key_hash TEXT NOT NULL UNIQUE,
    prefix TEXT NOT NULL,
    scope TEXT NOT NULL CHECK (scope IN ('read', 'write', 'admin')),
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used TIMESTAMP
);

CREATE INDEX idx_api_keys_team_id ON api_keys(team_id);
CREATE INDEX idx_api_keys_prefix ON api_keys(prefix);

-- Create machines table
CREATE TABLE IF NOT EXISTS machines (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL,
    provider_type TEXT NOT NULL,
    public_ip TEXT NOT NULL,
    wg_public_key TEXT,
    wg_endpoint TEXT,
    status TEXT NOT NULL CHECK (status IN ('provisioning', 'ready', 'error', 'destroyed')),
    assigned_env TEXT,
    ssh_host TEXT NOT NULL,
    ssh_user TEXT NOT NULL,
    region TEXT NOT NULL,
    size TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_machines_status ON machines(status);
CREATE INDEX idx_machines_assigned_env ON machines(assigned_env);

-- Create environments table
CREATE TABLE IF NOT EXISTS environments (
    id TEXT PRIMARY KEY,
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    project TEXT NOT NULL,
    format TEXT NOT NULL CHECK (format IN ('docker-compose', 'devcontainer')),
    machine_id TEXT REFERENCES machines(id) ON DELETE SET NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'provisioning', 'ready', 'error', 'destroyed')),
    subnet TEXT NOT NULL,
    services JSONB DEFAULT '[]'::jsonb,
    peers JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL,
    source TEXT NOT NULL
);

CREATE INDEX idx_environments_team_id ON environments(team_id);
CREATE INDEX idx_environments_status ON environments(status);
CREATE INDEX idx_environments_machine_id ON environments(machine_id);

-- Create usage_records table
CREATE TABLE IF NOT EXISTS usage_records (
    id TEXT PRIMARY KEY,
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    duration_sec INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_usage_records_team_id ON usage_records(team_id);
CREATE INDEX idx_usage_records_environment_id ON usage_records(environment_id);
CREATE INDEX idx_usage_records_start_time ON usage_records(start_time);
