# FRP Scaling Guide

**Version**: 1.0
**Last Updated**: 2025-12-08
**Audience**: Platform engineers, SREs

---

## Table of Contents

1. [Overview](#overview)
2. [Resource Requirements](#resource-requirements)
3. [Performance Tuning](#performance-tuning)
4. [Scaling to 50+ Agents](#scaling-to-50-agents)
5. [Load Testing](#load-testing)
6. [Bottleneck Identification](#bottleneck-identification)
7. [Horizontal Scaling](#horizontal-scaling)
8. [Cost Optimization](#cost-optimization)

---

## Overview

This guide covers scaling FRP from development to production, including resource planning, performance optimization, and capacity management.

### Scaling Dimensions

FRP scaling has multiple dimensions:

| Dimension | Metric | Impact |
|-----------|--------|--------|
| **Agent Count** | Number of RDEAgent CRs | Operator reconciliation load |
| **Concurrent Connections** | Active user connections | Server CPU/memory |
| **Throughput** | Bytes/second transferred | Server network I/O |
| **Yamux Streams** | Streams per agent | Server goroutine count |
| **Request Rate** | HTTP requests/second | Server CPU |

### Current Limits

**Tested Configuration** (as of v1.0):
- **Agents**: Up to 20 agents tested
- **Concurrent users**: Up to 50 connections
- **Throughput**: Up to 100 MB/s aggregate
- **Server**: Single replica, 2 CPU / 4 GB RAM

**Theoretical Limits**:
- **Agents**: 500+ per server (limited by Yamux sessions)
- **Concurrent connections**: 1000+ (limited by goroutines)
- **Throughput**: Network-bound, not CPU-bound

---

## Resource Requirements

### Small Deployment (1-10 Agents)

**Use Case**: Development, testing, small team

**Server Resources**:
```yaml
resources:
  requests:
    cpu: "500m"
    memory: "512Mi"
  limits:
    cpu: "1000m"
    memory: "1Gi"
```

**Operator Resources**:
```yaml
resources:
  requests:
    cpu: "100m"
    memory: "128Mi"
  limits:
    cpu: "200m"
    memory: "256Mi"
```

**Agent Resources** (per agent):
```yaml
resources:
  requests:
    cpu: "50m"
    memory: "64Mi"
  limits:
    cpu: "200m"
    memory: "256Mi"
```

**Cluster Requirements**:
- **Nodes**: 1-2 nodes (for HA)
- **Total CPU**: 2-4 cores
- **Total Memory**: 4-8 GB
- **Storage**: 10 GB (for logs, Keycloak DB)

### Medium Deployment (10-50 Agents)

**Use Case**: Production, medium team, moderate traffic

**Server Resources**:
```yaml
resources:
  requests:
    cpu: "2000m"
    memory: "4Gi"
  limits:
    cpu: "4000m"
    memory: "8Gi"
```

**Operator Resources**:
```yaml
resources:
  requests:
    cpu: "200m"
    memory: "256Mi"
  limits:
    cpu: "500m"
    memory: "512Mi"
```

**Agent Resources** (per agent):
```yaml
# Same as small deployment
# Workload resources vary by use case
```

**Cluster Requirements**:
- **Nodes**: 3-5 nodes
- **Total CPU**: 8-16 cores
- **Total Memory**: 16-32 GB
- **Storage**: 50 GB
- **Network**: 1 Gbps+

### Large Deployment (50+ Agents)

**Use Case**: Enterprise, high traffic, multi-team

**Server Resources**:
```yaml
# Multiple replicas (see Horizontal Scaling section)
replicas: 3
resources:
  requests:
    cpu: "4000m"
    memory: "8Gi"
  limits:
    cpu: "8000m"
    memory: "16Gi"
```

**Operator Resources**:
```yaml
# Leader-election enabled
replicas: 2
resources:
  requests:
    cpu: "500m"
    memory: "512Mi"
  limits:
    cpu: "1000m"
    memory: "1Gi"
```

**Cluster Requirements**:
- **Nodes**: 5-10+ nodes
- **Total CPU**: 32+ cores
- **Total Memory**: 64+ GB
- **Storage**: 200+ GB
- **Network**: 10 Gbps+

### Keycloak Resources

**Small/Medium**:
```yaml
resources:
  requests:
    cpu: "500m"
    memory: "1Gi"
  limits:
    cpu: "1000m"
    memory: "2Gi"
```

**Large** (with external database):
```yaml
resources:
  requests:
    cpu: "2000m"
    memory: "2Gi"
  limits:
    cpu: "4000m"
    memory: "4Gi"
# Plus PostgreSQL: 1 CPU, 2 GB RAM minimum
```

---

## Performance Tuning

### Server Optimization

#### Go Runtime Tuning

**GOMAXPROCS** (CPU parallelism):
```yaml
# In server deployment
env:
- name: GOMAXPROCS
  value: "4"  # Match CPU limit
```

**GOGC** (Garbage collection):
```yaml
env:
- name: GOGC
  value: "100"  # Default
  # Lower (e.g., 50) = more frequent GC, less memory
  # Higher (e.g., 200) = less GC, more memory
```

**GOMEMLIMIT** (Go 1.19+):
```yaml
env:
- name: GOMEMLIMIT
  value: "7GiB"  # Slightly below container limit
```

#### WebSocket Configuration

**Connection limits** (Optimized for large file transfers):
```go
// In server code - supports 2GB @ 10MB/s (~3.5 min)
server := &http.Server{
    Addr:              ":8080",
    ReadTimeout:       10 * time.Minute,  // Increased for large uploads
    WriteTimeout:      10 * time.Minute,  // Increased for large downloads
    IdleTimeout:       15 * time.Minute,  // Keep connections alive
    ReadHeaderTimeout: 30 * time.Second,  // Prevent slowloris attacks
    MaxHeaderBytes:    1 << 20,           // 1 MB max header size
}
```

**Yamux configuration** (Aligned timeouts for both server and agent):
```go
config := yamux.DefaultConfig()
config.AcceptBacklog = 256                    // Concurrent stream accepts
config.EnableKeepAlive = true
config.KeepAliveInterval = 30 * time.Second   // Increased for stability
config.ConnectionWriteTimeout = 120 * time.Second  // Supports large file transfers
```

**Bandwidth scenarios supported**:
- **2GB @ 10 MB/s**: ~205 seconds (3.4 minutes) ✅ Within timeout
- **2GB @ 50 MB/s**: ~41 seconds ✅ Within timeout
- **2GB @ 100 MB/s**: ~20 seconds ✅ Within timeout

#### HTTP Reverse Proxy Tuning

**Transport pool**:
```go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
    DisableKeepAlives:   false,
}
```

### Operator Optimization

#### Reconciliation Rate Limiting

**Current**: Unbounded reconciliation

**Recommended**:
```go
// In operator manager
opts := ctrl.Options{
    // Max reconciles per second
    RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(
        5*time.Millisecond,  // Base delay
        1000*time.Second,    // Max delay
    ),
}
```

#### Leader Election (Multi-Replica)

```yaml
# In operator deployment
args:
- --leader-elect=true
- --leader-election-id=kuberde-operator-leader
- --leader-election-namespace=kuberde
```

#### TTL Check Optimization

**Current**: Check all agents every 60 seconds

**Optimized**:
```go
// Only check agents with TTL enabled
// Stagger checks to distribute load
func (o *Operator) checkAgentTTL(agent *frpv1.RDEAgent) {
    if agent.Spec.TTL == "0" {
        return  // Skip TTL-disabled agents
    }

    // Stagger by agent name hash
    hash := hashString(agent.Name)
    delay := time.Duration(hash%60) * time.Second
    time.Sleep(delay)

    // Proceed with check
    // ...
}
```

### Agent Optimization

#### Token Caching

**Avoid unnecessary refreshes**:
```go
// Use oauth2.TokenSource with auto-refresh
tokenSource := oauth2config.TokenSource(ctx, token)

// Reuse token until expiry
for {
    token, err := tokenSource.Token()
    if err != nil {
        log.Error("token refresh failed", err)
        time.Sleep(30 * time.Second)
        continue
    }

    // Use token
    // TokenSource automatically refreshes when needed
}
```

#### Connection Retry Logic

**Exponential backoff**:
```go
backoff := 1 * time.Second
maxBackoff := 5 * time.Minute

for {
    err := connectToServer()
    if err == nil {
        break  // Success
    }

    log.Warn("connection failed, retrying", "backoff", backoff)
    time.Sleep(backoff)

    backoff *= 2
    if backoff > maxBackoff {
        backoff = maxBackoff
    }
}
```

### Workload Optimization

#### Resource Limits

**SSH workload** (minimal):
```yaml
resources:
  requests:
    cpu: "100m"
    memory: "128Mi"
  limits:
    cpu: "500m"
    memory: "512Mi"
```

**Jupyter Notebook** (moderate):
```yaml
resources:
  requests:
    cpu: "1000m"
    memory: "2Gi"
  limits:
    cpu: "4000m"
    memory: "8Gi"
```

**GPU workload** (high):
```yaml
resources:
  requests:
    cpu: "2000m"
    memory: "8Gi"
    nvidia.com/gpu: "1"
  limits:
    cpu: "8000m"
    memory: "32Gi"
    nvidia.com/gpu: "1"
```

---

## Scaling to 50+ Agents

### Pre-Scaling Checklist

- [ ] Enable resource quotas per namespace
- [ ] Configure Horizontal Pod Autoscaler (HPA) for server
- [ ] Set up Prometheus monitoring
- [ ] Configure alerting for high load
- [ ] Review network policies for performance
- [ ] Benchmark current system (see Load Testing)
- [ ] Plan rollout strategy (staged scaling)

### Server Horizontal Pod Autoscaler

**HPA Configuration**:
```yaml
# File: deploy/k8s/scaling/server-hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: frp-server-hpa
  namespace: kuberde
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: frp-server
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 100
        periodSeconds: 30
```

**Apply HPA**:
```bash
kubectl apply -f deploy/k8s/scaling/server-hpa.yaml

# Monitor HPA
kubectl get hpa -n kuberde -w
```

### Load Balancing Strategy

**Problem**: WebSocket connections are sticky, standard load balancing doesn't work

**Solution 1: Session Affinity** (Client IP):
```yaml
# In server Service
spec:
  sessionAffinity: ClientIP
  sessionAffinityConfig:
    clientIP:
      timeoutSeconds: 10800  # 3 hours
```

**Solution 2: Consistent Hashing** (Ingress):
```yaml
# In Ingress
annotations:
  nginx.ingress.kubernetes.io/upstream-hash-by: "$remote_addr"
```

**Solution 3: Agent-Side Load Balancing** (Recommended for 50+):
```go
// Agent connects to one of multiple server endpoints
servers := []string{
    "ws://frp-server-0.kuberde.svc/ws",
    "ws://frp-server-1.kuberde.svc/ws",
    "ws://frp-server-2.kuberde.svc/ws",
}

// Pick server based on agent ID hash
serverIndex := hashAgentID(agentID) % len(servers)
serverURL := servers[serverIndex]
```

### Database for Shared State

**Current**: In-memory session storage

**Problem**: Multiple server replicas don't share state

**Solution**: Redis for session storage

**Redis Deployment**:
```yaml
# File: deploy/k8s/scaling/redis.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: kuberde
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
        resources:
          requests:
            cpu: "250m"
            memory: "512Mi"
          limits:
            cpu: "500m"
            memory: "1Gi"
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: kuberde
spec:
  selector:
    app: redis
  ports:
  - port: 6379
```

**Server Integration** (code changes needed):
```go
import "github.com/go-redis/redis/v8"

// Initialize Redis client
rdb := redis.NewClient(&redis.Options{
    Addr: "redis.kuberde.svc:6379",
})

// Store session
rdb.Set(ctx, sessionID, tokenJSON, 24*time.Hour)

// Retrieve session
val, err := rdb.Get(ctx, sessionID).Result()
```

### Operator Leader Election

**Enable for multiple operator replicas**:

```yaml
# In operator deployment
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: manager
        args:
        - --leader-elect=true
```

**Verify leader**:
```bash
# Check which pod is leader
kubectl get lease -n kuberde kuberde-operator-leader -o yaml

# Should see holder identity
```

### Agent Distribution

**Spread agents across nodes**:

```yaml
# In operator-generated deployment
spec:
  template:
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchLabels:
                  component: frp-agent
              topologyKey: kubernetes.io/hostname
```

---

## Load Testing

### Test Environment Setup

**Prerequisites**:
- Kubernetes cluster with monitoring
- Test agents deployed
- Load testing tools installed

**Test Agent CRD**:
```yaml
# File: deploy/k8s/testing/load-test-agents.yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: load-test-{001..050}  # Creates 50 agents
spec:
  owner: loadtest
  serverUrl: "ws://frp-server.kuberde.svc/ws"
  authSecret: "frp-agent-auth"
  workloadImage: "nginx:alpine"
  localTarget: "localhost:80"
  ttl: "0"  # Disable TTL for testing
```

### Connection Load Test

**Tool**: Custom Go script

```go
// File: scripts/load-test-connections.go
package main

import (
    "fmt"
    "net/http"
    "sync"
    "time"
)

func main() {
    concurrency := 100
    duration := 5 * time.Minute
    target := "http://user-loadtest-load-test-001.frp.byai.uk/"

    var wg sync.WaitGroup
    start := time.Now()

    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for time.Since(start) < duration {
                resp, err := http.Get(target)
                if err != nil {
                    fmt.Printf("Worker %d error: %v\n", id, err)
                    continue
                }
                resp.Body.Close()
                time.Sleep(100 * time.Millisecond)
            }
        }(i)
    }

    wg.Wait()
    fmt.Println("Load test complete")
}
```

**Run test**:
```bash
go run scripts/load-test-connections.go
```

### Throughput Load Test

**Tool**: `iperf3` through FRP tunnel

**Setup**:
```bash
# On agent side (in workload container)
iperf3 -s

# On client side
kuberde-cli connect --agent-id user-loadtest-load-test-001 &
iperf3 -c localhost -p 2022 -t 60
```

**Expected Results**:
- **Small deployment**: 50-100 MB/s
- **Medium deployment**: 200-500 MB/s
- **Large deployment**: 500+ MB/s

### Stress Test

**Tool**: Vegeta

```bash
# Install Vegeta
go install github.com/tsenart/vegeta@latest

# Define targets
echo "GET http://user-loadtest-load-test-001.frp.byai.uk/" > targets.txt

# Run attack
vegeta attack -duration=60s -rate=1000 -targets=targets.txt | \
  vegeta report -type=text

# Example output:
# Requests      [total, rate, throughput]  60000, 1000.02, 995.12
# Latencies     [mean, 50, 95, 99, max]    12.5ms, 10ms, 25ms, 50ms, 200ms
# Success       [ratio]                     99.8%
```

### Operator Reconciliation Test

**Create/delete agents rapidly**:

```bash
#!/bin/bash
# File: scripts/operator-stress-test.sh

for i in {1..100}; do
  kubectl apply -f - <<EOF
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: stress-test-$(printf "%03d" $i)
  namespace: kuberde
spec:
  owner: stresstest
  serverUrl: "ws://frp-server.kuberde.svc/ws"
  authSecret: "frp-agent-auth"
  workloadImage: "nginx:alpine"
EOF
  sleep 0.1
done

echo "Created 100 agents, monitoring reconciliation..."
kubectl get rdeagent -n kuberde -w

# Wait 5 minutes
sleep 300

# Delete all
kubectl delete rdeagent -n kuberde -l owner=stresstest
```

**Monitor operator**:
```bash
# Watch operator logs
kubectl logs -n kuberde -l app=kuberde-operator -f | grep "Reconciling"

# Check metrics
kubectl top pod -n kuberde -l app=kuberde-operator
```

### Load Test Metrics to Monitor

| Metric | Good | Warning | Critical |
|--------|------|---------|----------|
| Server CPU | < 70% | 70-85% | > 85% |
| Server Memory | < 80% | 80-90% | > 90% |
| Request Latency (p95) | < 100ms | 100-500ms | > 500ms |
| Error Rate | < 0.1% | 0.1-1% | > 1% |
| Connection Success | > 99.9% | 99-99.9% | < 99% |
| Operator Reconcile Lag | < 5s | 5-30s | > 30s |

---

## Bottleneck Identification

### CPU Bottleneck

**Symptoms**:
- High CPU usage (> 80%)
- Request latency increasing
- Throttling in metrics

**Diagnosis**:
```bash
# Check CPU usage
kubectl top pod -n kuberde -l app=frp-server

# Profile CPU (requires pprof endpoint)
go tool pprof http://frp-server:6060/debug/pprof/profile?seconds=30
```

**Solutions**:
- Increase CPU limits
- Enable HPA for horizontal scaling
- Optimize hot paths (profiling)

### Memory Bottleneck

**Symptoms**:
- High memory usage (> 90%)
- OOMKilled events
- Frequent GC pauses

**Diagnosis**:
```bash
# Check memory
kubectl top pod -n kuberde -l app=frp-server

# Memory profile
go tool pprof http://frp-server:6060/debug/pprof/heap
```

**Solutions**:
- Increase memory limits
- Tune GOGC (more aggressive GC)
- Reduce session/cache sizes
- Fix memory leaks

### Network Bottleneck

**Symptoms**:
- Low throughput despite low CPU/memory
- Network errors in logs
- Packet loss

**Diagnosis**:
```bash
# Check network I/O
kubectl exec -n kuberde frp-server-xxx -- \
  netstat -i

# Check CNI plugin limits
kubectl describe node {node-name}
```

**Solutions**:
- Use nodes with higher network bandwidth
- Enable network optimizations (jumbo frames, TCP tuning)
- Distribute agents across more nodes

### Goroutine Leak

**Symptoms**:
- Goroutine count continuously increasing
- Memory slowly growing
- Eventually OOMKilled

**Diagnosis**:
```bash
# Goroutine profile
go tool pprof http://frp-server:6060/debug/pprof/goroutine

# Look for stuck goroutines
```

**Solutions**:
- Review io.Copy loops (ensure proper close)
- Add timeouts to blocking operations
- Fix deadlocks

### Database Bottleneck (Keycloak)

**Symptoms**:
- Slow authentication (> 1s)
- Keycloak high CPU/memory
- Token validation timeouts

**Solutions**:
- Use external PostgreSQL (not H2)
- Increase Keycloak replicas
- Enable connection pooling
- Cache JWKS responses

---

## Horizontal Scaling

### Server Scaling

**Stateless Server** (Recommended):

1. **Remove in-memory state**:
   - Move sessions to Redis
   - Use consistent hashing for agent routing

2. **Enable HPA** (see earlier section)

3. **Update Service**:
```yaml
# Headless service for agent discovery
apiVersion: v1
kind: Service
metadata:
  name: frp-server-headless
  namespace: kuberde
spec:
  clusterIP: None
  selector:
    app: frp-server
  ports:
  - port: 80
```

4. **Agent connects to any server**:
```go
// Use SRV records for discovery
servers, err := net.LookupSRV("", "", "frp-server-headless.kuberde.svc")
```

### Operator Scaling

**Leader-Follower Pattern**:

```yaml
# In operator deployment
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: manager
        args:
        - --leader-elect=true
        - --leader-election-lease-duration=15s
        - --leader-election-renew-deadline=10s
        - --leader-election-retry-period=2s
```

**Only leader reconciles, followers standby**

### Multi-Cluster Federation

**Architecture**:
```
Global Load Balancer
        │
    ┌───┴────┬────────┐
    │        │        │
 Cluster  Cluster  Cluster
  US-East  EU-West  AP-South
```

**Implementation**:
1. Deploy FRP server in each cluster
2. Use global load balancer (Cloudflare, AWS Route53)
3. Agents connect to nearest server
4. Share Keycloak (single source of truth)

---

## Cost Optimization

### Idle Agent Scaledown

**Current TTL feature**:
- Automatically scales idle agents to 0
- Saves CPU, memory, and costs

**Optimization**:
```yaml
# Set aggressive TTL for dev environments
spec:
  ttl: "30m"  # 30 minutes idle

# Longer TTL for production
spec:
  ttl: "8h"  # 8 hours idle
```

### Spot Instances for Agents

**Use spot/preemptible nodes**:

```yaml
# Node pool for agents (GKE example)
gcloud container node-pools create frp-agents \
  --cluster=my-cluster \
  --preemptible \
  --node-labels=workload-type=frp-agent \
  --node-taints=workload-type=frp-agent:NoSchedule

# Agent pod tolerations
spec:
  tolerations:
  - key: workload-type
    operator: Equal
    value: frp-agent
    effect: NoSchedule
  nodeSelector:
    workload-type: frp-agent
```

### Resource Right-Sizing

**Find over-provisioned resources**:

```bash
# Install metrics-server
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# Check actual usage vs requests
kubectl top pod -n kuberde --containers

# Compare to requests
kubectl get pod -n kuberde -o json | \
  jq '.items[] | {name: .metadata.name, requests: .spec.containers[].resources.requests}'
```

**Adjust based on data**:
- If usage < 50% of requests: reduce requests
- If usage > 80% of limits: increase limits

### Efficient Workload Images

**Use minimal base images**:
- ❌ `ubuntu:latest` (77 MB)
- ✅ `alpine:latest` (5 MB)
- ✅ `distroless` (2 MB)

**Multi-arch images** (ARM savings):
```dockerfile
# Build for both amd64 and arm64
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder
ARG TARGETOS TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o server ./cmd/server
```

### Monitoring Cost Metrics

**Track with Prometheus**:
```promql
# Total agent replicas (cost indicator)
sum(kube_deployment_spec_replicas{namespace="kuberde"})

# Average idle time (opportunity for savings)
avg(time() - frp_agent_last_activity_timestamp)

# Resource usage efficiency
sum(rate(container_cpu_usage_seconds_total{namespace="kuberde"}[5m])) /
sum(kube_pod_container_resource_requests{namespace="kuberde", resource="cpu"})
```

---

## Related Documentation

- [Monitoring Guide](MONITORING.md) - Metrics for scaling decisions
- [Security Guide](SECURITY.md) - Security at scale
- [Operators Runbook](../guides/OPERATORS_RUNBOOK.md) - Day-to-day operations
- [Architecture](../ARCHITECTURE.md) - System design and limits

---

**Document Version**: 1.0
**Maintainer**: Platform Team
**Next Review**: 2025-03-08
