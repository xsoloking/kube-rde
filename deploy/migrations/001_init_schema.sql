-- +goose Up
-- +goose StatementBegin

-- public.agent_templates definition

-- Drop table

-- DROP TABLE public.agent_templates;

CREATE TABLE public.agent_templates (
	id varchar(36) NOT NULL,
	"name" varchar(255) NOT NULL,
	agent_type varchar(50) NOT NULL,
	description text NULL,
	docker_image varchar(255) NOT NULL,
	default_local_target varchar(255) NOT NULL,
	default_external_port int4 NOT NULL,
	startup_args text NULL,
	env_vars jsonb NULL,
	security_context jsonb NULL,
	volume_mounts jsonb NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT agent_templates_agent_type_key UNIQUE (agent_type),
	CONSTRAINT agent_templates_pkey PRIMARY KEY (id)
);
CREATE INDEX idx_agent_templates_type ON public.agent_templates USING btree (agent_type);


-- public.resource_configs definition

-- Drop table

-- DROP TABLE public.resource_configs;

CREATE TABLE public.resource_configs (
	id int4 DEFAULT 1 NOT NULL,
	default_cpu_cores int4 DEFAULT 8 NOT NULL,
	default_memory_gi int4 DEFAULT 16 NOT NULL,
	storage_classes jsonb DEFAULT '[]'::jsonb NOT NULL,
	gpu_types jsonb DEFAULT '[]'::jsonb NOT NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT resource_configs_pkey PRIMARY KEY (id),
	CONSTRAINT single_row CHECK ((id = 1))
);


-- public.teams definition

-- Drop table

-- DROP TABLE public.teams;

CREATE TABLE public.teams (
	id serial4 NOT NULL,
	"name" varchar(255) NOT NULL,
	display_name varchar(255) NOT NULL,
	"namespace" varchar(255) NOT NULL,
	status varchar(50) DEFAULT 'active'::character varying NOT NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT teams_pkey PRIMARY KEY (id),
	CONSTRAINT teams_name_key UNIQUE (name),
	CONSTRAINT teams_namespace_key UNIQUE (namespace)
);
CREATE INDEX idx_teams_name ON public.teams USING btree (name);
CREATE INDEX idx_teams_status ON public.teams USING btree (status);


-- public.team_quotas definition

-- Drop table

-- DROP TABLE public.team_quotas;

CREATE TABLE public.team_quotas (
	id serial4 NOT NULL,
	team_id int4 NOT NULL,
	cpu_cores int4 DEFAULT 0 NOT NULL,
	memory_gi int4 DEFAULT 0 NOT NULL,
	storage_quota jsonb DEFAULT '[]'::jsonb NULL,
	gpu_quota jsonb DEFAULT '[]'::jsonb NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT team_quotas_pkey PRIMARY KEY (id),
	CONSTRAINT team_quotas_team_id_key UNIQUE (team_id)
);
CREATE INDEX idx_team_quotas_team_id ON public.team_quotas USING btree (team_id);


-- public.tunnel_connections definition

-- Drop table

-- DROP TABLE public.tunnel_connections;

CREATE TABLE public.tunnel_connections (
	id serial4 NOT NULL,
	agent_id varchar(255) NOT NULL,
	user_id varchar(255) NOT NULL,
	client_ip varchar(255) NULL,
	started_at timestamp NOT NULL,
	ended_at timestamp NULL,
	bytes_in int8 DEFAULT 0 NULL,
	bytes_out int8 DEFAULT 0 NULL,
	active bool DEFAULT true NULL,
	protocol varchar(50) NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NULL,
	CONSTRAINT tunnel_connections_pkey PRIMARY KEY (id)
);
CREATE INDEX idx_tunnel_connections_active ON public.tunnel_connections USING btree (active);
CREATE INDEX idx_tunnel_connections_agent_id ON public.tunnel_connections USING btree (agent_id);
CREATE INDEX idx_tunnel_connections_started_at ON public.tunnel_connections USING btree (started_at);
CREATE INDEX idx_tunnel_connections_user_id ON public.tunnel_connections USING btree (user_id);


-- public.audit_logs definition

-- Drop table

-- DROP TABLE public.audit_logs;

CREATE TABLE public.audit_logs (
	id varchar(255) NOT NULL,
	user_id varchar(255) NULL,
	"action" varchar(50) NOT NULL,
	resource varchar(50) NOT NULL,
	resource_id varchar(255) NULL,
	old_data text NULL,
	new_data text NULL,
	"timestamp" timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT audit_logs_pkey PRIMARY KEY (id)
);
CREATE INDEX idx_audit_logs_resource ON public.audit_logs USING btree (resource, resource_id);
CREATE INDEX idx_audit_logs_timestamp ON public.audit_logs USING btree ("timestamp");
CREATE INDEX idx_audit_logs_user_id ON public.audit_logs USING btree (user_id);


