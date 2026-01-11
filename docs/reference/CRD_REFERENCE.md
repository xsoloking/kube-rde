# RDEAgent CRD Reference

**System Version**: v2.1
**Last Updated**: 2025-12-07
**Audience**: Platform Engineers, Kubernetes Admins
**Related**: [API_REFERENCE.md](./API_REFERENCE.md), [CONFIGURATION.md](./CONFIGURATION.md)

---

## Overview

The **RDEAgent** Custom Resource Definition (CRD) allows declarative definition and management of FRP agents in Kubernetes. The operator watches RDEAgent CRs and automatically creates/updates Deployments to run agents.

### Key Points

- **CRD Group**: `kuberde.io`
- **CRD Version**: `v1`
- **Kind**: `RDEAgent`
- **Namespace**: `kuberde`
- **Operator**: Managed by `kuberde-operator` deployment

---

## Table of Contents

1. [Basic Structure](#basic-structure)
2. [Spec Fields](#spec-fields)
3. [Status Fields](#status-fields)
4. [Examples](#examples)
5. [Common Patterns](#common-patterns)
6. [Validation Rules](#validation-rules)

---

## Basic Structure

### Minimal RDEAgent Example

```yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: my-agent
  namespace: kuberde
spec:
  owner: alice
  localTarget: "127.0.0.1:3000"
```

This creates an agent with:
- **ID**: `user-alice-my-agent`
- **Target**: Local service on port 3000
- **TTL**: Default 5 minutes
- **Image**: Latest FRP agent
- **Replicas**: 1 (when active)

### Complete RDEAgent Example

```yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: my-app-dev
  namespace: kuberde
  labels:
    environment: development
    team: backend
  annotations:
    description: "Development environment for my-app"
spec:
  # Ownership and targeting
  owner: alice
  localTarget: "service-backend:8080"

  # Container configuration
  image: soloking/frp-agent:v2.1
  imagePullPolicy: IfNotPresent

  # Environment variables
  env:
    - name: SERVER_URL
      value: ws://frp-server:8080/ws
    - name: LOG_LEVEL
      value: info

  # Resource management
  resources:
    requests:
      cpu: "100m"
      memory: "64Mi"
    limits:
      cpu: "500m"
      memory: "256Mi"

  # Scheduling
  nodeSelector:
    agentWorkload: "true"
  tolerations:
    - key: "agentWorkload"
      operator: "Equal"
      value: "true"
      effect: "NoSchedule"

  # Time-to-live configuration
  ttlSecondsAfterFinished: 300  # 5 minutes

  # Security context
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    capabilities:
      drop:
        - ALL

status:
  phase: Synced
  activeReplicas: 1
  desiredReplicas: 1
  observedGeneration: 1
  lastActivity: "2025-12-07T12:45:30Z"
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2025-12-07T12:00:00Z"
      reason: "DeploymentReady"
      message: "Agent deployment is ready"
```

---

## Spec Fields

### Identity & Targeting

#### `owner` (string, required)

The username or identifier of the agent owner. Used in the agent ID as `user-{owner}-{name}`.

```yaml
spec:
  owner: alice
```

- **Format**: `[a-z0-9-]+`
- **Max Length**: 63 characters
- **Used For**: Authorization - only this user can access this agent
- **Generated Agent ID**: `user-alice-<metadata.name>`

#### `localTarget` (string, required)

The address of the local service to expose. Format: `hostname:port` or `service:port` (in-cluster) or `IP:port`.

```yaml
spec:
  localTarget: "my-service:8080"
```

**Examples**:
- `127.0.0.1:22` - SSH on localhost
- `service-backend:8080` - Kubernetes service
- `my-app.default.svc.cluster.local:3000` - Fully qualified service
- `10.0.1.5:5432` - Direct IP (database)

**Validation**:
- Must be valid `hostname:port` or `IP:port`
- Port must be 1-65535

---

### Container Configuration

#### `image` (string, optional)

Docker image for the agent.

```yaml
spec:
  image: soloking/frp-agent:v2.1
```

- **Default**: `soloking/frp-agent:latest`
- **Recommended**: Use specific version tag (e.g., `v2.1`) for stability
- **Registry**: Public Docker Hub (can be changed with `.imagePullPolicy`)

#### `imagePullPolicy` (string, optional)

When to pull the container image.

```yaml
spec:
  imagePullPolicy: IfNotPresent
```

- **IfNotPresent** (default): Use local image if exists
- **Always**: Always pull latest
- **Never**: Only use local image

---

### Environment Variables

#### `env` (array, optional)

Environment variables passed to the agent container.

```yaml
spec:
  env:
    - name: SERVER_URL
      value: ws://frp-server:8080/ws
    - name: LOG_LEVEL
      value: debug
    - name: CUSTOM_VAR
      valueFrom:
        secretKeyRef:
          name: agent-secrets
          key: api-key
```

**Standard Variables** (auto-set by operator):
| Variable | Set By | Purpose |
|----------|--------|---------|
| `SERVER_URL` | You (or operator default) | WebSocket endpoint of FRP server |
| `LOCAL_TARGET` | Operator (from `.localTarget`) | Service to expose |
| `AGENT_ID` | Operator (from owner + name) | Unique agent identifier |
| `AUTH_CLIENT_ID` | Secret | OAuth2 client ID |
| `AUTH_CLIENT_SECRET` | Secret | OAuth2 client secret |
| `AUTH_TOKEN_URL` | Secret | OAuth2 token endpoint |

**Custom Variables**:
```yaml
env:
  - name: LOG_LEVEL
    value: info
  - name: TIMEOUT
    value: "30"
  - name: RETRY_COUNT
    value: "3"
```

---

### Resource Management

#### `resources` (object, optional)

CPU and memory requests/limits for the agent pod.

```yaml
spec:
  resources:
    requests:
      cpu: "100m"      # 0.1 CPU cores
      memory: "64Mi"   # 64 MB
    limits:
      cpu: "500m"      # 0.5 CPU cores
      memory: "256Mi"  # 256 MB
```

**Defaults** (if not specified):
```
requests:
  cpu: 100m
  memory: 64Mi
limits:
  cpu: 500m
  memory: 256Mi
```

**Guidance**:
- **Minimal**: `100m` CPU, `64Mi` memory (for simple proxying)
- **Standard**: `200m` CPU, `128Mi` memory (recommended)
- **High-traffic**: `500m` CPU, `256Mi` memory (many concurrent users)
- **GPU workloads**: Set CPU/memory appropriately + add `nvidia.com/gpu: "1"`

---

### Scheduling

#### `nodeSelector` (object, optional)

Select which nodes can run this agent.

```yaml
spec:
  nodeSelector:
    agentWorkload: "true"
    disk: "ssd"
```

Requires nodes with matching labels:
```bash
kubectl label nodes my-node agentWorkload=true disk=ssd
```

#### `tolerations` (array, optional)

Tolerate node taints.

```yaml
spec:
  tolerations:
    - key: "agentWorkload"
      operator: "Equal"
      value: "true"
      effect: "NoSchedule"
    - key: "gpu"
      operator: "Exists"
      effect: "NoSchedule"
```

**When to use**:
- Agent node pool has taints
- Dedicated nodes for agents with special hardware
- Prevent agents from running on other workloads

---

### Security Context

#### `securityContext` (object, optional)

Security constraints for the agent pod.

```yaml
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 3000
    fsGroup: 2000
    capabilities:
      drop:
        - ALL
      add:
        - NET_BIND_SERVICE
    readOnlyRootFilesystem: true
    allowPrivilegeEscalation: false
```

**Common Patterns**:

1. **Minimal Security** (default):
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  capabilities:
    drop:
      - ALL
```

2. **SSH Access** (requires bind to low ports):
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  capabilities:
    add:
      - NET_BIND_SERVICE
```

3. **Strict Security**:
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
```

---

### Time-to-Live (TTL)

#### `ttlSecondsAfterFinished` (integer, optional)

Automatically scale down the agent after this many seconds of inactivity.

```yaml
spec:
  ttlSecondsAfterFinished: 300  # 5 minutes
```

**How it works**:
1. Agent connects to server, `lastActivity` is set
2. When users are connected, `lastActivity` updates
3. After `ttlSecondsAfterFinished` seconds with no activity, agent is scaled to `replicas=0`
4. When users connect again, agent is automatically scaled back up (auto-scale-up)

**Values**:
- `0` or omitted: TTL disabled (agent stays running)
- `60`: 1 minute (very aggressive)
- `300`: 5 minutes (default, recommended)
- `1800`: 30 minutes (longer idle tolerance)
- `3600`: 1 hour (allow long idle periods)

**Examples**:

```yaml
# Development: scale down after 5 minutes of inactivity
ttlSecondsAfterFinished: 300

# Production: scale down after 30 minutes
ttlSecondsAfterFinished: 1800

# Monitoring/always-on: disable TTL
ttlSecondsAfterFinished: 0
```

**Behavior Table**:

| TTL | Idle Time | Action |
|-----|-----------|--------|
| 300s | 0-299s | Running (replicas=1) |
| 300s | 300s+ | Scaled down (replicas=0) |
| 300s | 300s+ with new session | Auto-scaled up (replicas=1) |

---

## Status Fields

Status fields are **read-only**, updated by the operator.

### `phase` (string)

Current reconciliation state.

- **Pending**: Waiting for deployment to be created
- **Synced**: Deployment created and in sync
- **Failed**: Deployment creation failed
- **Unknown**: Operator status unknown

```yaml
status:
  phase: Synced
```

### `activeReplicas` (integer)

Number of ready agent pods.

```yaml
status:
  activeReplicas: 1
```

- `0`: Agent is scaled down (TTL enforced or no target)
- `1`: Normal running state
- `>1`: Agent deployment scaled up

### `desiredReplicas` (integer)

Number of replicas requested in the deployment spec.

```yaml
status:
  desiredReplicas: 1
```

Usually equals `activeReplicas` for healthy agents.

### `observedGeneration` (integer)

Tracks which generation of the CR was last processed.

```yaml
status:
  observedGeneration: 3
```

Used by operator to detect CR changes.

### `lastActivity` (timestamp)

When the agent last had an active user session.

```yaml
status:
  lastActivity: "2025-12-07T12:45:30Z"
```

**Used for**:
- TTL calculation (current time - lastActivity > ttlSecondsAfterFinished)
- Idle agent identification
- Usage tracking

### `conditions` (array)

List of conditions describing the agent state.

```yaml
status:
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2025-12-07T12:00:00Z"
      reason: "DeploymentReady"
      message: "Agent deployment is ready"
    - type: Connected
      status: "True"
      lastTransitionTime: "2025-12-07T12:00:15Z"
      reason: "WebSocketConnected"
      message: "Agent connected to server"
```

**Standard Conditions**:

| Type | Status | Meaning |
|------|--------|---------|
| `Ready` | True | Pods are running and ready |
| `Connected` | True | Agent connected to server |
| `Synced` | True | Deployment spec matches CR spec |

---

## Examples

### Example 1: Simple SSH Agent

```yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: ssh-tunnel
  namespace: kuberde
spec:
  owner: alice
  localTarget: "127.0.0.1:22"
  ttlSecondsAfterFinished: 300
```

Creates:
- Agent ID: `user-alice-ssh-tunnel`
- Exposes local SSH on port 22
- Scales down after 5 minutes of inactivity

### Example 2: Web Development Environment

```yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: dev-app
  namespace: kuberde
  labels:
    environment: development
spec:
  owner: bob
  localTarget: "localhost:3000"
  image: soloking/frp-agent:v2.1

  env:
    - name: LOG_LEVEL
      value: debug

  resources:
    requests:
      cpu: "100m"
      memory: "64Mi"
    limits:
      cpu: "200m"
      memory: "128Mi"

  ttlSecondsAfterFinished: 600  # 10 minutes
```

### Example 3: Production Database Access

```yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: postgres-prod
  namespace: kuberde
  labels:
    environment: production
    criticality: high
spec:
  owner: admin
  localTarget: "postgres-db.database.svc.cluster.local:5432"

  # Strict security for production
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    readOnlyRootFilesystem: true
    allowPrivilegeEscalation: false
    capabilities:
      drop:
        - ALL

  # Higher resources for database workload
  resources:
    requests:
      cpu: "200m"
      memory: "128Mi"
    limits:
      cpu: "500m"
      memory: "256Mi"

  # Disable TTL for critical production service
  ttlSecondsAfterFinished: 0

  # Run on dedicated nodes
  nodeSelector:
    node-type: production
```

### Example 4: Kubernetes Dashboard Access

```yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: k8s-dashboard
  namespace: kuberde
spec:
  owner: platform-admin
  localTarget: "kubernetes-dashboard.kubernetes-dashboard.svc.cluster.local:443"

  image: soloking/frp-agent:v2.1

  # Access kubernetes API securely
  env:
    - name: INSECURE_SKIP_VERIFY
      value: "false"

  # Keep running always for admin access
  ttlSecondsAfterFinished: 0
```

### Example 5: Multi-owner Setup

```yaml
---
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: api-dev
  namespace: kuberde
spec:
  owner: alice
  localTarget: "api-service:8080"
  ttlSecondsAfterFinished: 300

---
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: api-dev
  namespace: kuberde
spec:
  owner: bob
  localTarget: "api-service:8080"
  ttlSecondsAfterFinished: 300

# Creates two agents:
# user-alice-api-dev
# user-bob-api-dev
# Same owner can have multiple agents
```

---

## Common Patterns

### Pattern 1: Development Environment (Auto-Scaling)

Development agents should scale down when not in use to save resources:

```yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: dev-server
  namespace: kuberde
  labels:
    environment: dev
spec:
  owner: developer-name
  localTarget: "localhost:3000"
  ttlSecondsAfterFinished: 300  # 5 min inactivity
  resources:
    requests:
      cpu: "100m"
      memory: "64Mi"
    limits:
      cpu: "200m"
      memory: "128Mi"
```

### Pattern 2: Production Service (Always Running)

Production agents should never scale down:

```yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: prod-service
  namespace: kuberde
  labels:
    environment: prod
spec:
  owner: system
  localTarget: "production-db:5432"
  ttlSecondsAfterFinished: 0  # DISABLE TTL
  resources:
    requests:
      cpu: "500m"
      memory: "256Mi"
    limits:
      cpu: "1000m"
      memory: "512Mi"
  nodeSelector:
    node-pool: production
```

### Pattern 3: High-Traffic Workload

Agents with many concurrent users:

```yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: api-gateway
  namespace: kuberde
spec:
  owner: platform
  localTarget: "api-gateway:8080"
  ttlSecondsAfterFinished: 0  # Keep running
  resources:
    requests:
      cpu: "1000m"
      memory: "512Mi"
    limits:
      cpu: "2000m"
      memory: "1Gi"
```

### Pattern 4: Multi-Tenant Setup

Different agents per customer/team:

```yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: customer-a-api
  namespace: kuberde
  labels:
    customer: customer-a
spec:
  owner: customer-a-admin
  localTarget: "customer-a-api:8080"
  ttlSecondsAfterFinished: 1800  # 30 min for customers

---
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: customer-b-api
  namespace: kuberde
  labels:
    customer: customer-b
spec:
  owner: customer-b-admin
  localTarget: "customer-b-api:8080"
  ttlSecondsAfterFinished: 1800
```

---

## Validation Rules

### Required Fields
- `metadata.name`: Agent name (1-63 alphanumeric + hyphen)
- `metadata.namespace`: Must be `kuberde`
- `spec.owner`: Owner identifier (required for authorization)
- `spec.localTarget`: Must be valid `hostname:port` or `IP:port`

### Naming Constraints
- **CR Name** (`metadata.name`): `[a-z0-9-]+`, max 63 chars
- **Owner** (`spec.owner`): `[a-z0-9-]+`, max 63 chars
- **Generated Agent ID**: `user-{owner}-{name}`, max 63 chars total
  - If too long, agent creation will fail
  - Recommendation: keep name + owner totaling < 50 chars

### Port Validation
- Must be integer 1-65535
- Ports < 1024 require special privilege
- Common ports: 22 (SSH), 3306 (MySQL), 5432 (PostgreSQL), 8080 (HTTP)

### TTL Validation
- Must be 0 (disabled) or >= 10
- Value in seconds

### Resource Validation
- Requests must be < Limits
- CPU: `100m` - `10000m` (typical)
- Memory: `32Mi` - `4Gi` (typical)

### Image Validation
- Must be valid Docker image reference
- Format: `registry/image:tag` or `image:tag`
- Common: `soloking/frp-agent:v2.1`

---

## Troubleshooting CRs

### Agent Not Starting

**Check CR**:
```bash
kubectl get rdeagent -n kuberde <name> -o yaml
kubectl describe rdeagent -n kuberde <name>
```

**Check Status**:
```bash
kubectl get rdeagent -n kuberde <name> -o jsonpath='{.status.phase}'
```

**If Pending**:
- Operator may not have created deployment yet
- Check operator logs

**If Failed**:
- Check CR syntax: `kubectl apply --dry-run -f agent.yaml`
- Verify `localTarget` is reachable

### Deployment Not Synced

**Check CR matches Deployment**:
```bash
# Get CR image
kubectl get rdeagent -n kuberde <name> -o jsonpath='{.spec.image}'

# Get Deployment image
kubectl get deployment -n kuberde user-<owner>-<name> -o jsonpath='{.spec.template.spec.containers[0].image}'
```

**If mismatch**:
- Restart operator: `kubectl rollout restart deployment/kuberde-operator -n kuberde`

### Authentication Failed

**Verify Secret**:
```bash
kubectl get secret -n kuberde user-<owner>-<name> -o yaml
```

**Check credentials**:
```bash
kubectl get secret -n kuberde user-<owner>-<name> -o jsonpath='{.data.client_id}' | base64 -d
```

---

## Quick Reference Table

| Field | Required | Type | Default | Example |
|-------|----------|------|---------|---------|
| `owner` | Yes | string | N/A | `alice` |
| `localTarget` | Yes | string | N/A | `service:8080` |
| `image` | No | string | `latest` | `soloking/frp-agent:v2.1` |
| `ttlSecondsAfterFinished` | No | integer | 300 | `600` |
| `resources.requests.cpu` | No | string | `100m` | `200m` |
| `resources.requests.memory` | No | string | `64Mi` | `128Mi` |
| `resources.limits.cpu` | No | string | `500m` | `1000m` |
| `resources.limits.memory` | No | string | `256Mi` | `512Mi` |
| `nodeSelector` | No | map | N/A | `{key: value}` |
| `tolerations` | No | array | N/A | See examples |
| `securityContext` | No | object | N/A | See examples |

---

**Document Version**: 1.0
**Last Reviewed**: 2025-12-07
**Next Review**: 2025-12-14
