# KubeRDE Architecture Diagrams

This directory contains architecture diagrams for the KubeRDE (Kubernetes Remote Development Environment) project, generated using Python Diagrams library.

## Diagrams

### 1. Main Architecture Diagram
**File:** `kuberde_architecture.png`

This diagram shows the complete KubeRDE system architecture including:

- **Users**: Web users and CLI users accessing the system
- **Frontend Layer**: React-based Web UI
- **Public Access**: Ingress controller for routing traffic
- **Authentication**: Keycloak OIDC provider for user authentication
- **KubeRDE Server**: Core server components including:
  - REST API (`/api/*`) for Web UI
  - WebSocket endpoint (`/ws`) for agent connections
  - Management API (`/mgmt/*`) for operator integration
  - HTTP Proxy for agent traffic routing
- **Database**: PostgreSQL for persistent storage
- **Kubernetes Cluster**:
  - Operator watching and reconciling RDEAgent CRDs
  - Agent deployments with workload containers
  - PVC/PV for workspace persistence
- **CLI Client**: Command-line tool for SSH tunneling

**Key Flows:**
- User authentication through Keycloak OIDC
- Web UI to Server REST API communication
- CLI to Server WebSocket connections
- Server to Agent Yamux multiplexed streams
- Operator control loop for CRD reconciliation

### 2. Data Flow Diagram
**File:** `kuberde_data_flow.png`

This diagram illustrates the end-to-end data flow from user request to service response:

**Authentication Flow:**
1. User initiates login request
2. Keycloak handles OIDC authentication
3. JWT token issued to user

**Connection Flow:**
4. Server validates JWT token
5. Yamux session established for multiplexing
6. Agent pod receives connection
7. Traffic bridged to development service (SSH/Jupyter/Coder/Files)

**Data Flow:**
- User request → Server → Yamux Stream → Agent → Service
- Service response ← Agent ← Yamux Stream ← Server ← User

### 3. Operator Lifecycle Diagram
**File:** `kuberde_operator_lifecycle.png`

This diagram shows the complete lifecycle of an agent from creation to operation:

**User Action:**
- User creates service via Web UI
- API call to server (`POST /api/services`)

**Server Processing:**
- Service record written to database
- RDEAgent Custom Resource created in Kubernetes

**Operator Reconciliation:**
- Operator watches for RDEAgent CR events
- Creates Deployment for agent
- Creates PVC for workspace storage
- Polls agent stats for TTL-based auto-scaling

**Agent Connection:**
- Agent pod starts and connects to server via WebSocket
- Yamux session established
- Agent becomes ready and starts serving traffic

## Regenerating Diagrams

To regenerate the diagrams after making changes:

```bash
# Install dependencies (first time only)
pip3 install diagrams
brew install graphviz  # On macOS

# Generate diagrams
python3 docs/architecture.py
```

The script will generate three PNG files in the `docs/` directory.

## Architecture Highlights

### Multi-Tenancy
- Agent IDs follow pattern: `user-{owner}-{name}`
- Authorization based on JWT `preferred_username` claim
- Users can only access their own agents

### Transport
- WebSocket + Yamux for multiplexed connections
- Single WebSocket per agent supports multiple concurrent user sessions
- Bidirectional `io.Copy` for data bridging

### Authentication
- Agents: OAuth2 Client Credentials flow
- Users: OIDC Authorization Code flow (CLI) or Cookie session (Web)
- Server: JWKS validation for all JWT tokens

### Auto-Scaling
- Operator polls agent stats via management API
- TTL-based idle detection
- Automatic scale-down of inactive agents
- Preserves workspace data in PVCs

## Technology Stack

- **Backend**: Go 1.24+
- **Frontend**: React 18+ with TypeScript
- **Database**: PostgreSQL with GORM
- **Platform**: Kubernetes
- **Authentication**: Keycloak (OIDC)
- **Transport**: WebSocket + Yamux
- **Diagrams**: Python Diagrams library + Graphviz

## Related Documentation

- [ARCHITECTURE.md](./ARCHITECTURE.md) - Detailed architecture documentation
- [QUICK_START.md](./QUICK_START.md) - Quick start guide
- [design.md](./design.md) - Design decisions and rationale
- [CLAUDE.md](../CLAUDE.md) - Project instructions for development

## License

This project is part of KubeRDE. See the main repository for license information.
