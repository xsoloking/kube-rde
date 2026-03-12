-- +goose Up
CREATE TABLE IF NOT EXISTS agent_pod_sessions (
    agent_id   TEXT      PRIMARY KEY,
    pod_ip     TEXT      NOT NULL,
    pod_port   INTEGER   NOT NULL DEFAULT 8080,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS agent_pod_sessions;
