-- +goose Up
ALTER TABLE agent_pod_sessions
    ADD COLUMN IF NOT EXISTS cluster_name TEXT NOT NULL DEFAULT 'default';

-- +goose Down
ALTER TABLE agent_pod_sessions DROP COLUMN IF EXISTS cluster_name;
