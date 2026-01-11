# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is **KubeRDE (Kubernetes Remote Development Environment)** - a platform for securely exposing and managing remote development workspaces behind NAT/firewalls. It consists of:
- **Server**: Public relay with REST API, user/workspace/service management, quota enforcement, audit logging, and agent session management
- **Agent**: Kubernetes pod that connects to server and bridges traffic to internal services (SSH, Jupyter, Coder, File browser)
- **CLI**: User utility for OIDC authentication and SSH tunneling
- **Operator**: Kubernetes operator for declarative agent lifecycle management, PVC provisioning, and TTL-based auto-scaling
- **Web UI**: React-based management console for workspaces, services, users, quotas, templates, and audit logs
- **Database**: PostgreSQL for persistent storage of users, workspaces, services, templates, quotas, and audit logs

The system is built in Go (1.24+) for backend and React+TypeScript for frontend, using WebSockets + Yamux for transport, PostgreSQL for persistence, and OIDC/JWT for authentication.

## Building & Running

### Quick Start with Make

The project includes a comprehensive Makefile for all common operations. Run `make help` to see all available commands.

```bash
# View all available make commands
make help
```

### Building Binaries

```bash
# Build all binaries (server, agent, operator, CLI)
make build

# Or build individually
make build-server    # Outputs: server
make build-agent     # Outputs: agent
make build-operator  # Outputs: kuberde-operator
make build-cli       # Outputs: kuberde-cli
```

### Building and Pushing Docker Images

```bash
# Build all Docker images
make docker-build

# Push all Docker images to registry
make docker-push

# Build and push in one command (default: soloking/*)
make docker-build-push

# Custom registry and version
VERSION=v1.0.0 REGISTRY=myregistry make docker-build-push
```


### Deploying to Kubernetes

```bash
# Deploy all components to Kubernetes (namespace: kuberde)
make deploy

# Or deploy components individually
make deploy-namespace    # Create namespace
make deploy-crd          # Deploy RDEAgent CRD
make deploy-keycloak     # Deploy Keycloak
make deploy-postgresql   # Deploy PostgreSQL
make deploy-server       # Deploy KubeRDE Server
make deploy-operator     # Deploy Operator
make deploy-web          # Deploy Web UI
make deploy-ingress      # Deploy Ingress

# Custom namespace
NAMESPACE=my-namespace make deploy
```

### Managing Kubernetes Deployments

```bash
# Check status of all resources
make status

# View logs
make logs-server       # Server logs (follow)
make logs-operator     # Operator logs (follow)
make logs-postgresql   # PostgreSQL logs (follow)
make logs-agent        # Agent logs (follow)

# Restart services (rolling restart)
make restart           # Restart all services
make restart-server    # Restart server only
make restart-operator  # Restart operator only
make restart-web       # Restart Web UI only
make restart-postgresql # Restart PostgreSQL only

# Scaling
make scale-up          # Scale up deployments
make scale-down        # Scale down deployments
```

### Health Checks

All services (server, operator, web, keycloak) have health check endpoints configured for Kubernetes liveness and readiness probes:

**Server (kuberde-server)**:
- `/healthz` or `/livez` - Liveness probe (basic server health)
- `/readyz` - Readiness probe (checks database and OIDC connectivity)
- Port: 8080

**Operator (kuberde-operator)**:
- `/healthz` or `/livez` - Liveness probe (basic operator health)
- `/readyz` - Readiness probe (checks informer sync and K8s connectivity)
- Port: 8080 (internal only, not exposed via Service)
- Note: Operator uses HTTP health server for probes, following Kubernetes operator best practices

**Web UI (kuberde-web)**:
- `/healthz` - Liveness probe (nginx health)
- `/readyz` - Readiness probe (nginx ready)
- Port: 80

**Keycloak**:
- `/health/live` - Liveness probe
- `/health/ready` - Readiness probe
- Port: 8080
- Note: Requires `--health-enabled=true` flag in Keycloak 23+