-- public.services definition

-- Drop table

-- DROP TABLE public.services;

CREATE TABLE public.services (
	id varchar(255) NOT NULL,
	workspace_id varchar(255) NOT NULL,
	"name" varchar(255) NOT NULL,
	local_target varchar(255) NOT NULL,
	external_port int4 NOT NULL,
	agent_id varchar(255) NULL,
	status varchar(50) DEFAULT 'stopped'::character varying NOT NULL,
	created_by_id varchar(255) NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	last_heartbeat timestamp NULL,
	agent_type varchar(50) NULL,
	template_id varchar(36) NULL,
	startup_args text NULL,
	env_vars jsonb NULL,
	cpu_cores varchar(50) NULL,
	memory_gib varchar(50) NULL,
	gpu_count int8 NULL,
	gpu_model varchar(255) NULL,
	gpu_resource_name varchar(255) NULL,
	gpu_node_selector jsonb NULL,
	ttl varchar(50) DEFAULT '24h'::character varying NULL,
	is_pinned bool DEFAULT false NOT NULL,
	CONSTRAINT services_pkey PRIMARY KEY (id)
);
CREATE INDEX idx_services_agent_id ON public.services USING btree (agent_id);
CREATE INDEX idx_services_agent_type ON public.services USING btree (agent_type);
CREATE INDEX idx_services_created_by_id ON public.services USING btree (created_by_id);
CREATE INDEX idx_services_is_pinned ON public.services USING btree (is_pinned) WHERE (is_pinned = true);
CREATE INDEX idx_services_template_id ON public.services USING btree (template_id);
CREATE INDEX idx_services_workspace_id ON public.services USING btree (workspace_id);
CREATE UNIQUE INDEX idx_services_workspace_port ON public.services USING btree (workspace_id, external_port);


-- public.user_quotas definition

-- Drop table

-- DROP TABLE public.user_quotas;

CREATE TABLE public.user_quotas (
	user_id varchar(255) NOT NULL,
	cpu_cores int4 NOT NULL,
	memory_gi int4 NOT NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	storage_quota jsonb DEFAULT '[]'::jsonb NULL,
	gpu_quota jsonb DEFAULT '[]'::jsonb NULL,
	CONSTRAINT user_quotas_pkey PRIMARY KEY (user_id)
);
CREATE INDEX idx_user_quotas_user_id ON public.user_quotas USING btree (user_id);


-- public.users definition

-- Drop table

-- DROP TABLE public.users;

CREATE TABLE public.users (
	id varchar(255) NOT NULL,
	username varchar(255) NOT NULL,
	email varchar(255) NULL,
	full_name varchar(255) NULL,
	default_workspace_id varchar(255) NULL,
	team_id int4 NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	ssh_keys jsonb DEFAULT '[]'::jsonb NULL, -- Array of SSH public keys
	CONSTRAINT users_pkey PRIMARY KEY (id),
	CONSTRAINT users_username_key UNIQUE (username)
);
CREATE INDEX idx_users_username ON public.users USING btree (username);
CREATE INDEX idx_users_team_id ON public.users USING btree (team_id);

-- Column comments

COMMENT ON COLUMN public.users.ssh_keys IS 'Array of SSH public keys';


-- public.workspaces definition

-- Drop table

-- DROP TABLE public.workspaces;

CREATE TABLE public.workspaces (
	id varchar(255) NOT NULL,
	"name" varchar(255) NOT NULL,
	description text NULL,
	owner_id varchar(255) NOT NULL,
	team_id int4 NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	storage_size varchar(20) DEFAULT '50Gi'::character varying NULL,
	pvc_name varchar(255) NULL,
	storage_class varchar(100) DEFAULT 'standard'::character varying NULL,
	CONSTRAINT workspaces_pkey PRIMARY KEY (id)
);
CREATE INDEX idx_workspaces_owner_id ON public.workspaces USING btree (owner_id);
CREATE INDEX idx_workspaces_pvc_name ON public.workspaces USING btree (pvc_name);
CREATE INDEX idx_workspaces_team_id ON public.workspaces USING btree (team_id);


-- public.audit_logs foreign keys

ALTER TABLE public.audit_logs ADD CONSTRAINT audit_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


-- public.services foreign keys

