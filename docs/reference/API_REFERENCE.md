# FRP Server API Reference

**System Version**: v2.1
**Last Updated**: 2025-12-07
**Audience**: Integrators, Platform Engineers, External Tools
**Related**: [CRD_REFERENCE.md](./CRD_REFERENCE.md), [CONFIGURATION.md](./CONFIGURATION.md)

---

## Overview

The FRP Server exposes REST APIs for monitoring and managing agent connections. These APIs are used by:
- **Kubernetes Operator**: To monitor agent session activity and enforce TTL-based scaling
- **External Tools**: For integration with monitoring, load balancing, or orchestration systems
- **Admin Dashboards**: For real-time visibility into agent and user connection status

---

## Table of Contents

1. [Base URL & Authentication](#base-url--authentication)
2. [Agent Management APIs](#agent-management-apis)
3. [Health & Status APIs](#health--status-apis)
4. [Error Handling](#error-handling)
5. [Rate Limiting & Pagination](#rate-limiting--pagination)
6. [Examples](#examples)

---

## Base URL & Authentication

### Endpoint Base URL

```
http://frp-server:8080/mgmt/
```

In Kubernetes:
```
http://frp-server.kuberde.svc.cluster.local:8080/mgmt/
```

### Authentication

**All requests require Bearer token authentication**:

```bash
curl -H "Authorization: Bearer <JWT_TOKEN>" \
  http://frp-server:8080/mgmt/agents
```

### Token Sources

Tokens can be obtained from:
1. **Server's OIDC Provider**: Request from Keycloak using Client Credentials flow
2. **JWKS Validation**: Server validates tokens signed by configured JWKS endpoint
3. **Token Format**: JWT with `preferred_username` claim (for user identification)

---

## Agent Management APIs

### GET /mgmt/agents

List all connected agents and their status.

**Request**:
```bash
GET /mgmt/agents HTTP/1.1
Host: frp-server:8080
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "items": [
    {
      "id": "user-alice-dev",
      "status": {
        "phase": "Active",
        "sessions": 2,
        "connectedAt": "2025-12-07T12:00:00Z",
        "lastActivity": "2025-12-07T12:45:30Z",
        "uptime": "45m30s",
        "bytesIn": 1024000,
        "bytesOut": 2048000
      },
      "localTarget": "127.0.0.1:3000",
      "remoteProxy": "tcp://0.0.0.0:2022"
    },
    {
      "id": "user-bob-prod",
      "status": {
        "phase": "Active",
        "sessions": 1,
        "connectedAt": "2025-12-07T10:30:00Z",
        "lastActivity": "2025-12-07T12:46:15Z",
        "uptime": "2h16m",
        "bytesIn": 5120000,
        "bytesOut": 10240000
      },
      "localTarget": "127.0.0.1:22",
      "remoteProxy": "tcp://0.0.0.0:2023"
    }
  ],
  "summary": {
    "totalAgents": 2,
    "activeAgents": 2,
    "totalSessions": 3,
    "timestamp": "2025-12-07T12:46:30Z"
  }
}
```

**Status Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique agent identifier (format: `user-{owner}-{name}`) |
| `phase` | string | Connection state: `Active`, `Disconnected`, `Error` |
| `sessions` | integer | Number of active user sessions through this agent |
| `connectedAt` | timestamp | When agent first connected to server |
| `lastActivity` | timestamp | When last session was active (used for TTL calculation) |
| `uptime` | duration | How long agent has been connected |
| `bytesIn` | integer | Total bytes received from agent |
| `bytesOut` | integer | Total bytes sent to agent |

**Curl Example**:
```bash
# List all agents
curl -s -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents | jq '.'

# Count active agents
curl -s -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents | jq '.summary.activeAgents'

# Find agent for specific user
curl -s -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents | jq '.items[] | select(.id | contains("alice"))'
```

---

### GET /mgmt/agents/{id}

Get detailed status for a specific agent.

**Request**:
```bash
GET /mgmt/agents/user-alice-dev HTTP/1.1
Host: frp-server:8080
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "id": "user-alice-dev",
  "status": {
    "phase": "Active",
    "sessions": 1,
    "connectedAt": "2025-12-07T12:00:00Z",
    "lastActivity": "2025-12-07T12:46:15Z",
    "uptime": "46m15s",
    "bytesIn": 1024000,
    "bytesOut": 2048000
  },
  "localTarget": "127.0.0.1:3000",
  "remoteProxy": "tcp://0.0.0.0:2022",
  "configuration": {
    "image": "soloking/frp-agent:v2.1",
    "environment": {
      "SERVER_URL": "ws://frp-server:8080/ws",
      "LOCAL_TARGET": "127.0.0.1:3000",
      "AGENT_ID": "user-alice-dev"
    }
  },
  "sessions": [
    {
      "sessionId": "sess-12345",
      "user": "alice",
      "protocol": "ssh",
      "connectedAt": "2025-12-07T12:45:00Z",
      "bytesIn": 50000,
      "bytesOut": 100000
    }
  ]
}
```

**Response Codes**:
- `200 OK`: Agent found and status returned
- `404 Not Found`: Agent with specified ID not connected
- `401 Unauthorized`: Invalid or missing token
- `403 Forbidden`: User lacks permissions for this agent

**Curl Example**:
```bash
# Get specific agent status
curl -s -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents/user-alice-dev | jq '.status'

# Check if agent is active
curl -s -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents/user-alice-dev | \
  jq '.status.phase == "Active"'

# Get session count
curl -s -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents/user-alice-dev | \
  jq '.status.sessions'
```

---

### GET /mgmt/agents/{id}/sessions

Get detailed information about all active sessions for an agent.

**Request**:
```bash
GET /mgmt/agents/user-alice-dev/sessions HTTP/1.1
Host: frp-server:8080
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "agentId": "user-alice-dev",
  "sessions": [
    {
      "sessionId": "sess-12345",
      "user": "alice",
      "protocol": "ssh",
      "remoteAddr": "203.0.113.25:54321",
      "connectedAt": "2025-12-07T12:45:00Z",
      "lastActivity": "2025-12-07T12:46:15Z",
      "duration": "1m15s",
      "bytesIn": 50000,
      "bytesOut": 100000
    }
  ],
  "summary": {
    "totalSessions": 1,
    "activeSessions": 1,
    "totalBytesTransferred": 150000
  }
}
```

**Curl Example**:
```bash
# Check if agent has active sessions
curl -s -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents/user-alice-dev/sessions | \
  jq '.summary.activeSessions'
```

---

## Health & Status APIs

### GET /health

Server health check endpoint (no authentication required).

**Request**:
```bash
GET /health HTTP/1.1
Host: frp-server:8080
```

**Response** (200 OK):
```json
{
  "status": "healthy",
  "version": "v2.1",
  "uptime": "72h15m30s",
  "timestamp": "2025-12-07T12:46:30Z",
  "components": {
    "database": "ok",
    "oidc": "ok",
    "ws": "ok"
  }
}
```

**Curl Example**:
```bash
# Basic health check
curl http://frp-server:8080/health | jq '.'

# Use in Kubernetes liveness probe
curl -f http://localhost:8080/health || exit 1
```

---

### GET /metrics

Prometheus-compatible metrics endpoint (no authentication required).

**Request**:
```bash
GET /metrics HTTP/1.1
Host: frp-server:8080
```

**Response** (200 OK):
```
# HELP frp_agents_total Total number of agents
# TYPE frp_agents_total gauge
frp_agents_total 5

# HELP frp_sessions_active Number of active user sessions
# TYPE frp_sessions_active gauge
frp_sessions_active 3

# HELP frp_bytes_transferred Total bytes transferred
# TYPE frp_bytes_transferred counter
frp_bytes_transferred 1024000

# ... (more metrics)
```

**Common Metrics**:

| Metric | Type | Description |
|--------|------|-------------|
| `frp_agents_total` | gauge | Total connected agents |
| `frp_agents_active` | gauge | Agents with active sessions |
| `frp_sessions_active` | gauge | Total active user sessions |
| `frp_bytes_in_total` | counter | Total bytes received |
| `frp_bytes_out_total` | counter | Total bytes sent |
| `frp_agent_uptime_seconds` | gauge | Per-agent connection uptime |
| `frp_request_duration_seconds` | histogram | Request latency |

**Curl Example**:
```bash
# Scrape metrics for Prometheus
curl http://frp-server:8080/metrics | grep frp_agents

# Monitor active sessions
curl http://frp-server:8080/metrics | grep frp_sessions_active
```

---

## Error Handling

### Error Response Format

All error responses use this format:

```json
{
  "error": "Agent not found",
  "code": "AGENT_NOT_FOUND",
  "status": 404,
  "timestamp": "2025-12-07T12:46:30Z",
  "details": {
    "agentId": "user-unknown-dev",
    "message": "No agent with ID 'user-unknown-dev' is currently connected"
  }
}
```

### Common Error Codes

| HTTP Status | Code | Meaning | Solution |
|-------------|------|---------|----------|
| 400 | `BAD_REQUEST` | Invalid request format | Check request syntax and parameters |
| 401 | `UNAUTHORIZED` | Missing or invalid token | Provide valid JWT token in Authorization header |
| 403 | `FORBIDDEN` | User lacks permissions | Verify user has access to requested agent |
| 404 | `AGENT_NOT_FOUND` | Agent not connected | Verify agent ID and check if agent is running |
| 429 | `RATE_LIMITED` | Too many requests | Wait before retrying |
| 500 | `INTERNAL_ERROR` | Server error | Check server logs and retry |
| 503 | `SERVICE_UNAVAILABLE` | Server not ready | Wait for server to be ready |

### Example Error Handling

```bash
#!/bin/bash
RESPONSE=$(curl -s -w "\n%{http_code}" \
  -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents/user-alice-dev)

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -n -1)

if [ "$HTTP_CODE" = "200" ]; then
  echo "Success: $(echo $BODY | jq '.status.phase')"
elif [ "$HTTP_CODE" = "404" ]; then
  echo "Agent not found"
elif [ "$HTTP_CODE" = "401" ]; then
  echo "Invalid token"
else
  echo "Error ($HTTP_CODE): $(echo $BODY | jq '.error')"
fi
```

---

## Rate Limiting & Pagination

### Rate Limiting

The server may implement rate limiting on API endpoints:

| Endpoint | Limit | Window | Headers |
|----------|-------|--------|---------|
| `/mgmt/agents` | 100 | 1 minute | `X-RateLimit-Limit`, `X-RateLimit-Remaining` |
| `/mgmt/agents/{id}` | 200 | 1 minute | `X-RateLimit-Limit`, `X-RateLimit-Remaining` |

**Rate Limit Headers**:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1702027830
```

**When Rate Limited** (429):
```json
{
  "error": "Rate limit exceeded",
  "code": "RATE_LIMITED",
  "status": 429,
  "retryAfter": 30
}
```

### Pagination

Large result sets are paginated:

```bash
# First page (default: 20 items per page)
curl -H "Authorization: Bearer $TOKEN" \
  "http://frp-server:8080/mgmt/agents?page=1&pageSize=50"
```

**Pagination Parameters**:

| Parameter | Type | Default | Max | Description |
|-----------|------|---------|-----|-------------|
| `page` | integer | 1 | N/A | Page number (1-based) |
| `pageSize` | integer | 20 | 100 | Items per page |
| `sort` | string | `id` | N/A | Sort field: `id`, `lastActivity`, `sessions` |
| `order` | string | `asc` | N/A | Sort order: `asc`, `desc` |

**Paginated Response**:
```json
{
  "items": [...],
  "pagination": {
    "page": 1,
    "pageSize": 20,
    "totalItems": 150,
    "totalPages": 8,
    "hasNext": true,
    "hasPrev": false
  }
}
```

---

## Examples

### Example 1: Monitor Agent for TTL Enforcement

Kubernetes Operator TTL monitor loop:

```bash
#!/bin/bash
AGENT_ID="user-alice-dev"
TOKEN="$KEYCLOAK_TOKEN"

# Check if agent has active sessions
SESSIONS=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents/$AGENT_ID | \
  jq '.status.sessions')

echo "Agent: $AGENT_ID, Active Sessions: $SESSIONS"

# Get lastActivity for TTL calculation
LAST_ACTIVITY=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents/$AGENT_ID | \
  jq -r '.status.lastActivity')

echo "Last Activity: $LAST_ACTIVITY"

# Calculate TTL expiry
TTL_SECONDS=300  # 5 minutes
EXPIRES_AT=$(date -d "$LAST_ACTIVITY + $TTL_SECONDS seconds" +%s)
NOW=$(date +%s)

if [ $NOW -gt $EXPIRES_AT ]; then
  echo "TTL Expired! Scale down deployment"
  kubectl scale deployment user-alice-dev --replicas=0
else
  REMAINING=$((EXPIRES_AT - NOW))
  echo "TTL expires in ${REMAINING}s"
fi
```

### Example 2: Alert on Session Spike

Monitoring tool to detect unusual session activity:

```bash
#!/bin/bash
THRESHOLD=10  # Alert if > 10 concurrent sessions
TOKEN="$KEYCLOAK_TOKEN"

ACTIVE_SESSIONS=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents | \
  jq '[.items[].status.sessions] | add')

echo "Total active sessions: $ACTIVE_SESSIONS"

if [ "$ACTIVE_SESSIONS" -gt "$THRESHOLD" ]; then
  echo "ALERT: High session count ($ACTIVE_SESSIONS > $THRESHOLD)"

  # List top agents by session count
  curl -s -H "Authorization: Bearer $TOKEN" \
    http://frp-server:8080/mgmt/agents | \
    jq '.items | sort_by(.status.sessions) | reverse | .[0:5] | .[] | "\(.id): \(.status.sessions) sessions"'
fi
```

### Example 3: Integration with External Load Balancer

Keeping load balancer in sync with agent availability:

```bash
#!/bin/bash
TOKEN="$KEYCLOAK_TOKEN"
LOAD_BALANCER_API="https://lb.example.com/api"

# Get all active agents
AGENTS=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents | \
  jq '.items[] | select(.status.phase == "Active")')

while IFS= read -r agent; do
  AGENT_ID=$(echo "$agent" | jq -r '.id')
  REMOTE_PROXY=$(echo "$agent" | jq -r '.remoteProxy')

  # Register with load balancer
  curl -X POST "$LOAD_BALANCER_API/register" \
    -H "Content-Type: application/json" \
    -d "{\"id\": \"$AGENT_ID\", \"endpoint\": \"$REMOTE_PROXY\"}"

  echo "Registered: $AGENT_ID at $REMOTE_PROXY"
done < <(echo "$AGENTS")
```

### Example 4: Prometheus Scrape Config

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'frp-server'
    static_configs:
      - targets: ['frp-server:8080']
    metrics_path: '/metrics'
    scrape_interval: 30s
```

---

## Testing the API

### Using curl

```bash
# Get token
TOKEN=$(curl -s -X POST http://keycloak:8080/token \
  -d "client_id=frp-admin&client_secret=secret&grant_type=client_credentials" | \
  jq -r '.access_token')

# List agents
curl -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents

# Get agent detail
curl -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents/user-alice-dev

# Check health
curl http://frp-server:8080/health
```

### Using Postman

1. Create collection: "FRP API"
2. Add authentication:
   - Type: Bearer Token
   - Token: `{{token}}`
3. Create pre-request script to refresh token:
   ```javascript
   if (!pm.environment.get('token') || pm.environment.get('token_expires') < Date.now()) {
     const request = {
       method: 'POST',
       url: 'http://keycloak:8080/token',
       body: {...}
     };
     pm.sendRequest(request, (err, response) => {
       pm.environment.set('token', response.json().access_token);
       pm.environment.set('token_expires', Date.now() + 3600000);
     });
   }
   ```
4. Add requests:
   - GET `/mgmt/agents`
   - GET `/mgmt/agents/{{agentId}}`
   - GET `/mgmt/agents/{{agentId}}/sessions`

---

## SDK & Client Libraries

### Go

```go
import "github.com/kuberde/sdk-go"

client := sdk.NewClient("http://frp-server:8080", token)
agents, err := client.ListAgents(ctx)
if err != nil {
  log.Fatal(err)
}

for _, agent := range agents {
  fmt.Printf("%s: %d sessions\n", agent.ID, agent.Status.Sessions)
}
```

### Python

```python
import requests

headers = {"Authorization": f"Bearer {token}"}
response = requests.get("http://frp-server:8080/mgmt/agents", headers=headers)
agents = response.json()

for agent in agents['items']:
    print(f"{agent['id']}: {agent['status']['sessions']} sessions")
```

### JavaScript/Node.js

```javascript
const axios = require('axios');

const client = axios.create({
  baseURL: 'http://frp-server:8080',
  headers: { 'Authorization': `Bearer ${token}` }
});

const agents = await client.get('/mgmt/agents');
agents.data.items.forEach(agent => {
  console.log(`${agent.id}: ${agent.status.sessions} sessions`);
});
```

---

## Troubleshooting API Issues

### 401 Unauthorized

**Cause**: Token invalid or expired
**Solution**:
```bash
# Request new token
TOKEN=$(curl -s -X POST http://keycloak:8080/token \
  -d "client_id=frp&client_secret=secret&grant_type=client_credentials" | \
  jq -r '.access_token')

# Verify token
curl -H "Authorization: Bearer $TOKEN" http://frp-server:8080/health
```

### 404 Not Found

**Cause**: Agent not connected
**Solution**:
```bash
# Check if agent is running
kubectl get pods -n kuberde -l instance=user-alice-dev

# Check agent logs
kubectl logs -n kuberde <agent-pod>

# Verify agent is registered on server
curl -H "Authorization: Bearer $TOKEN" \
  http://frp-server:8080/mgmt/agents | jq '.items[].id'
```

### Connection Refused

**Cause**: Server not accessible
**Solution**:
```bash
# Verify server is running
kubectl get pod -n kuberde -l app=frp-server

# Check service
kubectl get svc -n kuberde frp-server

# Test connectivity
kubectl run -it debug --image=curlimages/curl --restart=Never -- \
  curl http://frp-server:8080/health
```

---

**Document Version**: 1.0
**Last Reviewed**: 2025-12-07
**Next Review**: 2025-12-14