**Testing Health Checks**:
```bash
# Port-forward to test locally
kubectl port-forward -n kuberde deployment/kuberde-server 8080:8080

# Test server health
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz

# Test operator health (port 8080 is for internal probes only)
# Operator doesn't expose a Service, so we can only check via pod
OPERATOR_POD=$(kubectl get pods -n kuberde -l app=kuberde-operator -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n kuberde $OPERATOR_POD -- curl -s http://localhost:8080/healthz
kubectl exec -n kuberde $OPERATOR_POD -- curl -s http://localhost:8080/readyz

# Or check probe status indirectly
kubectl get pod -n kuberde -l app=kuberde-operator -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")]}'

# Test web health
kubectl port-forward -n kuberde deployment/kuberde-web 8082:80
curl http://localhost:8082/healthz
curl http://localhost:8082/readyz

# Check pod health status
kubectl get pods -n kuberde
kubectl describe pod -n kuberde <pod-name>
```

### Manual Local Execution (Development)

For development without Docker Compose:

```bash
# Terminal 1: Start PostgreSQL
docker run -d --name kuberde-db \
  -e POSTGRES_DB=kuberde \
  -e POSTGRES_USER=kuberde \
  -e POSTGRES_PASSWORD=kuberde \
  -p 5432:5432 postgres:12

# Terminal 2: Start server
export DATABASE_URL="postgres://kuberde:kuberde@localhost:5432/kuberde?sslmode=disable"
export KEYCLOAK_URL="https://sso.byai.uk/realms/kuberde"
go run ./cmd/server
# Listens: 8080 (HTTP/WebSocket + REST API)

# Terminal 3: Start Web UI dev server
cd web
npm install
npm run dev
# Visit http://localhost:5173

# Terminal 4: Start agent (optional for testing)
export SERVER_URL="ws://127.0.0.1:8080/ws"
export LOCAL_TARGET="127.0.0.1:22"
export AGENT_ID="user-testuser-dev"
export AUTH_CLIENT_ID="frp-agent"
export AUTH_CLIENT_SECRET="your-secret"
export AUTH_TOKEN_URL="https://sso.byai.uk/realms/kuberde/protocol/openid-connect/token"
go run ./cmd/agent
```

### Testing

```bash
# Run all tests
make test

# Run specific tests
make test-build    # Test that builds work
make test-vet      # Run go vet
```

### Cleanup

```bash
# Clean everything (binaries, Docker images, K8s resources)
make clean

# Or clean individually
make clean-binaries  # Remove built binaries
make clean-docker    # Remove Docker images
make clean-k8s       # Delete K8s resources
```

### Configuration Variables

You can customize the build and deployment with these variables:

```bash
VERSION=v1.0.0      # Docker image version (default: latest)
REGISTRY=myregistry # Docker registry (default: soloking)
NAMESPACE=my-ns     # Kubernetes namespace (default: kuberde)
```

Example:
```bash
VERSION=v1.2.3 REGISTRY=mycompany make docker-build-push deploy
```

## Key Architecture Patterns

### Connection Flow

1. **Agent → Server**: Agent establishes persistent WebSocket + Yamux multiplexed session
2. **User → Server**: User connects via CLI or HTTP, sending OIDC JWT in headers/cookies
3. **Server → Agent**: Server opens Yamux streams to signal new user connections
4. **Data Bridge**: Bidirectional `io.Copy` between WebSocket streams and TCP connections

### Authentication Model

- **Agents**: Use OAuth2 Client Credentials flow to obtain JWT tokens from Keycloak
- **Users**:
  - CLI users: OIDC Authorization Code flow (via browser), token stored at `~/.frp/token.json`
  - Web users: OIDC Cookie session set after `/auth/callback`
- **Server**: Validates all incoming JWTs using JWKS fetched from Keycloak at startup

### Authorization

Based on `preferred_username` claim in JWT and Agent ID pattern:
- Agent IDs: `user-{owner}-{name}` (e.g., `user-alice-dev`)
- Only the owner can access their agents (server validates claim matches owner in Agent ID)

### Key Files by Component

