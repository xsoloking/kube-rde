# Multi-Service Workload Design Document

**Date:** 2025-12-09
**Status:** Design Complete
**Scope:** Support SSH, File Server, Coder Server in a single Kubernetes workload

---

## Executive Summary

Enable a single RDEAgent CRD to expose multiple services (TCP and HTTP) running in the same Kubernetes workload, reducing operational complexity and resource overhead.

**Key Design Decisions:**
- One RDEAgent CRD → One Kubernetes Deployment → One Pod with multiple services
- Single Agent process → One WebSocket connection → Multiple Yamux streams (one per service)
- Explicit service definition in CRD spec with auto-generated Agent IDs
- TCP services use port-based routing; HTTP services use subdomain-based routing

---

## Architecture Overview

### Current (Single-Service) Model
```
RDEAgent CR (ssh-server)
    ↓
Deployment (ssh-server)
    ↓
Pod (openssh-server on port 2222)
    ↓
Agent Process (localTarget: localhost:2222)
    ↓
Server (listens on :2222)
```

### New (Multi-Service) Model
```
RDEAgent CR (my-workload)
    ↓
Deployment (my-workload)
    ↓
Pod (SSH 2222 + Files 3000 + Coder 8080)
    ↓
Agent Process (manages 3 services)
    ↓
Server (routes to correct service)
```

---

## Component Changes

### 1. CRD Schema Updates

**File:** `deploy/k8s/01-crd.yaml`

Remove single `localTarget` field, add `services` array:

```yaml
apiVersion: frp.byai.io/v1beta1
kind: RDEAgent
metadata:
  name: my-workload
  namespace: kuberde
spec:
  owner: "alice"
  serverUrl: "ws://frp-server.kuberde.svc:80/ws"

  # New: Define multiple services
  services:
    - name: "ssh"
      port: 2222
      protocol: "TCP"

    - name: "files"
      port: 3000
      protocol: "HTTP"

    - name: "coder"
      port: 8080
      protocol: "HTTP"

  # Single workload container with all services
  workloadContainer:
    image: "my-multi-service:latest"
    ports:
      - containerPort: 2222
        name: ssh
      - containerPort: 3000
        name: files
      - containerPort: 8080
        name: coder
```

**Schema Definition:**
```go
// Service defines a single service in the workload
type Service struct {
  Name     string `json:"name"`      // Unique within the workload (ssh, files, coder)
  Port     int    `json:"port"`      // Local container port
  Protocol string `json:"protocol"`  // TCP or HTTP
}

// RDEAgentSpec extended
type RDEAgentSpec struct {
  Services []Service `json:"services"`
  // ... other fields remain the same
}
```

---

### 2. Agent Implementation Changes

**File:** `cmd/agent/main.go`

**Key Changes:**

1. **Configuration Loading:**
   - Remove single `localTarget` environment variable
   - Add `SERVICES_CONFIG` environment variable (JSON format)
   - Operator injects services list into Agent

2. **Services Configuration Structure:**
```go
type ServiceConfig struct {
  Name     string `json:"name"`
  Port     int    `json:"port"`
  Protocol string `json:"protocol"`
}

type AgentConfig struct {
  Services []ServiceConfig `json:"services"`
}
```

3. **Initialization:**
```go
func init() {
  // Load services from SERVICES_CONFIG env var
  servicesJSON := os.Getenv("SERVICES_CONFIG")
  err := json.Unmarshal([]byte(servicesJSON), &config)
  // Build service map for quick lookup by name
  serviceMap := make(map[string]*ServiceConfig)
  for _, svc := range config.Services {
    serviceMap[svc.Name] = &svc
  }
}
```

4. **Stream Handling with Service Routing:**
```go
func handleStream(stream net.Conn, serviceName string) {
  defer stream.Close()

  // Find service in serviceMap
  svc, exists := serviceMap[serviceName]
  if !exists {
    log.Printf("Unknown service: %s", serviceName)
    return
  }

  // Connect to local service
  localConn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", svc.Port))
  if err != nil {
    log.Printf("Failed to connect to service %s on port %d: %v", serviceName, svc.Port, err)
    return
  }
  defer localConn.Close()

  // Bidirectional copy (unchanged)
  go io.Copy(localConn, stream)
  io.Copy(stream, localConn)
}

// Main loop (modified)
func main() {
  // ... setup code ...

  for {
    stream, err := session.Accept()
    if err != nil {
      log.Printf("Session accept failed: %v", err)
      break
    }

    // Read service name from stream first message
    serviceName := readServiceName(stream)
    go handleStream(stream, serviceName)
  }
}
```

5. **Helper to read service identifier from stream:**
```go
func readServiceName(stream net.Conn) string {
  // First message: service name (e.g., "ssh\n")
  buf := make([]byte, 64)
  n, _ := stream.Read(buf)
  return strings.TrimSpace(string(buf[:n]))
}
```

