# Server Startup and Health Check Behavior

## Overview

The KubeRDE server implements a robust startup mechanism with retry logic for critical dependencies (PostgreSQL and Keycloak). This ensures the service only becomes ready after all dependencies are successfully initialized.

## Startup Sequence

### 1. Keycloak (OIDC) Initialization

**Location**: `cmd/server/main.go` - `initAuth()` function

**Behavior**:
- Attempts to connect to Keycloak OIDC provider
- **Retry logic**: 3 attempts with 5-second intervals
- **Total retry time**: ~15 seconds (3 attempts × 5s)
- **On failure**: Pod exits with `log.Fatalf()`, triggering Kubernetes restart

**Example logs**:
```
WARNING: Failed to initialize OIDC provider (attempt 1/3): dial tcp: connect: connection refused. Retrying in 5s...
WARNING: Failed to initialize OIDC provider (attempt 2/3): dial tcp: connect: connection refused. Retrying in 5s...
FATAL: Failed to initialize OIDC provider after 3 retries: dial tcp: connect: connection refused. Pod will restart.
```

### 2. PostgreSQL Database Initialization

**Location**: `cmd/server/main.go` - `main()` function (database initialization section)

**Behavior**:
- Attempts to connect to PostgreSQL database
- **Retry logic**: 3 attempts with 5-second intervals
- **Total retry time**: ~15 seconds (3 attempts × 5s)
- **On failure**: Pod exits with `log.Fatalf()`, triggering Kubernetes restart

**Example logs**:
```
WARNING: Failed to initialize PostgreSQL database (attempt 1/3): dial tcp 10.43.0.10:5432: connect: connection refused. Retrying in 5s...
WARNING: Failed to initialize PostgreSQL database (attempt 2/3): dial tcp 10.43.0.10:5432: connect: connection refused. Retrying in 5s...
FATAL: Failed to initialize PostgreSQL database after 3 retries: dial tcp 10.43.0.10:5432: connect: connection refused. Pod will restart.
```

### 3. Successful Initialization

When both dependencies are successfully connected:
```
✓ PostgreSQL database initialized
✓ Repository instances initialized
OIDC Auth initialized successfully
```

## Health Check Endpoints

### `/healthz` (Liveness Probe)

**Purpose**: Basic server health check

**Checks**:
- Server process is running and responding

**Returns**:
- `200 OK`: Server is alive
- Response: `{"status":"ok","service":"kuberde-server"}`

**Use case**: Kubernetes uses this to determine if the pod needs to be restarted

### `/readyz` (Readiness Probe)

**Purpose**: Service readiness check

**Checks**:
1. **Database connection**: Verifies PostgreSQL connection with `db.Ping()`
2. **OIDC verifier**: Verifies Keycloak OIDC provider is initialized

**Returns**:
- `200 OK`: Service is ready to accept traffic
  - Response: `{"status":"ready","service":"kuberde-server"}`
- `503 Service Unavailable`: Service is not ready
  - Database failure: `{"status":"not ready","reason":"database connection failed"}`
  - OIDC failure: `{"status":"not ready","reason":"OIDC verifier not initialized"}`

**Use case**: Kubernetes uses this to determine if the pod should receive traffic

## Kubernetes Configuration

### Readiness Probe Configuration

**values.yaml**:
```yaml
server:
  readinessProbe:
    enabled: true
    path: /readyz
    port: 8080
    initialDelaySeconds: 20  # Allow time for DB and Keycloak connection retries
    periodSeconds: 5
    timeoutSeconds: 3
    failureThreshold: 3      # Allow 3 failures before marking as not ready
```

**Timing breakdown**:
- **initialDelaySeconds: 20s**: Gives enough time for:
  - Keycloak initialization retries (~15s max)
  - PostgreSQL initialization retries (~15s max)
  - Server startup overhead (~5s)
- **periodSeconds: 5s**: Check readiness every 5 seconds
- **failureThreshold: 3**: Allow 3 consecutive failures (15s grace period)

### Liveness Probe Configuration

**values.yaml**:
```yaml
server:
  livenessProbe:
    enabled: true
    path: /healthz
    port: 8080
    initialDelaySeconds: 30  # Give startup time
    periodSeconds: 10
    timeoutSeconds: 5
    failureThreshold: 3
```

