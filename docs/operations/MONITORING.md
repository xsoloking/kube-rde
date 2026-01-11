# FRP Monitoring Guide

**Version**: 1.0
**Last Updated**: 2025-12-08
**Audience**: Platform operators, SREs

---

## Table of Contents

1. [Overview](#overview)
2. [Key Metrics](#key-metrics)
3. [Prometheus Integration](#prometheus-integration)
4. [Common Alerts](#common-alerts)
5. [Health Checks](#health-checks)
6. [Log Aggregation](#log-aggregation)
7. [Dashboards](#dashboards)
8. [Troubleshooting](#troubleshooting)

---

## Overview

This guide covers monitoring and observability for the FRP system in production. It includes metrics collection, alerting strategies, health check procedures, and dashboard recommendations.

### Monitoring Goals

- **Availability**: Track uptime of server, agents, and operator
- **Performance**: Monitor latency, throughput, and resource usage
- **Capacity**: Identify scaling needs before issues occur
- **Security**: Detect authentication failures and suspicious activity
- **Cost**: Track idle agents and resource consumption

### Current State

**Implemented**:
- Server access logs (HTTP/WebSocket)
- Operator reconciliation logs
- Kubernetes native metrics (CPU/memory/pods)
- Agent statistics API (`/mgmt/agents/{id}`)

**Planned** (Near-Term):
- Prometheus metrics export
- Grafana dashboards
- AlertManager integration
- OpenTelemetry tracing

---

## Key Metrics

### Server Metrics

#### Connection Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `frp_agent_connections_total` | Counter | Total agent WebSocket connections | N/A |
| `frp_agent_connections_active` | Gauge | Currently connected agents | < 50% expected |
| `frp_user_connections_total` | Counter | Total user connections (SSH/HTTP) | N/A |
| `frp_user_connections_active` | Gauge | Active user connections | > 1000 |
| `frp_connection_errors_total` | Counter | Failed connection attempts | > 10/min |
| `frp_auth_failures_total` | Counter | OIDC authentication failures | > 5/min |

#### Performance Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `frp_bytes_transferred_total` | Counter | Total bytes proxied | N/A |
| `frp_request_duration_seconds` | Histogram | HTTP request latency | p99 > 2s |
| `frp_yamux_stream_open_duration` | Histogram | Yamux stream open latency | p95 > 500ms |
| `frp_websocket_message_size_bytes` | Histogram | WebSocket message sizes | N/A |

#### Resource Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `frp_server_cpu_usage` | Gauge | CPU usage percentage | > 80% |
| `frp_server_memory_bytes` | Gauge | Memory usage | > 90% limit |
| `frp_goroutines_count` | Gauge | Active goroutines | > 10000 |
| `frp_yamux_sessions_count` | Gauge | Active Yamux sessions | N/A |

### Operator Metrics

#### Reconciliation Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `frp_operator_reconcile_total` | Counter | Reconciliation attempts | N/A |
| `frp_operator_reconcile_errors_total` | Counter | Reconciliation failures | > 5/min |
| `frp_operator_reconcile_duration` | Histogram | Reconciliation duration | p95 > 5s |
| `frp_operator_ttl_scaledowns_total` | Counter | TTL-triggered scaledowns | N/A |
| `frp_operator_scale_ups_total` | Counter | Auto scale-up operations | N/A |

#### Agent Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `frp_agents_total` | Gauge | Total RDEAgent CRs | N/A |
| `frp_agents_active` | Gauge | Agents with replicas > 0 | N/A |
| `frp_agents_idle` | Gauge | Agents scaled to 0 | N/A |
| `frp_deployments_pending` | Gauge | Deployments not ready | > 0 for > 5min |

### Agent Metrics

#### Connection Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `frp_agent_server_connected` | Gauge | Connected to server (0/1) | 0 for > 2min |
| `frp_agent_streams_active` | Gauge | Active Yamux streams | > 50 |
| `frp_agent_local_connections_total` | Counter | Local target connections | N/A |
| `frp_agent_local_errors_total` | Counter | Local dial failures | > 10/min |

#### Authentication Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `frp_agent_token_refreshes_total` | Counter | OAuth2 token refreshes | N/A |
| `frp_agent_token_refresh_failures` | Counter | Token refresh failures | > 3 |
| `frp_agent_auth_errors_total` | Counter | Authentication errors | > 5 |

---

## Prometheus Integration

### Installation

If Prometheus is not installed in your cluster:

```bash
# Install Prometheus Operator
kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml

# Create Prometheus instance
kubectl apply -f - <<EOF
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: frp-prometheus
  namespace: kuberde
spec:
  serviceAccountName: prometheus
  serviceMonitorSelector:
    matchLabels:
      app: frp
  resources:
    requests:
      memory: 400Mi
EOF
```

### Metrics Endpoints

**FRP Server** (when metrics implemented):
```yaml
# Expose metrics port in Service
apiVersion: v1
kind: Service
metadata:
  name: frp-server
  namespace: kuberde
spec:
  ports:
  - name: http
    port: 80
  - name: metrics
    port: 9090  # Prometheus metrics
  selector:
    app: frp-server
```

**FRP Operator**:
```yaml
# Operator metrics exposed on :8080/metrics (controller-runtime default)
apiVersion: v1
kind: Service
metadata:
  name: kuberde-operator-metrics
  namespace: kuberde
spec:
  ports:
  - name: metrics
    port: 8080
    targetPort: 8080
  selector:
    app: kuberde-operator
```

### ServiceMonitor Configuration

Create ServiceMonitor for automatic scraping:

```yaml
# File: deploy/k8s/monitoring/servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: frp-server
  namespace: kuberde
  labels:
    app: frp
spec:
  selector:
    matchLabels:
      app: frp-server
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics

---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: kuberde-operator
  namespace: kuberde
  labels:
    app: frp
spec:
  selector:
    matchLabels:
      app: kuberde-operator
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

Apply with:
```bash
kubectl apply -f deploy/k8s/monitoring/servicemonitor.yaml
```

### Scrape Configuration (Manual)

If not using Prometheus Operator, add to `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'frp-server'
    kubernetes_sd_configs:
    - role: pod
      namespaces:
        names:
        - kuberde
    relabel_configs:
    - source_labels: [__meta_kubernetes_pod_label_app]
      action: keep
      regex: frp-server
    - source_labels: [__meta_kubernetes_pod_container_port_number]
      action: keep
      regex: "9090"

  - job_name: 'kuberde-operator'
    kubernetes_sd_configs:
    - role: pod
      namespaces:
        names:
        - kuberde
    relabel_configs:
    - source_labels: [__meta_kubernetes_pod_label_app]
      action: keep
      regex: kuberde-operator
    - source_labels: [__meta_kubernetes_pod_container_port_number]
      action: keep
      regex: "8080"
```

### Verifying Metrics Collection

```bash
# Check ServiceMonitor status
kubectl get servicemonitor -n kuberde

# Check if targets are discovered (port-forward to Prometheus)
kubectl port-forward -n kuberde svc/prometheus-operated 9090:9090

# Open http://localhost:9090/targets in browser
# Should see frp-server and kuberde-operator targets
```

### Query Examples

```promql
# Active agent count
frp_agent_connections_active

# User connection rate (per minute)
rate(frp_user_connections_total[1m])

# Error rate percentage
rate(frp_connection_errors_total[5m]) / rate(frp_user_connections_total[5m]) * 100

# p95 latency
histogram_quantile(0.95, frp_request_duration_seconds_bucket)

# Agents scaled down by TTL in last hour
increase(frp_operator_ttl_scaledowns_total[1h])
```

---

## Common Alerts

### Critical Alerts

#### Server Down

```yaml
# File: deploy/k8s/monitoring/alerts.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: frp-critical-alerts
  namespace: kuberde
spec:
  groups:
  - name: frp-server
    interval: 30s
    rules:
    - alert: FRPServerDown
      expr: up{job="frp-server"} == 0
      for: 2m
      labels:
        severity: critical
        component: server
      annotations:
        summary: "FRP Server is down"
        description: "FRP Server has been down for more than 2 minutes"
        runbook: "https://docs.frp.example.com/runbooks/server-down"
```

#### Operator Down

```yaml
    - alert: FRPOperatorDown
      expr: up{job="kuberde-operator"} == 0
      for: 5m
      labels:
        severity: critical
        component: operator
      annotations:
        summary: "FRP Operator is down"
        description: "Operator has been down for 5+ minutes. New agents cannot be created."
```

#### High Error Rate

```yaml
    - alert: FRPHighErrorRate
      expr: |
        rate(frp_connection_errors_total[5m]) > 10
      for: 5m
      labels:
        severity: critical
        component: server
      annotations:
        summary: "High connection error rate"
        description: "More than 10 connection errors per second in last 5 minutes"
```

### Warning Alerts

#### High Memory Usage

```yaml
  - name: frp-resources
    interval: 1m
    rules:
    - alert: FRPServerHighMemory
      expr: |
        container_memory_usage_bytes{pod=~"frp-server.*"}
        / container_spec_memory_limit_bytes{pod=~"frp-server.*"} > 0.9
      for: 10m
      labels:
        severity: warning
        component: server
      annotations:
        summary: "FRP Server memory usage > 90%"
        description: "Server memory usage has been above 90% for 10 minutes"
```

#### Agents Offline

```yaml
    - alert: FRPManyAgentsOffline
      expr: frp_agents_active / frp_agents_total < 0.5
      for: 15m
      labels:
        severity: warning
        component: operator
      annotations:
        summary: "More than 50% agents offline"
        description: "Check if mass TTL scaledown or connectivity issue"
```

#### Authentication Failures

```yaml
    - alert: FRPAuthFailureSpike
      expr: rate(frp_auth_failures_total[5m]) > 5
      for: 5m
      labels:
        severity: warning
        component: server
      annotations:
        summary: "Authentication failure spike detected"
        description: "May indicate misconfiguration or attack attempt"
```

### Info Alerts

#### Deployment Pending

```yaml
  - name: frp-deployments
    interval: 2m
    rules:
    - alert: FRPDeploymentPending
      expr: frp_deployments_pending > 0
      for: 10m
      labels:
        severity: info
        component: operator
      annotations:
        summary: "FRP agent deployment stuck pending"
        description: "Check resource availability and image pull status"
```

### Applying Alerts

```bash
kubectl apply -f deploy/k8s/monitoring/alerts.yaml

# Verify rules loaded
kubectl get prometheusrule -n kuberde
```

---

## Health Checks

### Server Health Check

**Kubernetes Liveness Probe** (when implemented):

```yaml
# In server Deployment spec
livenessProbe:
  httpGet:
    path: /health
    port: 80
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
```

**Manual Check**:
```bash
# Check server pod
kubectl get pods -n kuberde -l app=frp-server

# Check logs for errors
kubectl logs -n kuberde -l app=frp-server --tail=50

# Test WebSocket endpoint
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" \
  http://frp-server.kuberde.svc/ws
```

### Operator Health Check

**Liveness Probe** (controller-runtime default):

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8081
  initialDelaySeconds: 15
  periodSeconds: 20
```

**Manual Check**:
```bash
# Check operator pod
kubectl get pods -n kuberde -l app=kuberde-operator

# Check reconciliation loop is working
kubectl logs -n kuberde -l app=kuberde-operator | grep "Reconciling"

# Create test agent and verify it's created
kubectl apply -f - <<EOF
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: healthcheck-agent
  namespace: kuberde
spec:
  owner: healthcheck
  serverUrl: "ws://frp-server.kuberde.svc/ws"
  authSecret: "frp-agent-auth"
  workloadImage: "nginx:alpine"
EOF

# Check deployment created
kubectl get deployment -n kuberde user-healthcheck-healthcheck-agent

# Cleanup
kubectl delete rdeagent healthcheck-agent -n kuberde
```

### Agent Health Check

**Via Server API**:
```bash
# Get agent stats (requires OIDC token)
curl -H "Authorization: Bearer $TOKEN" \
  http://frp-server.kuberde.svc/mgmt/agents/user-testuser-agent-001

# Response should include:
# {
#   "agent_id": "user-testuser-agent-001",
#   "online": true,
#   "last_activity": "2025-12-08T12:34:56Z",
#   "bytes_received": 1048576,
#   "bytes_sent": 2097152
# }
```

**Via Kubernetes**:
```bash
# Check agent pod
kubectl get pods -n kuberde -l agent-id=user-testuser-agent-001

# Check agent logs
kubectl logs -n kuberde -l agent-id=user-testuser-agent-001 -c frp-agent

# Look for successful connection
# Should see: "Connected to server" or "Yamux session established"
```

### End-to-End Health Check

**Full user flow test**:

```bash
#!/bin/bash
# File: scripts/e2e-healthcheck.sh

set -e

echo "1. Testing OIDC authentication..."
kuberde-cli login --server-url http://frp.byai.uk || exit 1

echo "2. Testing agent connection..."
timeout 10s kuberde-cli connect --agent-id user-testuser-agent-001 < /dev/null || exit 1

echo "3. Testing HTTP proxy..."
curl -f http://user-testuser-agent-001.frp.byai.uk/ || exit 1

echo "All health checks passed!"
```

Run periodically via cron or Kubernetes CronJob:

```yaml
# File: deploy/k8s/monitoring/healthcheck-cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: frp-healthcheck
  namespace: kuberde
spec:
  schedule: "*/15 * * * *"  # Every 15 minutes
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: healthcheck
            image: kuberde-cli:latest
            command: ["/scripts/e2e-healthcheck.sh"]
          restartPolicy: OnFailure
```

---

## Log Aggregation

### Current Logging

**Server Logs**:
- WebSocket connection events
- OIDC authentication results
- HTTP proxy requests
- Yamux stream lifecycle
- Error traces

**Operator Logs**:
- Reconciliation loop events
- CRD creation/update/delete
- Deployment management
- TTL scaledown decisions
- Error conditions

**Agent Logs**:
- Server connection status
- Token refresh events
- Local target dial attempts
- Stream handling errors

### Viewing Logs

**Server**:
```bash
# Real-time logs
kubectl logs -n kuberde -l app=frp-server -f

# Last 100 lines
kubectl logs -n kuberde -l app=frp-server --tail=100

# Errors only
kubectl logs -n kuberde -l app=frp-server | grep -i error
```

**Operator**:
```bash
kubectl logs -n kuberde -l app=kuberde-operator -f
```

**Specific Agent**:
```bash
kubectl logs -n kuberde user-testuser-agent-001 -c frp-agent
kubectl logs -n kuberde user-testuser-agent-001 -c workload
```

### Elasticsearch/Loki Integration

**Fluent Bit DaemonSet** (example):

```yaml
# File: deploy/k8s/monitoring/fluent-bit.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluent-bit-config
  namespace: kuberde
data:
  fluent-bit.conf: |
    [INPUT]
        Name              tail
        Path              /var/log/containers/*frp*.log
        Parser            docker
        Tag               kube.*
        Refresh_Interval  5

    [OUTPUT]
        Name  es
        Match *
        Host  elasticsearch.logging.svc
        Port  9200
        Index frp-logs
```

**Query Examples** (Elasticsearch):
```bash
# Authentication failures
GET /frp-logs/_search
{
  "query": {
    "match": { "message": "auth failed" }
  }
}

# High latency requests
GET /frp-logs/_search
{
  "query": {
    "range": { "latency_ms": { "gte": 1000 } }
  }
}
```

### Log Retention

Recommended retention periods:
- **Production errors**: 90 days
- **Info logs**: 30 days
- **Debug logs**: 7 days
- **Audit logs**: 1 year (if compliance required)

---

## Dashboards

### Grafana Setup

```bash
# Install Grafana
kubectl apply -f https://raw.githubusercontent.com/grafana/helm-charts/main/charts/grafana/manifests/all-in-one.yaml

# Get admin password
kubectl get secret -n kuberde grafana -o jsonpath="{.data.admin-password}" | base64 -d

# Port forward
kubectl port-forward -n kuberde svc/grafana 3000:80
```

### FRP Overview Dashboard

**JSON Model** (import to Grafana):

```json
{
  "dashboard": {
    "title": "FRP System Overview",
    "panels": [
      {
        "title": "Active Agents",
        "targets": [
          { "expr": "frp_agent_connections_active" }
        ]
      },
      {
        "title": "User Connections (Rate)",
        "targets": [
          { "expr": "rate(frp_user_connections_total[5m])" }
        ]
      },
      {
        "title": "Error Rate",
        "targets": [
          { "expr": "rate(frp_connection_errors_total[5m])" }
        ]
      },
      {
        "title": "Request Latency (p95)",
        "targets": [
          { "expr": "histogram_quantile(0.95, frp_request_duration_seconds_bucket)" }
        ]
      }
    ]
  }
}
```

### Key Panels

1. **Agent Status**
   - Total agents (gauge)
   - Active agents (gauge)
   - Idle agents (gauge)
   - Agent list with last activity (table)

2. **Performance**
   - Request rate (graph)
   - Latency percentiles (graph)
   - Bytes transferred (graph)
   - Error rate (graph)

3. **Resources**
   - CPU usage (graph)
   - Memory usage (graph)
   - Goroutine count (graph)
   - Network I/O (graph)

4. **Operator**
   - Reconciliation rate (graph)
   - Reconciliation errors (graph)
   - TTL scaledowns (counter)
   - Deployment status (table)

### Pre-Built Dashboards

Dashboard templates available at:
- `deploy/k8s/monitoring/dashboards/frp-overview.json`
- `deploy/k8s/monitoring/dashboards/frp-performance.json`
- `deploy/k8s/monitoring/dashboards/frp-security.json`

---

## Troubleshooting

### No Metrics Appearing

**Check**:
1. Metrics endpoints responding:
   ```bash
   kubectl port-forward -n kuberde svc/frp-server 9090:9090
   curl http://localhost:9090/metrics
   ```

2. ServiceMonitor created and targeting correct labels:
   ```bash
   kubectl get servicemonitor -n kuberde
   kubectl describe servicemonitor frp-server -n kuberde
   ```

3. Prometheus discovering targets:
   - Open Prometheus UI → Targets
   - Look for `frp-server` and `kuberde-operator`

### High Memory Alert False Positive

**Cause**: Go garbage collector behavior (holds memory until pressure)

**Fix**: Tune `GOGC` environment variable in deployments:
```yaml
env:
- name: GOGC
  value: "50"  # More aggressive GC (default 100)
```

### Missing Agent Metrics

**Cause**: Agent not exposing metrics or ServiceMonitor not configured

**Fix**:
1. Add metrics port to agent deployment
2. Create ServiceMonitor for agents
3. Restart agent pods

### Alert Fatigue

**Symptoms**: Too many low-priority alerts

**Solutions**:
- Increase `for` duration on warning alerts
- Add `severity` labels for filtering
- Use alert inhibition rules
- Review and adjust thresholds quarterly

---

## Related Documentation

- [Operators Runbook](../guides/OPERATORS_RUNBOOK.md) - Daily operations
- [Security Guide](SECURITY.md) - Security monitoring
- [Scaling Guide](SCALING.md) - Performance metrics for scaling decisions
- [Architecture](../ARCHITECTURE.md) - System design and components

---

**Document Version**: 1.0
**Maintainer**: Platform Team
**Next Review**: 2025-03-08