---

### 3. Server Implementation Changes

**File:** `cmd/server/main.go`

**Key Changes:**

1. **TCP Service Port Mapping Registration:**
```go
type TCPPortMapping struct {
  AgentID   string
  Service   string
}

var portToAgent map[int]TCPPortMapping // port 2222 -> (agent-id, service-name)
var agentToServices map[string][]ServiceInfo // agent-id -> [services]

// Called by Operator when service registered
func registerTCPService(agentID string, serviceName string, port int) error {
  portToAgent[port] = TCPPortMapping{AgentID: agentID, Service: serviceName}
  log.Printf("Registered TCP service: %s -> port %d", agentID, port)
}
```

2. **TCP Connection Handler (modified):**
```go
func handleTCPConnection(conn net.Conn, port int) {
  defer conn.Close()

  // Look up which agent and service
  mapping, exists := portToAgent[port]
  if !exists {
    log.Printf("No service registered for port %d", port)
    return
  }

  // Get agent session
  session := getAgentSession(mapping.AgentID)
  if session == nil {
    log.Printf("Agent %s not connected", mapping.AgentID)
    return
  }

  // Open Yamux stream
  stream, err := session.Open()
  if err != nil {
    log.Printf("Failed to open stream: %v", err)
    return
  }
  defer stream.Close()

  // Send service identifier as first message
  stream.Write([]byte(mapping.Service + "\n"))

  // Bridge connection
  go io.Copy(stream, conn)
  io.Copy(conn, stream)
}
```

3. **HTTP Service Handler (modified):**
```go
func handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
  // Parse hostname: {serviceName}.user-{owner}-{workload}.frp.byai.uk
  // Example: files.user-alice-myworkload.frp.byai.uk
  hostname := r.Host
  parts := strings.Split(hostname, ".")

  if len(parts) < 3 {
    http.Error(w, "Invalid hostname", 400)
    return
  }

  serviceName := parts[0]
  agentID := strings.Join(parts[1:len(parts)-2], "-") // Extract user-alice-myworkload

  session := getAgentSession(agentID)
  if session == nil {
    http.Error(w, "Agent not found", 502)
    return
  }

  stream, err := session.Open()
  if err != nil {
    http.Error(w, "Failed to open stream", 502)
    return
  }
  defer stream.Close()

  // Send service identifier
  stream.Write([]byte(serviceName + "\n"))

  // Use RoundTripper to bridge HTTP
  resp, err := httpConn.RoundTrip(r)
  // ... handle response ...
}
```

---

### 4. Operator Implementation Changes

**File:** `cmd/operator/main.go`

**Key Changes:**

1. **Extract Services from CRD:**
```go
func extractServices(cr *unstructured.Unstructured) ([]Service, error) {
  services, _, err := unstructured.NestedSlice(cr.Object, "spec", "services")
  if err != nil {
    return nil, err
  }

  var result []Service
  for _, svcInterface := range services {
    svcMap := svcInterface.(map[string]interface{})
    service := Service{
      Name:     svcMap["name"].(string),
      Port:     int(svcMap["port"].(float64)),
      Protocol: svcMap["protocol"].(string),
    }
    result = append(result, service)
  }
  return result, nil
}
```

2. **Build Services Configuration JSON:**
```go
func buildServicesConfig(services []Service) (string, error) {
  config := map[string]interface{}{
    "services": services,
  }
  jsonBytes, err := json.Marshal(config)
  return string(jsonBytes), err
}
```

3. **Generate Agent IDs for each service:**
```go
func generateAgentIDs(owner, crName string, services []Service) []string {
  var agentIDs []string
  for _, svc := range services {
    agentID := fmt.Sprintf("user-%s-%s-%s", owner, crName, svc.Name)
    agentIDs = append(agentIDs, agentID)
  }
  return agentIDs
}
```

4. **Reconcile Deployment with services:**
```go
func (c *Controller) reconcileDeployment(cr *unstructured.Unstructured) error {
  spec, _, _ := unstructured.NestedMap(cr.Object, "spec")
  owner := spec["owner"].(string)
  crName := cr.GetName()
  namespace := cr.GetNamespace()

  // Extract services
  services, err := extractServices(cr)
  if err != nil {
    return err
  }

  // Build services config
  servicesConfig, err := buildServicesConfig(services)
  if err != nil {
    return err
  }

  // Create/update Deployment with SERVICES_CONFIG env var
  deployment := buildDeployment(cr, services, servicesConfig)

  // Register services with Server
  for _, svc := range services {
    agentID := fmt.Sprintf("user-%s-%s-%s", owner, crName, svc.Name)

    if svc.Protocol == "TCP" {
      // Dynamically allocate port or use predefined
      port := 2000 + hash(agentID) % 1000
      err := c.registerTCPServiceWithServer(agentID, svc.Name, port)
      if err != nil {
        log.Printf("Failed to register TCP service: %v", err)
      }
    } else if svc.Protocol == "HTTP" {
      // Register subdomain mapping
      err := c.registerHTTPServiceWithServer(agentID, svc.Name)
      if err != nil {
        log.Printf("Failed to register HTTP service: %v", err)
      }
    }
  }

  // Deploy Kubernetes Deployment
  _, err = c.k8sClient.AppsV1().Deployments(namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
  if err != nil && !errors.IsAlreadyExists(err) {
    return err
  }

  return nil
}
```