## Failure Scenarios

### Scenario 1: Keycloak Unavailable at Startup

**What happens**:
1. Server attempts to connect to Keycloak (3 times, 5s interval)
2. All attempts fail
3. Server logs fatal error and exits
4. Kubernetes detects pod exit and restarts it
5. Process repeats until Keycloak becomes available

**Kubernetes behavior**:
- Pod shows `CrashLoopBackOff` status
- Restart delay increases exponentially (10s, 20s, 40s, 80s, 160s, max 300s)
- Once Keycloak is available, next restart will succeed

### Scenario 2: PostgreSQL Unavailable at Startup

**What happens**:
1. Server attempts to connect to PostgreSQL (3 times, 5s interval)
2. All attempts fail
3. Server logs fatal error and exits
4. Kubernetes detects pod exit and restarts it
5. Process repeats until PostgreSQL becomes available

**Kubernetes behavior**: Same as Scenario 1

### Scenario 3: Keycloak Becomes Unavailable After Startup

**What happens**:
1. Server is running and healthy
2. Keycloak becomes unavailable
3. `/readyz` endpoint returns `503 Service Unavailable`
4. Kubernetes marks pod as not ready
5. Pod continues running (liveness check still passes)
6. No traffic is routed to the pod
7. When Keycloak recovers, `/readyz` returns `200 OK`
8. Pod becomes ready again and receives traffic

### Scenario 4: PostgreSQL Becomes Unavailable After Startup

**What happens**: Same as Scenario 3

## Best Practices

### 1. Deployment Order

For initial cluster setup, deploy in this order:
```bash
# 1. Deploy PostgreSQL first
kubectl apply -f deploy/k8s/06-postgresql.yaml

# 2. Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -l app=postgresql --timeout=300s

# 3. Deploy Keycloak
kubectl apply -f deploy/k8s/02-keycloak.yaml

# 4. Wait for Keycloak to be ready
kubectl wait --for=condition=ready pod -l app=keycloak --timeout=300s

# 5. Deploy server
kubectl apply -f deploy/k8s/03-server.yaml
```

### 2. Monitoring Startup Issues

**Check pod events**:
```bash
kubectl describe pod -l app=kuberde-server
```

**Check server logs**:
```bash
kubectl logs -l app=kuberde-server --tail=100
```

**Check readiness status**:
```bash
kubectl get pods -l app=kuberde-server
```

### 3. Production Considerations

**Resource requests/limits**: Ensure sufficient memory and CPU for startup:
```yaml
server:
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: 2000m
      memory: 2Gi
```

**Multiple replicas**: For HA, run multiple replicas:
```yaml
server:
  replicaCount: 2  # or more
```

**Pod disruption budget**: Prevent all pods from being terminated simultaneously:
```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: kuberde-server-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: kuberde-server
```

## Troubleshooting

### Pod is in CrashLoopBackOff

**Check logs for fatal errors**:
```bash
kubectl logs -l app=kuberde-server --previous
```

**Common causes**:
1. Keycloak not reachable
2. PostgreSQL not reachable
3. Invalid credentials
4. Network policy blocking connections

### Pod is Running but Not Ready

**Check readiness endpoint**:
```bash
kubectl port-forward svc/kuberde-server 8080:8080
curl http://localhost:8080/readyz
```

**Common causes**:
1. Database connection lost
2. Keycloak connection lost
3. Readiness probe timeout too short

### Slow Startup

**Check timing**:
- Keycloak connection: Max 15s
- PostgreSQL connection: Max 15s
- Total: ~30s + overhead

If startup takes longer:
1. Check network latency to dependencies
2. Verify resource limits are not causing CPU throttling
3. Increase `initialDelaySeconds` in readiness probe

## Related Files

- **Server code**: `cmd/server/main.go`
- **Helm values**: `charts/kuberde/values.yaml`
- **Deployment template**: `charts/kuberde/templates/server/deployment.yaml`
- **Local dev values**: `charts/kuberde/values-local-dev.yaml`
- **Production values**: `charts/kuberde/values-production.yaml`
