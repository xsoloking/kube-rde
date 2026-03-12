-- +goose Up
ALTER TABLE teams
    ADD COLUMN IF NOT EXISTS cluster_name TEXT NOT NULL DEFAULT 'default';

COMMENT ON COLUMN teams.cluster_name IS 'Karmada member cluster name; "default" = hub cluster local scheduling';

-- +goose Down
ALTER TABLE teams DROP COLUMN IF EXISTS cluster_name;