**Server** (`cmd/server/main.go` - ~6500 lines):
- Database integration (PostgreSQL with GORM)
- Agent session management (map of Agent ID → Yamux sessions)
- OIDC token validation and JWKS refresh
- REST API endpoints (50+ endpoints):
  - `/api/users/*` - User CRUD, SSH keys, quotas
  - `/api/workspaces/*` - Workspace management
  - `/api/services/*` - Service CRUD with resource config
  - `/api/agent-templates/*` - Template management with import/export
  - `/api/audit-logs/*` - Audit log retrieval
  - `/api/resource-config/*` - System defaults (CPU, memory, GPU, storage)
  - `/api/user-quotas/*` - User quota management
  - `/mgmt/agents/{id}` - Agent stats and TTL tracking
  - `/auth/*` - OIDC authentication flow
- HTTP reverse proxy with `YamuxRoundTripper` for agent communication
- WebSocket upgrade and Yamux session creation
- Keycloak admin client integration for user provisioning
- Audit logging for all CRUD operations

**Agent** (`cmd/agent/main.go`):
- Environment-based configuration (SERVER_URL, LOCAL_TARGET, AUTH_CLIENT_ID, AUTH_CLIENT_SECRET, AUTH_TOKEN_URL)
- Token acquisition via Client Credentials flow with auto-refresh
- Yamux client accepting streams from server
- Local TCP connection bridging to internal services

**CLI** (`cmd/cli/main.go` + `cmd/cli/cmd/`):
- `login` command: Opens browser for OIDC auth, saves token locally at `~/.frp/token.json`
- `connect` command: Reads token, establishes WebSocket to server, acts as SSH ProxyCommand

**Operator** (`cmd/operator/` - ~2500 lines):
- Watches `RDEAgent` CRD (group: `kuberde.io`, version: `v1beta1`)
- Creates/updates/deletes Kubernetes Deployments based on CR state
- PVC provisioning and binding for workspace storage
- Injects credentials from Secrets into agent pods
- Implements TTL-based idle scaledown via `/mgmt/agents/{id}` polling
- Status tracking (online/offline/error states)
- Security context management (UID/GID, volume mounts)
- Resource limit enforcement (CPU, memory, GPU)

**Web UI** (`web/` - React + TypeScript):
- **Pages** (15 total):
  - Dashboard, Workspaces, WorkspaceCreate, WorkspaceDetail
  - ServiceCreate, ServiceDetail, ServiceEdit
  - UserManagement, UserEdit, AdminWorkspaces
  - AuditLogs, AgentTemplates, ResourceManagement, Help, Login
- **Components**: Header, Sidebar, ProtectedRoute
- **Services**: API client (`api.ts`) with full CRUD for all entities
- **Contexts**: AuthContext for authentication state management
- **Tech stack**: React 18+, TypeScript, Vite, Tailwind CSS, React Router

**Database** (`pkg/models/models.go`, `pkg/db/`, `pkg/repositories/`):
- **Models**: User, Workspace, Service, AgentTemplate, AuditLog, ResourceConfig, UserQuota, SSHKey
- **Repositories**: CRUD operations for all models
- **Migrations**: GORM auto-migrations or Goose SQL migrations in `deploy/migrations/`

## Configuration

### Environment Variables

**Server**:
- `DATABASE_URL`: PostgreSQL connection string (required, e.g., `postgres://user:pass@host:5432/dbname?sslmode=disable`)
- `KEYCLOAK_URL`: Keycloak realm URL (default inferred from requests)
- `KEYCLOAK_ADMIN_URL`: Keycloak admin API URL for user provisioning
- `KEYCLOAK_ADMIN_CLIENT_ID`, `KEYCLOAK_ADMIN_CLIENT_SECRET`: Admin credentials for user sync
- **Domain Configuration**:
  - `KUBERDE_PUBLIC_URL`: Public service URL (default: `https://frp.byai.uk`)
    - Used for OAuth redirects, cookie domain, and frontend URLs
    - Example: `https://frp.byai.uk`, `https://www.kuberde.com`
  - `KUBERDE_AGENT_DOMAIN`: Agent wildcard domain (default: derived from KUBERDE_PUBLIC_URL)
    - Used for agent subdomain routing and cookie sharing
    - Example: `frp.byai.uk`, `kuberde.com`, `agent.kuberde.com`
    - Agents are accessible at `{agent-id}.{KUBERDE_AGENT_DOMAIN}`