5. **Register with Server:**
```go
func (c *Controller) registerTCPServiceWithServer(agentID, serviceName string, port int) error {
  body := map[string]interface{}{
    "agentID":     agentID,
    "service":     serviceName,
    "port":        port,
  }
  jsonBody, _ := json.Marshal(body)

  resp, err := c.httpClient.Post(
    fmt.Sprintf("http://frp-server/mgmt/services/tcp"),
    "application/json",
    bytes.NewReader(jsonBody),
  )
  // ... handle response ...
}
```

---

## Traffic Flow Examples

### Example 1: SSH (TCP) Access
```
User connects to: frp-server:2222
    ↓
Server looks up portToAgent[2222] → user-alice-myworkload-ssh
    ↓
Server calls session.Open() on Agent connection
    ↓
Server sends "ssh\n" to stream
    ↓
Agent reads service name, routes to localhost:2222
    ↓
Agent connects to local SSH server
    ↓
User ↔ SSH Server (bidirectional io.Copy)
```

### Example 2: File Server (HTTP) Access
```
User connects to: files.user-alice-myworkload.frp.byai.uk
    ↓
Server parses hostname: serviceName=files, agentID=user-alice-myworkload
    ↓
Server opens Yamux stream
    ↓
Server sends "files\n" to stream
    ↓
Agent reads service name, routes to localhost:3000
    ↓
Agent connects to local file server
    ↓
User ↔ File Server (HTTP request/response via io.Copy)
```

---

## Data Structures

### CRD Definition
```go
type Service struct {
  Name     string `json:"name"`      // ssh, files, coder
  Port     int    `json:"port"`      // 2222, 3000, 8080
  Protocol string `json:"protocol"`  // TCP or HTTP
}

type RDEAgentSpec struct {
  Owner        string    `json:"owner"`
  ServerUrl    string    `json:"serverUrl"`
  AuthSecret   string    `json:"authSecret"`
  Services     []Service `json:"services"`  // NEW
  WorkloadContainer WorkloadContainer `json:"workloadContainer"`
  TTL          string    `json:"ttl"`
}
```

### Agent ID Format
```
user-{owner}-{crName}-{serviceName}

Examples:
  user-alice-myworkload-ssh
  user-alice-myworkload-files
  user-alice-myworkload-coder
```

### Yamux Stream Protocol
```
First message (service identifier): "serviceName\n"
Subsequent messages: raw TCP/HTTP data
```

---

## Implementation Checklist

- [ ] Update CRD schema in `deploy/k8s/01-crd.yaml`
- [ ] Modify Agent to parse and route multiple services
- [ ] Extend Server to handle TCP port and HTTP subdomain routing
- [ ] Update Operator to extract services and register with Server
- [ ] Create example multi-service CRD manifest
- [ ] Test single service → multiple services migration
- [ ] Performance testing with 3+ services per workload
- [ ] Documentation and deployment guide

---

## Migration Path

**Phase 1:** Deploy multi-service code alongside existing single-service CRDs (no breaking changes needed since product is pre-launch)

**Phase 2:** Convert existing single-service CRDs to multi-service format

**Phase 3:** Decommission single-service code paths

---

## Testing Strategy

### Scenario 1: Basic Multi-Service
- Deploy RDEAgent with SSH + Files + Coder
- Verify each service is accessible independently
- Verify port isolation (SSH on 2222, Files on 3000, etc.)

### Scenario 2: Service Failure Resilience
- Stop one service (e.g., File Server)
- Verify other services (SSH, Coder) continue working
- Restart failed service and re-register

### Scenario 3: Multi-Workload Coexistence
- Deploy 5 RDEAgents, each with 3 services
- Verify 15 services are all independently routable
- Verify no port conflicts

### Scenario 4: Performance
- Compare single-service vs multi-service Agent memory/CPU
- Verify single WebSocket connection handles all services efficiently
- Load test with concurrent connections to different services

---

## Summary

This design enables efficient, declarative multi-service workload exposure through FRP by:
1. Extending CRD to define multiple services
2. Simplifying Agent to route based on service identifier
3. Updating Server to manage port/subdomain mappings
4. Coordinating through Operator

**Benefits:**
- Reduced operational complexity (one CRD instead of three)
- Single Agent process (lower resource overhead)
- Clear service definitions (declarative)
- Maintains existing authorization model