ALTER TABLE public.services ADD CONSTRAINT fk_services_template_id FOREIGN KEY (template_id) REFERENCES public.agent_templates(id) ON DELETE SET NULL;
ALTER TABLE public.services ADD CONSTRAINT services_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id) ON DELETE SET NULL;
ALTER TABLE public.services ADD CONSTRAINT services_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspaces(id) ON DELETE CASCADE;


-- public.user_quotas foreign keys

ALTER TABLE public.user_quotas ADD CONSTRAINT user_quotas_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


-- public.users foreign keys

ALTER TABLE public.users ADD CONSTRAINT fk_users_default_workspace FOREIGN KEY (default_workspace_id) REFERENCES public.workspaces(id) ON DELETE SET NULL;


-- public.workspaces foreign keys

ALTER TABLE public.workspaces ADD CONSTRAINT workspaces_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE public.workspaces ADD CONSTRAINT workspaces_team_id_fkey FOREIGN KEY (team_id) REFERENCES public.teams(id) ON DELETE SET NULL;


-- public.team_quotas foreign keys

ALTER TABLE public.team_quotas ADD CONSTRAINT team_quotas_team_id_fkey FOREIGN KEY (team_id) REFERENCES public.teams(id) ON DELETE CASCADE;


-- public.users foreign keys (team_id)

ALTER TABLE public.users ADD CONSTRAINT users_team_id_fkey FOREIGN KEY (team_id) REFERENCES public.teams(id) ON DELETE SET NULL;

-- =====================================================
-- SEED DATA: Agent Templates
-- =====================================================
INSERT INTO agent_templates (
    id, name, agent_type, description, docker_image,
    default_local_target, default_external_port,
    startup_args, env_vars, security_context, volume_mounts
) VALUES
-- SSH Server
(
    'tpl-ssh-001',
    'ssh',
    'ssh',
    'Secure remote shell access with no password required',
    'soloking/ssh-server:latest',
    '127.0.0.1:22',
    22,
    '',
    '{}'::jsonb,
    '{}'::jsonb,
    '[{"name": "workspace", "mountPath": "/root", "readOnly": false}]'::jsonb
),
-- File Browser
(
    'tpl-file-001',
    'file',
    'file',
    'Web-based file manager for workspace navigation and management',
    'filebrowser/filebrowser:latest',
    '127.0.0.1:80',
    80,
    '--noauth',
    '{}'::jsonb,
    '{"runAsUser": 0, "runAsGroup": 0, "fsGroup": 0}'::jsonb,
    '[{"name": "workspace", "mountPath": "/root", "readOnly": false}]'::jsonb
),
-- Coder Server (VS Code)
(
    'tpl-coder-001',
    'coder',
    'coder',
    'VS Code in the browser with full development environment',
    'codercom/code-server:latest',
    '127.0.0.1:8080',
    8080,
    '--auth none --bind-addr 0.0.0.0:8080 --user-data-dir /root/.local/share/code-server --config /root/.config/code-server/config.yaml /root',
    '{}'::jsonb,
    '{"runAsUser": 0, "runAsGroup": 0, "fsGroup": 0}'::jsonb,
    '[{"name": "workspace", "mountPath": "/root", "readOnly": false}]'::jsonb
),
-- Jupyter Server
(
    'tpl-jupyter-001',
    'jupyter',
    'jupyter',
    'Jupyter notebook for data science and interactive development',
    'jupyter/datascience-notebook:latest',
    '127.0.0.1:8888',
    8888,
    'start-notebook.py --allow-root --NotebookApp.token= --NotebookApp.password=',
    '{"GRANT_SUDO": "yes"}'::jsonb,
    '{"runAsUser": 0, "runAsGroup": 0, "fsGroup": 0}'::jsonb,
    '[{"name": "workspace", "mountPath": "/root", "readOnly": false}]'::jsonb
);

-- =====================================================
-- SEED DATA: Default Resource Configuration
-- =====================================================
INSERT INTO resource_configs (
    storage_classes,
    gpu_types,
    default_cpu_cores,
    default_memory_gi
) VALUES (
    '[{"name": "local-path", "limit_gi": 20, "is_default": true}]'::jsonb,
    '[{"name": "nvidia.com/gpu", "limit": 1, "is_default": true, "model_name": "NVIDIA 5070", "resource_name": "nvidia.com/gpu", "node_label_key": "nvidia.com/model", "node_label_value": "5070"}]'::jsonb,
    '4',
    '16'
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tunnel_connections;
DROP TABLE IF EXISTS user_quotas;
DROP TABLE IF EXISTS team_quotas;
DROP TABLE IF EXISTS resource_configs;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS agent_templates;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS teams;
-- +goose StatementEnd
