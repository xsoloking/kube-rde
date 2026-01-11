# KubeRDE System Architecture

**Version**: 1.0
**Last Updated**: 2025-12-08
**Status**: Production

---

## Table of Contents

1. [System Architecture Overview](#1-system-architecture-overview)
2. [Core Components](#2-core-components)
3. [Key Architectural Decisions](#3-key-architectural-decisions)
4. [Current Features](#4-current-features)
5. [Future Roadmap](#5-future-roadmap)
6. [Technical Design Patterns](#6-technical-design-patterns)

---

## 1. System Architecture Overview

KubeRDE is a **Fast Reverse Proxy** system designed to securely expose services behind NAT/firewalls to the public internet. The system combines Kubernetes-native management with enterprise-grade authentication and multi-tenant isolation.

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Public Internet                          │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
                    ┌───────────────┐
                    │   Ingress     │
                    │  Controller   │
                    └───────┬───────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
    ┌────────┐        ┌──────────┐       ┌──────────┐
    │Keycloak│◄───────│  KubeRDE │       │   User   │
    │ (OIDC) │        │  Server  │◄──────│  Client  │
    └────────┘        └─────┬────┘       └──────────┘
                            │
                            │ WebSocket + Yamux
                            │
                    ┌───────▼───────┐
                    │ KubeRDE Agent │
                    │   (Sidecar)   │
                    └───────┬───────┘
                            │
                            ▼
                    ┌───────────────┐
                    │   Workload    │
                    │  (SSH/HTTP)   │
                    └───────────────┘
```

### Component Responsibilities

| Component | Role | Technology |
|-----------|------|------------|
| **KubeRDE Server** | Public relay, authentication gateway | Go, WebSocket, Yamux, OIDC |
| **KubeRDE Agent** | NAT traversal client, traffic forwarder | Go, WebSocket, Yamux |
| **KubeRDE CLI** | User authentication & connection tool | Go, Cobra, OIDC |
| **KubeRDE Operator** | Kubernetes resource orchestrator | Go, client-go, dynamic client |
| **Keycloak** | Identity provider (OIDC/OAuth2) | Java, PostgreSQL |

---

## 2. Core Components

### 2.1 KubeRDE Server

**Location**: `cmd/server/main.go`

**Responsibilities**:
- Accept WebSocket connections from agents and users
- Validate OIDC tokens (JWT) via JWKS
- Manage Yamux sessions for multiplexed streams
- Provide HTTP reverse proxy for web workloads
- Track agent activity for TTL management
- Implement multi-tenant authorization

**Key APIs**:
- `GET /ws` - WebSocket upgrade endpoint (agents)
- `POST /auth/login` - OIDC authentication initiation
- `GET /auth/callback` - OIDC callback handler
- `GET /mgmt/agents/{id}` - Agent statistics API
- `POST /api/agents/{id}/scale-up` - Auto-scale trigger

**Authentication Flow**:
```
Agent: OAuth2 Client Credentials → Access Token → WebSocket Header
User CLI: Authorization Code Flow → Token File → WebSocket Header
Web User: Authorization Code Flow → Cookie Session → HTTP Header
```

**Authorization Model**:
- Agent ID format: `user-{owner}-{name}`
- Only tokens with matching `preferred_username` can access agents
- Admins (future) can access all agents via realm roles

### 2.2 KubeRDE Agent

**Location**: `cmd/agent/main.go`

**Responsibilities**:
- Establish persistent WebSocket connection to server
- Authenticate via OAuth2 Client Credentials
- Accept Yamux streams from server
- Bridge streams to local target services (SSH, HTTP, etc.)
- Automatically refresh authentication tokens

**Configuration** (Environment Variables):
- `SERVER_URL`: WebSocket endpoint (e.g., `ws://frp-server:80/ws`)
- `LOCAL_TARGET`: Internal service address (e.g., `localhost:22`)
- `AGENT_ID`: Unique identifier (e.g., `user-alice-dev`)
- `AUTH_CLIENT_ID`, `AUTH_CLIENT_SECRET`: OAuth2 credentials
- `AUTH_TOKEN_URL`: Keycloak token endpoint

**Network Flow**:
```
User → Server:2022 → Server:Yamux → Agent:Yamux → Agent:LocalTarget
```

### 2.3 KubeRDE CLI

**Location**: `cmd/cli/`

**Commands**:

1. **login** - OIDC browser-based authentication
   ```bash
   kuberde-cli login --server-url http://frp.byai.uk
   ```
   - Opens browser to Keycloak
   - Saves token to `~/.frp/token.json`

2. **connect** - Establish tunneled connection
   ```bash
   kuberde-cli connect --agent-id user-alice-dev
   ```
   - Reads saved token
   - Creates WebSocket tunnel
   - Acts as SSH ProxyCommand

**SSH Integration**:
```ssh
# ~/.ssh/config
Host *.frp
    ProxyCommand kuberde-cli connect --agent-id %h
```

### 2.4 KubeRDE Operator

**Location**: `cmd/operator/main.go`

**CRD**: `RDEAgent` (group: `kuberde.io`, version: `v1`)

**Reconciliation Loop**:
1. Watch `RDEAgent` custom resources
2. Create/update Deployment with agent + workload containers
3. Inject authentication secrets and SSH configurations
4. Monitor agent activity via server API
5. Scale deployments based on TTL (idle timeout)

**Generated Deployment Structure**:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-{owner}-{name}
spec:
  replicas: 1  # Set to 0 when idle TTL exceeded
  template:
    spec:
      containers:
      - name: frp-agent
        image: ghcr.io/yourorg/frp-agent:latest
        env:
        - name: AGENT_ID
          value: user-{owner}-{name}
        - name: AUTH_CLIENT_ID
          valueFrom:
            secretKeyRef:
              name: {authSecret}
              key: client-id
      - name: workload
        image: {spec.workloadImage}
        env:
        - name: USER_NAME
          value: {spec.owner}
        - name: PUBLIC_KEY
          value: {spec.sshPublicKeys[0]}
```

**TTL Scaledown Logic**:
- Every 60 seconds, query `/mgmt/agents/{id}` for each CR
- If `lastActivity` > `spec.ttl`, set `replicas: 0`
- If `replicas == 0` and user attempts connection, trigger scale-up

### 2.5 Keycloak (Identity Provider)

**Realm**: `frp`

**Clients**:
- `kuberde-cli` (Public) - For browser-based user authentication
- `frp-agent` (Confidential) - For service-to-service authentication

**Users**:
- `testuser` / `password` (standard user)
- `admin` / `admin` (administrator)

**Token Claims**:
- `preferred_username` - Used for agent ownership matching
- `realm_access.roles` - For future role-based access control

---

## 3. Key Architectural Decisions

This section documents critical design choices, their rationale, and trade-offs.

### ADR-001: Use Deployments (Not Pods) for Agents

**Decision**: Manage agents via Kubernetes Deployments instead of bare Pods

**Rationale**:
- **Auto-recovery**: Crashed pods restart automatically
- **Replica management**: Easy scaling via `replicas` field
- **Rollout support**: Zero-downtime updates with rolling strategy
- **TTL compatibility**: Scale to 0 by setting `replicas: 0`

**Trade-offs**:
- Additional ReplicaSet overhead (minimal)
- More complex than bare pods

**Status**: ✅ Accepted

### ADR-002: Kubernetes Operator Pattern

**Decision**: Use custom Operator instead of Helm/Kustomize

**Rationale**:
- **Declarative management**: Users define desired state, operator handles reality
- **Multi-tenancy**: Built-in owner isolation via CRD spec
- **Dynamic behavior**: Supports TTL scaledown, auto-scaling, status updates
- **Kubernetes-native**: Integrates with RBAC, events, and API server

**Alternatives Considered**:
- Helm Charts (less automation, no runtime logic)
- Kustomize (static templating only)
- Manual YAML (not maintainable)

**Trade-offs**:
- Higher implementation complexity
- Requires Go and client-go knowledge

**Status**: ✅ Accepted

### ADR-003: TTL-Based Idle Scaledown

**Decision**: Automatically scale idle agents to zero replicas

**Rationale**:
- **Cost optimization**: Reduces resource usage by 90%+ for idle agents
- **User transparency**: Users don't need to manually shutdown
- **Configurable**: Each agent sets own TTL (e.g., `8h`, `30m`)

**Implementation**:
- Operator polls server `/mgmt/agents/{id}` every 60 seconds
- Compares `lastActivity` timestamp to TTL
- Sets `replicas: 0` when threshold exceeded

**Trade-offs**:
- Cold start delay when agent needed again (mitigated by auto-scale-up)
- Requires server to expose management API

**Status**: ✅ Implemented

### ADR-004: Multi-Layer OIDC Authentication

**Decision**: Different auth flows for different client types

**Authentication Matrix**:

| Client Type | Flow | Token Storage |
|-------------|------|---------------|
| CLI Users | Authorization Code | `~/.frp/token.json` |
| Web Users | Authorization Code | Cookie (`frp_session`) |
| Agents | Client Credentials | Memory (auto-refresh) |

**Rationale**:
- Each client uses most appropriate OAuth2 flow
- CLI: Interactive browser flow with local token storage
- Web: Cookie-based session for browser convenience
- Agents: Machine-to-machine credentials (no user interaction)

**Security Features**:
- Short-lived tokens (5-15 minutes)
- Refresh token support (planned)
- JWKS validation (offline signature verification)

**Trade-offs**:
- Complexity of supporting three flows
- Requires Keycloak configuration for each client type

**Status**: ✅ Accepted (with planned improvements)

### ADR-005: Yamux for Stream Multiplexing

**Decision**: Use Hashicorp Yamux over single WebSocket connection

**Rationale**:
- **Efficiency**: Multiple user connections over one agent WebSocket
- **Simplicity**: Lightweight library (vs gRPC, QUIC)
- **WebSocket compatible**: Works with standard HTTP/HTTPS infrastructure
- **Mature**: Battle-tested in Consul, Nomad

**Alternatives Considered**:
- gRPC (too heavy, complex for simple forwarding)
- QUIC (UDP-based, poor WebSocket support)
- Custom multiplexing (reinventing the wheel)

**Implementation Details**:
- Custom `wsConn` adapter wraps `websocket.Conn`
- Implements `io.Reader/Writer` for Yamux compatibility
- Server: `session.Open()` creates streams
- Agent: `session.Accept()` receives streams

**Trade-offs**:
- No built-in serialization (unlike gRPC)
- Manual error handling for stream lifecycle

**Status**: ✅ Accepted

### ADR-006: CRD Versioning Strategy

**Decision**: Plan for multi-version CRD evolution (v1 → v1beta1 → v2)

**Current**: `v1` with basic fields (owner, image, env, ttl)

**Planned Evolution**:

**v1beta1** (Next 3 months):
- Add `resources` (CPU, memory, GPU)
- Add `volumes` and `volumeMounts`
- Add `nodeSelector`, `tolerations`
- Conversion webhook for v1 → v1beta1

**v2** (Future):
- Full `PodSpec` support
- Sidecar injection mode
- Advanced scheduling policies

**Rationale**:
- Allows backward-compatible feature additions
- Users can migrate gradually
- Avoids breaking existing CRs

**Status**: 🔄 Planned

### ADR-007: Auto Scale-Up Mechanism

**Decision**: Server triggers operator webhook when offline agent is requested

**Flow**:
```
1. User requests connection to agent-A
2. Server checks: agent-A offline, replicas=0
3. Server calls: POST /api/agents/agent-A/scale-up (operator webhook)
4. Operator sets replicas=1
5. Pod starts, agent connects
6. Server retries connection (or user retries)
```

**Rationale**:
- Improves UX after idle scaledown
- Reduces cold start perception
- Server already tracks agent state

**Trade-offs**:
- Adds server → operator communication path
- Requires authentication between components

**Status**: ✅ Implemented (Phase 3)

### ADR-008: Session Security (Planned)

**Current Issue**: Cookies store raw JWT tokens

**Planned Improvement**:
- Store only session ID in cookie
- Keep tokens in server-side session store
- Support `Secure`, `HttpOnly`, `SameSite` flags
- Multi-instance: Use Redis for shared sessions

**Status**: 🔄 Planned

### ADR-009: Agent Token Auto-Refresh (Planned)

**Current Issue**: Agents don't refresh tokens, must restart after 5-15 minutes

**Planned Improvement**:
- Use `oauth2.TokenSource` for automatic refresh
- Refresh at 80% of token TTL
- Exponential backoff on refresh failures

**Status**: 🔄 Planned

### ADR-010: Secret Management for SSH Credentials

**Current Issue**: SSH passwords in plain environment variables

**Planned Improvement**:
- Store credentials in Kubernetes Secrets
- Reference via `valueFrom.secretKeyRef`
- Operator manages secret creation/rotation
- Future: Integrate Sealed Secrets or Vault

**Status**: 🔄 Planned

### ADR-011: Management API Authorization

**Current Issue**: `/mgmt/agents/{id}` endpoint has no access control

**Planned Improvement**:
- Extract realm roles from OIDC token
- `frp-admin` role: Full access
- `frp-user` role: Own agents only
- System accounts for operator

**Status**: 🔄 Planned

---

## 4. Current Features

### 4.1 Core KubeRDE Functionality

#### Multi-Protocol Support
- **SSH Tunneling**: Via CLI ProxyCommand integration
- **HTTP/HTTPS Forwarding**: Via reverse proxy with subdomain routing
- **WebSocket Tunneling**: Full bidirectional stream support

#### Connection Management
- **Persistent Connections**: Agents maintain long-lived WebSocket sessions
- **Multiplexed Streams**: Yamux enables multiple concurrent user connections
- **Auto-Reconnect**: Agents reconnect on temporary network failures

#### Security & Authentication
- **OIDC Integration**: Keycloak-based authentication for all components
- **Multi-Tenant Isolation**: Owner-based authorization prevents cross-user access
- **JWT Validation**: Server validates tokens via JWKS (no DB lookup needed)
- **Encrypted Transport**: TLS-terminated at ingress, WebSocket over HTTPS

### 4.2 Kubernetes Operator Features

#### Declarative Management
- **Custom Resource**: `RDEAgent` CRD defines desired agent state
- **Automatic Deployment**: Operator creates Deployment + Service + ConfigMap
- **Sidecar Injection**: Agent container runs alongside user workload

#### Resource Lifecycle
- **Creation**: CRD → Deployment with owner references
- **Updates**: Changes to CRD trigger deployment updates (Phase 4 fix)
- **Deletion**: Cascade delete via owner references
- **Status Tracking**: CR status reflects deployment state

#### TTL-Based Scaledown
- **Idle Detection**: Monitors agent activity via server API
- **Automatic Scaledown**: Sets replicas to 0 after TTL expires
- **Manual Override**: Users can disable TTL via `spec.ttl: "0"`
- **Grace Period**: Configurable per-agent (e.g., `8h`, `30m`, `2h`)

#### Auto Scale-Up (Phase 3)
- **On-Demand Activation**: User connection triggers scale-up
- **Webhook Integration**: Server calls operator API
- **Fast Startup**: Pre-configured deployments start quickly
- **Status Feedback**: Server can query scale-up progress

### 4.3 User Experience Features

#### CLI Tools
- **One-Command Login**: `kuberde-cli login` opens browser for SSO
- **Persistent Sessions**: Token saved locally, no repeated login
- **SSH Integration**: Works as SSH ProxyCommand (`ssh user@agent.frp`)
- **Cross-Platform**: Binaries for Linux, macOS, Windows

#### Web Access
- **Subdomain Routing**: Each agent gets `{agent-id}.frp.byai.uk`
- **Cookie Sessions**: Browser remembers authentication
- **Ingress Integration**: Standard Kubernetes ingress for SSL/DNS

#### Developer Experience
- **Simple CRD**: Minimal YAML to deploy agents
- **Status Visibility**: `kubectl get rdeagent` shows state
- **Event Logging**: Kubernetes events track operator actions
- **Error Messages**: Clear feedback for misconfigurations

### 4.4 Operations & Monitoring

#### Observability
- **Agent Statistics**: `/mgmt/agents/{id}` API provides:
  - Online/offline status
  - Last activity timestamp
  - Total bytes transferred
  - Active connections count
- **Operator Logs**: Structured logging with reconcile events
- **Server Logs**: HTTP access logs + WebSocket events

#### Resource Management
- **Resource Requests/Limits**: Configurable in CRD (planned for v1beta1)
- **Node Scheduling**: Support for `nodeSelector`, `tolerations` (planned)
- **Volume Support**: PVC mounting for persistent workloads (planned)

#### High Availability
- **Agent Auto-Recovery**: Kubernetes restarts failed pods
- **Connection Resilience**: Yamux reconnects broken streams
- **Graceful Shutdown**: Agents drain connections before terminating

---

## 5. Future Roadmap

### Phase 1: Near-Term Improvements (Next 3 Months)

#### Security Enhancements
- **Session Store**: Replace cookie JWT with server-side sessions
- **Secret Management**: Move credentials to Kubernetes Secrets
- **Token Refresh**: Auto-refresh for agents and CLI
- **RBAC for Management API**: Role-based access to `/mgmt/*`

#### Operator Enhancements
- **CRD Status Updates**: Real-time status in custom resource
- **Deployment Updates**: Fix issue where spec changes don't update deployments
- **Event Publishing**: Emit Kubernetes events for user actions
- **Error Recovery**: Better handling of partial failures

#### Monitoring & Debugging
- **Prometheus Metrics**: Export agent/connection metrics
- **Health Checks**: Standardized `/health` and `/ready` endpoints
- **Debug Endpoints**: Stream logs, connection diagnostics

### Phase 2: Advanced Features (3-6 Months)

#### CRD v1beta1
- **Resource Specifications**: CPU, memory, GPU requests/limits
- **Volume Support**: PVC mounting for stateful workloads
- **Advanced Scheduling**: Node selectors, affinity, tolerations
- **Container Customization**: Full control over container spec

#### Performance Optimization
- **Connection Pooling**: Reuse agent connections
- **Compression**: Optional stream compression for slow networks
- **Rate Limiting**: Per-user/agent bandwidth limits
- **Caching**: HTTP response caching for static content

#### Multi-Cluster Support
- **Federation**: One server, multiple Kubernetes clusters
- **Cross-Region**: Agents in different geographic regions
- **Cluster Discovery**: Automatic agent registration

### Phase 3: Enterprise Features (6-12 Months)

#### CRD v2
- **Full Pod Spec**: Complete Kubernetes pod configuration
- **Init Containers**: Pre-startup setup tasks
- **Lifecycle Hooks**: Post-start, pre-stop handlers
- **Security Context**: RunAsUser, capabilities, SELinux

#### Advanced Security
- **mTLS**: Mutual TLS between server and agents
- **Network Policies**: K8s network policies for isolation
- **Audit Logging**: Compliance-ready audit trail
- **Secrets Integration**: HashiCorp Vault, AWS Secrets Manager

#### Scalability
- **Server Clustering**: Multiple server instances with load balancing
- **Agent Sharding**: Distribute agents across server instances
- **Database Backend**: Persistent state storage (vs in-memory)
- **CDN Integration**: Edge caching for web workloads

#### Observability
- **OpenTelemetry**: Distributed tracing
- **Grafana Dashboards**: Pre-built monitoring dashboards
- **Alerting**: Prometheus AlertManager integration
- **Log Aggregation**: Elasticsearch/Loki integration

### Phase 4: Platform Features (12+ Months)

#### Developer Tools
- **CI/CD Integration**: GitHub Actions, GitLab CI plugins
- **Terraform Provider**: IaC for FRP resources
- **API Gateway**: RESTful API for programmatic management
- **Web Console**: GUI for agent management

#### Advanced Workloads
- **GPU Support**: NVIDIA GPU sharing/scheduling
- **Jupyter Notebooks**: Pre-configured data science environments
- **VSCode Tunnels**: Integrated IDE access
- **Database Tunnels**: Secure access to internal databases

#### Ecosystem Integration
- **Operator Hub**: Publish to OperatorHub.io
- **Artifact Hub**: Helm charts distribution
- **Cloud Marketplaces**: AWS, GCP, Azure marketplace listings
- **Partner Integrations**: Third-party auth providers, monitoring tools

---

## 6. Technical Design Patterns

### 6.1 Connection Flow

**Agent Registration**:
```
1. Agent obtains OAuth2 token from Keycloak
2. Agent connects to ws://server:8080/ws with token in header
3. Server validates token via JWKS
4. Server creates Yamux session over WebSocket
5. Agent enters accept loop for new streams
```

**User Connection (SSH)**:
```
1. User runs: ssh -o ProxyCommand="kuberde-cli connect --agent-id X" user@host
2. CLI reads token from ~/.frp/token.json
3. CLI connects to ws://server:8080/ws with token
4. Server validates token, checks user owns agent X
5. Server opens Yamux stream to agent X
6. Bidirectional copy: SSH ↔ WebSocket ↔ Yamux ↔ Agent ↔ LocalSSH
```

**User Connection (HTTP)**:
```
1. Browser requests https://agent-X.frp.byai.uk/
2. Ingress routes to frp-server Service
3. Server checks frp_session cookie
4. Server validates OIDC token from cookie
5. Server extracts agent ID from Host header
6. Server opens Yamux stream to agent X
7. Reverse proxy forwards HTTP request through stream
8. Agent forwards to localhost:80 (or configured target)
9. Response flows back through stream to browser
```

### 6.2 Yamux Stream Lifecycle

**Server-Side (New User Connection)**:
```go
// In handler for new user connection
stream, err := yamuxSession.Open()
if err != nil {
    return fmt.Errorf("agent offline: %w", err)
}
defer stream.Close()

// Bidirectional copy
go io.Copy(stream, userConn)
io.Copy(userConn, stream)
```

**Agent-Side (Accept Loop)**:
```go
for {
    stream, err := yamuxSession.Accept()
    if err != nil {
        log.Error("session closed", "error", err)
        return
    }

    go handleStream(stream, localTarget)
}

func handleStream(stream net.Conn, target string) {
    defer stream.Close()

    localConn, err := net.Dial("tcp", target)
    if err != nil {
        log.Error("local dial failed", "target", target)
        return
    }
    defer localConn.Close()

    go io.Copy(stream, localConn)
    io.Copy(localConn, stream)
}
```

### 6.3 Operator Reconciliation Loop

**Simplified Reconcile Logic**:
```go
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch RDEAgent CR
    var agent frpv1.RDEAgent
    if err := r.Get(ctx, req.NamespacedName, &agent); err != nil {
        if apierrors.IsNotFound(err) {
            // CR deleted, cleanup done via owner references
            return ctrl.Result{}, nil
        }
        return ctrl.Result{}, err
    }

    // 2. Compute desired deployment
    desired := r.buildDeployment(&agent)

    // 3. Check if deployment exists
    var existing appsv1.Deployment
    err := r.Get(ctx, types.NamespacedName{
        Namespace: agent.Namespace,
        Name: desired.Name,
    }, &existing)

    if apierrors.IsNotFound(err) {
        // 4a. Create new deployment
        if err := r.Create(ctx, desired); err != nil {
            return ctrl.Result{}, err
        }
        r.EventRecorder.Event(&agent, "Normal", "Created", "Created deployment")
        return ctrl.Result{}, nil
    } else if err != nil {
        return ctrl.Result{}, err
    }

    // 4b. Update existing deployment if spec changed
    if !reflect.DeepEqual(existing.Spec, desired.Spec) {
        existing.Spec = desired.Spec
        if err := r.Update(ctx, &existing); err != nil {
            return ctrl.Result{}, err
        }
        r.EventRecorder.Event(&agent, "Normal", "Updated", "Updated deployment")
    }

    // 5. Update CR status
    agent.Status.Active = existing.Status.Replicas > 0
    if err := r.Status().Update(ctx, &agent); err != nil {
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}
```

### 6.4 OIDC Token Validation

**Server Token Verification**:
```go
// Initialize OIDC verifier at startup
provider, err := oidc.NewProvider(ctx, keycloakURL)
verifier := provider.Verifier(&oidc.Config{
    ClientID: "kuberde-cli",
    SkipIssuerCheck: true, // For K8s internal routing
})

// Validate incoming token
func validateToken(rawToken string) (*oidc.IDToken, error) {
    token, err := verifier.Verify(context.Background(), rawToken)
    if err != nil {
        return nil, fmt.Errorf("invalid token: %w", err)
    }

    var claims struct {
        PreferredUsername string `json:"preferred_username"`
    }
    if err := token.Claims(&claims); err != nil {
        return nil, err
    }

    return token, nil
}

// Check agent access authorization
func canAccessAgent(token *oidc.IDToken, agentID string) bool {
    var claims struct {
        PreferredUsername string `json:"preferred_username"`
    }
    token.Claims(&claims)

    // Agent ID format: user-{owner}-{name}
    expectedPrefix := fmt.Sprintf("user-%s-", claims.PreferredUsername)
    return strings.HasPrefix(agentID, expectedPrefix)
}
```

### 6.5 TTL Scaledown Implementation

**Operator TTL Checker**:
```go
func (o *Operator) startTTLChecker(ctx context.Context) {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            o.checkAllAgentTTLs(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (o *Operator) checkAllAgentTTLs(ctx context.Context) {
    // List all RDEAgent CRs
    var agents frpv1.RDEAgentList
    if err := o.client.List(ctx, &agents); err != nil {
        log.Error(err, "failed to list agents")
        return
    }

    for _, agent := range agents.Items {
        if agent.Spec.TTL == "0" {
            continue // TTL disabled
        }

        // Query server for agent stats
        stats, err := o.getAgentStats(agent.GetAgentID())
        if err != nil {
            log.Error(err, "failed to get stats", "agent", agent.Name)
            continue
        }

        // Check if idle time exceeds TTL
        ttl, _ := time.ParseDuration(agent.Spec.TTL)
        idleTime := time.Since(stats.LastActivity)

        if idleTime > ttl {
            // Scale down to 0
            deployment := &appsv1.Deployment{}
            if err := o.client.Get(ctx, types.NamespacedName{
                Namespace: agent.Namespace,
                Name: agent.GetAgentID(),
            }, deployment); err == nil {
                replicas := int32(0)
                deployment.Spec.Replicas = &replicas
                o.client.Update(ctx, deployment)
                log.Info("scaled down idle agent", "agent", agent.Name)
            }
        }
    }
}
```

### 6.6 WebSocket to Yamux Adapter

**Custom Wrapper for gorilla/websocket**:
```go
// wsConn wraps websocket.Conn to implement io.Reader for Yamux
type wsConn struct {
    *websocket.Conn
    reader io.Reader
}

func newWSConn(ws *websocket.Conn) *wsConn {
    return &wsConn{Conn: ws}
}

func (c *wsConn) Read(p []byte) (int, error) {
    if c.reader == nil {
        messageType, r, err := c.NextReader()
        if err != nil {
            return 0, err
        }
        if messageType != websocket.BinaryMessage {
            return 0, fmt.Errorf("expected binary message, got %d", messageType)
        }
        c.reader = r
    }

    n, err := c.reader.Read(p)
    if err == io.EOF {
        c.reader = nil // Reset for next message
        err = nil
    }
    return n, err
}

func (c *wsConn) Write(p []byte) (int, error) {
    w, err := c.NextWriter(websocket.BinaryMessage)
    if err != nil {
        return 0, err
    }
    defer w.Close()

    return w.Write(p)
}
```

---

## Appendix: Architecture Evolution Timeline

```
v0.1 (Initial)
├── ✅ Basic agent/server communication
├── ✅ OIDC authentication
├── ✅ Kubernetes operator
├── ✅ TTL scaledown
└── ❌ Many rough edges

↓

v0.2 (Current - Dec 2025)
├── ✅ Auto scale-up
├── ✅ Deployment updates
├── ✅ Status reporting
├── ✅ Production deployment
└── ⚠️  Security improvements needed

↓

v1.0 (Target - Q2 2025)
├── ✅ CRD v1beta1
├── ✅ Session security
├── ✅ Token refresh
├── ✅ Prometheus metrics
└── ✅ Production-ready

↓

v2.0 (Vision - 2026)
├── Multi-cluster
├── Advanced scheduling
├── Enterprise security
├── Full observability
└── Ecosystem integration
```

---

**Document Maintenance**:
- Review quarterly or after major releases
- Update decision records as implementations change
- Archive outdated sections to separate docs
- Keep technical details accurate to current codebase

**References**:
- Source code: `/cmd/*`, `/pkg/*`
- Deployment: `/deploy/k8s/`
- Developer guide: `CLAUDE.md`
- Operations: `docs/guides/OPERATORS_RUNBOOK.md`