- **Agent Connection Configuration**:
  - `KUBERDE_AGENT_SERVER_URL`: WebSocket URL that agents should connect to (default: `ws://kuberde-server:8080/ws`)
    - Internal cluster address for agent-to-server communication
    - Used when creating RDEAgent custom resources
  - `KUBERDE_AGENT_AUTH_SECRET`: Secret name containing agent authentication credentials (default: `kuberde-agents-auth`)
    - Name of the Kubernetes secret with agent client credentials
    - Referenced by RDEAgent custom resources
  - `KUBERDE_NAMESPACE`: Kubernetes namespace for creating PVCs and RDEAgents (default: auto-detected from pod's namespace, fallback to `kuberde`)
    - Server auto-detects the namespace from `/var/run/secrets/kubernetes.io/serviceaccount/namespace`
    - Can be explicitly set via environment variable for non-standard deployments
- Listens on:
  - `:8080` - All HTTP/WebSocket traffic (agents, REST API, management API)
  - `:8080/ws` - WebSocket endpoint for agents
  - `:8080/api/*` - REST API for Web UI
  - `:8080/mgmt/*` - Management API for operator

**Agent**:
- `SERVER_URL`: WebSocket URL to server (default: `ws://127.0.0.1:8080/ws`)
- `LOCAL_TARGET`: Target service address (default: `127.0.0.1:22`)
- `AGENT_ID`: Unique agent identifier (format: `user-{owner}-{name}`)
- `AUTH_CLIENT_ID`: Keycloak client ID for authentication
- `AUTH_CLIENT_SECRET`: Keycloak client secret
- `AUTH_TOKEN_URL`: Keycloak token endpoint

**CLI**:
- Token stored at `~/.frp/token.json` (created during `login`)

**Operator**:
- `SERVER_BASE_URL`: KubeRDE server API endpoint for agent status polling
- `POLL_INTERVAL`: Agent status polling interval (default: 30s)

**Web UI** (build-time variables):
- `VITE_API_BASE_URL`: Backend API URL (default: same origin)
- `VITE_KEYCLOAK_URL`: Keycloak realm URL for authentication

### Kubernetes Deployment

See `/deploy/k8s/` for Kubernetes manifests:
- `00-namespace.yaml`: Creates `kuberde` namespace
- `01-crd.yaml`: RDEAgent CustomResourceDefinition (v1beta1)
- `02-keycloak*.yaml`: Keycloak deployment and service
- `02-web.yaml`: Web UI deployment (Nginx serving React app)
- `03-server.yaml`: KubeRDE Server deployment with database config
- `03-agent.yaml`: Example agent deployment
- `04-operator.yaml`: Operator deployment with RBAC
- `05-ingress.yaml`: Ingress configuration for domain routing
- `06-postgresql.yaml`: PostgreSQL database deployment
- `all-in-one.yaml`: Combined manifest for easy deployment

Additional configs:
- `/deploy/keycloak/realm-export.json`: Keycloak realm configuration
- `/deploy/migrations/`: Database migration scripts (if using Goose)

## Important Implementation Details

### WebSocket Connection Pattern

Server and Agent use a custom `wsConn` adapter that wraps `*websocket.Conn` to provide an `io.Reader` interface compatible with `yamux.Server()` and `yamux.Client()`.

### Yamux Stream Lifecycle

1. Agent initiates connection with authentication
2. Server creates `yamux.Server` on WebSocket
3. User connects to server
4. Server calls `session.Open()` to create a Yamux stream
5. Agent receives stream via `session.Accept()`
6. Both sides bridge their respective connections (user ↔ stream, stream ↔ local service)

### Agent ID Pattern in Kubernetes

When `RDEAgent.spec.owner` is set, the Operator generates Agent ID as `user-{owner}-{name}`, where `{name}` is derived from the CR name. This enables multi-tenant authorization.

## Testing

See `cmd/mock/` for a simple TCP echo server useful for testing without real services. Dockerfiles use multi-stage builds for minimal image sizes.

## Common Development Tasks

### Backend (Go)
- **Add authentication to new endpoint**: Extract JWT from Authorization header or cookie, validate with `oidcVerifier.Verify()`, check claims
- **Add new REST API endpoint**:
  1. Define handler in `cmd/server/main.go`
  2. Add route with `http.HandleFunc()` or router
  3. Implement CORS middleware for Web UI access
  4. Add authentication middleware
  5. Test with curl or Web UI
- **Add new database model**:
  1. Define struct in `pkg/models/models.go` with GORM tags
  2. Add repository methods in `pkg/db/` or `pkg/repositories/`
  3. Run migrations (auto-migrate or add Goose migration)
  4. Update API handlers to use new model
- **Add new agent configuration**: Update environment variable parsing in agent's `init()` function
- **Modify CRD schema**:
  1. Update `/deploy/k8s/01-crd.yaml` CRD definition
  2. Update struct tags in operator code (`cmd/operator/`)
  3. Update reconciliation logic to handle new fields
- **Test locally without Keycloak**: Use unsigned test tokens (useful for development, not production)
- **Add audit logging**: Call `createAuditLog()` with user ID, action, resource type, resource ID, old/new data

### Frontend (React)
- **Add new page**:
  1. Create component in `web/pages/`
  2. Add route in `web/App.tsx`
  3. Add navigation link in `web/components/Sidebar.tsx`
  4. Implement with TypeScript and Tailwind CSS
- **Add new API call**:
  1. Add function to `web/services/api.ts`
  2. Use in component with `useState` and `useEffect`
  3. Handle loading, success, and error states
- **Add authentication check**: Wrap route with `<ProtectedRoute>` component and specify required role
- **Update UI styling**: Use Tailwind CSS utility classes, follow existing patterns for consistency

### Database
- **Run migrations locally**:
  ```bash
  export DATABASE_URL="postgres://kuberde:kuberde@localhost:5432/kuberde?sslmode=disable"
  go run cmd/migrate/main.go up  # If migration tool exists
  # Or rely on GORM auto-migrate on server startup
  ```
- **Add database migration**: Create SQL file in `deploy/migrations/` with timestamp prefix (if using Goose)
- **Query database directly**:
  ```bash
  psql postgres://kuberde:kuberde@localhost:5432/kuberde
  ```

### Domain Configuration

**Configure custom domains**:
The system supports flexible domain configuration for different deployment scenarios. Set these environment variables in the server deployment:

1. **Single domain setup** (default):
   ```bash
   export KUBERDE_PUBLIC_URL=https://frp.byai.uk
   export KUBERDE_AGENT_DOMAIN=frp.byai.uk
   ```
   - Service: `https://frp.byai.uk`
   - Agents: `*.frp.byai.uk`
   - Cookie domain: `.frp.byai.uk` (shared across all subdomains)

2. **Separate www subdomain**:
   ```bash
   export KUBERDE_PUBLIC_URL=https://www.kuberde.com
   export KUBERDE_AGENT_DOMAIN=kuberde.com
   ```
   - Service: `https://www.kuberde.com`
   - Agents: `*.kuberde.com`
   - Cookie domain: `.kuberde.com` (shared between www and agent subdomains)

### Testing
- **Test REST API**: Use curl, Postman, or Web UI dev tools
- **Test WebSocket**: Use CLI or agent connection
- **Test Web UI**: Run `npm run dev` and access http://localhost:5173
- **Test operator**: Apply RDEAgent CR and watch operator logs
- **Test agent subdomain**: Access `http://{agent-id}.{KUBERDE_AGENT_DOMAIN}` in browser after authentication
