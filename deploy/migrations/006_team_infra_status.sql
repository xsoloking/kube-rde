-- +goose Up
-- Add infra_status to teams to track the lifecycle of the team's Kubernetes infrastructure.
-- Values: pending (CR not yet created), syncing (CR created, Operator reconciling),
--         ready (Operator confirmed all resources provisioned), error (last attempt failed).
ALTER TABLE teams ADD COLUMN IF NOT EXISTS infra_status VARCHAR(20) NOT NULL DEFAULT 'pending';

-- Back-fill existing teams: if they have a namespace in k8s already, treat them as ready.
-- This is a best-effort update; teams that genuinely need re-provisioning can be reset manually.
UPDATE teams SET infra_status = 'ready' WHERE status = 'active';

-- +goose Down
ALTER TABLE teams DROP COLUMN IF EXISTS infra_status;
