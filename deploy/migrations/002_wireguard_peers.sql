-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS public.agent_wireguard_peers (
    agent_id    varchar(255) NOT NULL,
    public_key  varchar(255) NOT NULL,
    endpoints   text         NOT NULL DEFAULT '[]',
    updated_at  timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT agent_wireguard_peers_pkey PRIMARY KEY (agent_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS public.agent_wireguard_peers;

-- +goose StatementEnd
