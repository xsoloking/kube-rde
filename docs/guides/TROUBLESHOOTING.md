# FRP Operator Troubleshooting Guide

**System Version**: v2.1
**Last Updated**: 2025-12-07
**Audience**: SRE/Support Teams, Platform Engineers
**Related**: [OPERATORS_RUNBOOK.md](./OPERATORS_RUNBOOK.md) for daily operations

---

## Table of Contents

1. [Quick Diagnostics](#quick-diagnostics)
2. [Failure Mode Diagnosis](#failure-mode-diagnosis)
3. [Common Issues & Solutions](#common-issues--solutions)
4. [Deep Dive: Root Cause Analysis](#deep-dive-root-cause-analysis)
5. [Performance & Resource Issues](#performance--resource-issues)
6. [Network & Connectivity Issues](#network--connectivity-issues)
7. [Data & State Consistency Issues](#data--state-consistency-issues)
8. [Recovery Procedures](#recovery-procedures)

---

## Quick Diagnostics

### System Health Check Script

Run this first to assess overall health:

```bash
#!/bin/bash
echo "=== FRP Operator System Health Check ==="
echo ""

# Check 1: Operator pod status
echo "[1] Operator Pod Status:"
kubectl get pods -n kuberde -l app=kuberde-operator -o wide
echo ""

# Check 2: Operator logs for errors
echo "[2] Recent Operator Errors (last 50 lines):"
kubectl logs -n kuberde -l app=kuberde-operator --tail=50 | grep -i "error\|fatal\|panic" || echo "No errors found ✓"
echo ""

# Check 3: All agents status
echo "[3] Agent CRD Status:"
kubectl get rdeagent -n kuberde -o wide
echo ""

# Check 4: Deployment status
echo "[4] Agent Deployment Status:"
kubectl get deployment -n kuberde -l app=frp-agent --sort-by=.metadata.creationTimestamp
echo ""

# Check 5: Pod status
echo "[5] Agent Pod Status (issues only):"
kubectl get pods -n kuberde -l app=frp-agent --field-selector=status.phase!=Running -o wide || echo "All running ✓"
echo ""

# Check 6: Server pod
echo "[6] Server Pod Status:"
kubectl get pods -n kuberde -l app=frp-server -o wide
echo ""

echo "=== Health Check Complete ==="
```

### Key Status Fields to Check

For **RDEAgent CRs**:
```bash
kubectl get rdeagent -n kuberde -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\t"}{.status.activeReplicas}{"\t"}{.status.lastActivity}{"\n"}{end}'
```

For **Deployments**:
```bash
kubectl get deployment -n kuberde -l app=frp-agent -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.replicas}{"\t"}{.status.readyReplicas}{"\t"}{.spec.template.metadata.labels.pod-template-hash}{"\n"}{end}'
```

---

## Failure Mode Diagnosis

### Diagnosis Decision Tree

```
Agent not working?
├── [No pods running]
│   ├── Replicas = 0?
│   │   ├── Yes → TTL enforced (expected, session-based auto-recovery)
│   │   └── No → [Issue: Pods not starting]
│   └── [Issue: Pod startup failure]
├── [Pods in CrashLoopBackOff]
│   └── [Issue: Container crash cycle]
├── [Pods Running but not connecting]
│   └── [Issue: Agent connection failure]
├── [Replicas constantly changing]
│   └── [Issue: Unstable reconciliation]
└── [Replicas cycling every N seconds]
    └── [Issue: Pod rebuild loops]
```

### Identify the Issue

**Step 1: Check operator pod**
```bash
kubectl get pod -n kuberde -l app=kuberde-operator -o jsonpath='{.items[0].status.phase}'
```
- **Running**: Operator OK, check agent
- **NotReady/Pending/CrashLoopBackOff**: [Go to Section: Operator Pod Issues](#operator-pod-issues)

**Step 2: Check agent CR status**
```bash
kubectl get rdeagent -n kuberde <agent-name> -o jsonpath='{.status.phase}'
```
- **Synced**: Deployment created, check deployment
- **Pending**: Waiting for deployment creation
- **Failed**: Check CR or operator logs

**Step 3: Check deployment and pods**
```bash
kubectl get deployment -n kuberde user-testuser-<agent-name> -o wide
kubectl get pods -n kuberde -l instance=user-testuser-<agent-name> -o wide
```
- **Replicas != ReadyReplicas**: Pods not starting
- **Replicas = 0**: TTL enforced or scaled down
- **Replicas changing frequently**: Unstable reconciliation

---

## Common Issues & Solutions

### Issue: Operator Pod CrashLoopBackOff

**Symptom**: Operator continuously restarting, agents not reconciled

**Diagnosis**:
```bash
# Check logs
kubectl logs -n kuberde -l app=kuberde-operator --tail=100

# Check crash reason
kubectl get pod -n kuberde -l app=kuberde-operator -o jsonpath='{.items[0].status.containerStatuses[0].lastState.terminated.reason}'
```

**Common Causes & Solutions**:

| Cause | Symptom | Solution |
|-------|---------|----------|
| RBAC permissions missing | `forbidden` error | [See RBAC section](#rbac-permissions-missing) |
| Kubernetes API timeout | Connection timeout | Increase API server resources or reduce reconciliation frequency |
| Memory leak | Memory usage 1-2GB+ | Restart operator: `kubectl rollout restart deployment/kuberde-operator -n kuberde` |
| CRD missing | `no kind RDEAgent` | `kubectl apply -f deploy/k8s/03-operator.yaml` |
| Conflicting controller | Multiple operators reconciling | Ensure only one operator deployment running |

**Recovery**:
```bash
# Restart operator
kubectl rollout restart deployment/kuberde-operator -n kuberde
kubectl rollout status deployment/kuberde-operator -n kuberde -w

# Verify recovery
kubectl logs -n kuberde -l app=kuberde-operator --tail=20 | grep "Starting reconciliation"
```

### Issue: Agent Pod Stuck in CrashLoopBackOff

**Symptom**: Pods constantly restarting, replicas > 0 but no running pods

**Diagnosis**:
```bash
# View pod status
kubectl get pods -n kuberde -l instance=user-testuser-<agent-name> -o wide

# Check logs
kubectl logs -n kuberde -l instance=user-testuser-<agent-name> --previous

# Check events
kubectl describe pod -n kuberde <pod-name>
```

**Common Causes & Solutions**:

| Cause | Log Message | Solution |
|-------|-------------|----------|
| Invalid credentials | `auth/oidc:` error | Verify Secret mounted correctly: `kubectl get secret -n kuberde user-testuser-<agent-name>` |
| Server unreachable | `connection refused` | Check server running: `kubectl get svc -n kuberde frp-server` |
| Invalid LOCAL_TARGET | `connection refused` | Verify target service exists and is reachable from agent pod |
| Image not found | `image not found` | Check agent image in CR: `kubectl get rdeagent -n kuberde <agent-name> -o jsonpath='{.spec.image}'` |
| Bad deployment spec | Various | Check CR for syntax errors: `kubectl get rdeagent -n kuberde <agent-name> -o yaml` |

**Recovery**:
```bash
# Option 1: Fix the CR
kubectl patch rdeagent -n kuberde <agent-name> -p '{"spec":{"image":"soloking/frp-agent:v2.1"}}'

# Option 2: Delete and recreate deployment
kubectl delete deployment -n kuberde user-testuser-<agent-name>
kubectl rollout status deployment/kuberde-operator -n kuberde -w  # Wait for reconciliation

# Check logs after recovery
kubectl logs -n kuberde -l instance=user-testuser-<agent-name> -f
```

### Issue: Agents Rebuild Every 20-50 Seconds

**Symptom**: Pod template hash constantly changing, pods restarting frequently
**Root Cause**: False-positive change detection in reconciliation loop
**Status**: ✅ Fixed in v2.1 (Phase 4.1)

**Verify Fix**:
```bash
# Check operator version
kubectl get deployment -n kuberde kuberde-operator -o jsonpath='{.spec.template.spec.containers[0].image}'
# Should be: soloking/kuberde-operator:v2.1 or later

# Check logs for change detection
kubectl logs -n kuberde -l app=kuberde-operator --tail=100 | grep "deploymentSpecChanged"
# Should see "deploymentSpecChanged returned: false" (not true)

# Monitor pod stability
kubectl get deployment -n kuberde user-testuser-<agent-name> -w -o jsonpath='{.spec.template.metadata.labels.pod-template-hash}{"\n"}'
# Hash should remain stable (no constant changes)
```

**If still occurring** (shouldn't be with v2.1):
1. Verify operator image is v2.1 or later
2. Check operator logs for unusual reconciliation patterns
3. Run Extended Stability Test [below](#extended-stability-test)
4. [Contact support with logs](#when-to-escalate)

### Issue: Replicas Stuck at Non-Zero After TTL Expiry

**Symptom**: TTL expired but replicas never scale down to 0
**Root Cause**: TTL enforcer loop not running or operator restart preventing state update

**Diagnosis**:
```bash
# Check lastActivity
kubectl get rdeagent -n kuberde <agent-name> -o jsonpath='{.status.lastActivity}'

# Calculate if TTL expired (example: 5m TTL = 300 seconds)
# Current time - lastActivity > TTL seconds?

# Check deployment replicas
kubectl get deployment -n kuberde user-testuser-<agent-name> -o jsonpath='{.spec.replicas}'

# Check operator logs for TTL enforcer
kubectl logs -n kuberde -l app=kuberde-operator --tail=50 | grep -i "ttl\|scale.*down"
```

**Solutions**:

1. **Manual scale-down**:
```bash
kubectl scale deployment -n kuberde user-testuser-<agent-name> --replicas=0
```

2. **Restart TTL enforcer** (via operator restart):
```bash
kubectl rollout restart deployment/kuberde-operator -n kuberde
```

3. **Check TTL configuration**:
```bash
kubectl get rdeagent -n kuberde <agent-name> -o jsonpath='{.spec.ttlSecondsAfterFinished}'
# Should not be null or 0
```

### Issue: Agent Can't Connect to Server

**Symptom**: Agent pod running but no agent sessions on server

**Diagnosis**:
```bash
# Check agent logs
kubectl logs -n kuberde -l instance=user-testuser-<agent-name> | grep -i "server\|connect\|ws://"

# Check server is accepting connections
kubectl exec -n kuberde -it <server-pod> -- curl http://localhost:8080/health || echo "Server not responding"

# Check network connectivity from agent pod
kubectl exec -n kuberde -it <agent-pod> -- curl -v http://frp-server:8080/health

# Check if agent registered on server
kubectl exec -n kuberde -it <server-pod> -- curl http://localhost:8080/mgmt/agents | jq '.'
```

**Solutions**:

1. **Verify SERVER_URL environment variable**:
```bash
kubectl get rdeagent -n kuberde <agent-name> -o jsonpath='{.spec.env[?(@.name=="SERVER_URL")].value}'
# Should be ws://frp-server:8080/ws (or external hostname)
```

2. **Check server service**:
```bash
kubectl get svc -n kuberde frp-server
kubectl get endpoints -n kuberde frp-server
```

3. **Verify credentials**:
```bash
# Check Secret exists
kubectl get secret -n kuberde user-testuser-<agent-name>

# Verify token is valid
kubectl get secret -n kuberde user-testuser-<agent-name> -o jsonpath='{.data.token}' | base64 -d
```

### Issue: RBAC Permissions Missing

**Symptom**: `forbidden` error when operator tries to create/update deployments

**Diagnosis**:
```bash
# Check current RBAC
kubectl get rolebinding -n kuberde kuberde-operator
kubectl get clusterrolebinding | grep kuberde-operator

# Test RBAC permission
kubectl auth can-i create deployments --as=system:serviceaccount:kuberde:kuberde-operator -n kuberde
# Should return "yes"
```

**Solution**:
```bash
# Apply RBAC manifests
kubectl apply -f deploy/k8s/03-operator.yaml
# (This includes ServiceAccount, Role, RoleBinding)

# Verify RBAC is correct
kubectl get role -n kuberde
kubectl get rolebinding -n kuberde
```

**Required permissions**:
- `deployments.apps`: create, get, update, patch, delete, list, watch
- `pods`: get, list, watch
- `rdeagents.kuberde.io`: get, list, watch
- `rdeagents/status.kuberde.io`: get, patch, update

---

## Deep Dive: Root Cause Analysis

### Understanding the Three Control Loops

The operator has **three independent control loops**, each can fail independently:

```
┌─────────────────────────────────────────────────┐
│  Kubernetes Event Stream                        │
│  (RDEAgent CR changes)                          │
│         ↓                                        │
│  [1] RECONCILIATION LOOP (Event-driven)         │
│      - Updates deployment from CR               │
│      - Respects existing TTL state              │
│      - Runs on every CR change                  │
└─────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│  Timer Loop (30 seconds)                        │
│         ↓                                        │
│  [2] SERVER SESSION MONITOR                     │
│      - Polls /mgmt/agents/{id} on server        │
│      - Updates lastActivity timestamp           │
│      - Triggers auto-scale-up when sessions     │
│        are detected                             │
└─────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│  Timer Loop (60 seconds)                        │
│         ↓                                        │
│  [3] TTL ENFORCER                               │
│      - Checks if TTL has expired                │
│      - Scales down to replicas=0 when expired   │
│      - Preserves state across reconciliation    │
└─────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│  Timer Loop (30 seconds)                        │
│         ↓                                        │
│  [4] STATUS UPDATER                             │
│      - Updates RDEAgent CR .status              │
│      - Reports pod count and phase              │
└─────────────────────────────────────────────────┘
```

### Identifying Which Loop Failed

**Failure Pattern A: Replicas never scale down**
- **Likely Cause**: TTL Enforcer loop (Loop 3) not running
- **Diagnosis**: Check operator logs for `TTL enforcer` lines
- **Fix**: Restart operator or check for scheduler issues

**Failure Pattern B: lastActivity never updates**
- **Likely Cause**: Server Session Monitor loop (Loop 2) failing
- **Diagnosis**: Check operator logs for `session monitor` or `/mgmt/agents` API calls
- **Fix**: Verify server is running and accessible, check credentials

**Failure Pattern C: Deployment spec constantly changed**
- **Likely Cause**: Reconciliation loop (Loop 1) detecting false changes
- **Diagnosis**: Check operator logs for `deploymentSpecChanged returned: true`
- **Fix**: Verify Phase 4.1 fix is deployed (v2.1+), check for CR mutations

**Failure Pattern D: Pod status not updating**
- **Likely Cause**: Status Updater loop (Loop 4) failing
- **Diagnosis**: Check if `.status` fields on CR are stale
- **Fix**: Restart operator or check API server performance

### Loop-Specific Diagnostics

#### Troubleshooting Loop 1: Reconciliation

```bash
# Check for reconciliation errors
kubectl logs -n kuberde -l app=kuberde-operator | grep -i "reconcil"

# Monitor reconciliation frequency
kubectl logs -n kuberde -l app=kuberde-operator | grep "RDEAgent" | head -20

# Check deployment spec (should match CR spec)
AGENT_NAME="<agent-name>"
echo "CR Spec:"
kubectl get rdeagent -n kuberde $AGENT_NAME -o jsonpath='{.spec.image}'

echo "Deployment Spec:"
kubectl get deployment -n kuberde user-testuser-$AGENT_NAME -o jsonpath='{.spec.template.spec.containers[0].image}'
```

**If specs don't match**:
1. Check CR syntax: `kubectl get rdeagent -n kuberde $AGENT_NAME -o yaml`
2. Check operator has RBAC permissions
3. Check operator logs for update errors

#### Troubleshooting Loop 2: Server Session Monitor

```bash
# Check if operator can reach server
kubectl exec -n kuberde -it <operator-pod> -- curl http://frp-server:8080/mgmt/agents

# Check server agent list
kubectl exec -n kuberde -it <server-pod> -- curl http://localhost:8080/mgmt/agents | jq '.items | length'

# Check operator logs for API calls
kubectl logs -n kuberde -l app=kuberde-operator | grep -i "mgmt\|server monitor"

# Verify credentials for server auth
kubectl get secret -n kuberde user-testuser-<agent-name> -o jsonpath='{.data.token}' | base64 -d | head -20
```

**If API calls failing**:
1. Check server pod is running
2. Check network connectivity between operator and server
3. Verify credentials haven't expired

#### Troubleshooting Loop 3: TTL Enforcer

```bash
# Calculate if TTL has expired
AGENT_NAME="<agent-name>"
LAST_ACTIVITY=$(kubectl get rdeagent -n kuberde $AGENT_NAME -o jsonpath='{.status.lastActivity}')
TTL_SECONDS=$(kubectl get rdeagent -n kuberde $AGENT_NAME -o jsonpath='{.spec.ttlSecondsAfterFinished}')

echo "Last Activity: $LAST_ACTIVITY"
echo "TTL Seconds: $TTL_SECONDS"
echo "Should expire at: <calculate: LAST_ACTIVITY + TTL_SECONDS>"

# Check operator logs for TTL enforcement
kubectl logs -n kuberde -l app=kuberde-operator | grep -i "ttl"

# Check if replicas were actually scaled
kubectl get deployment -n kuberde user-testuser-$AGENT_NAME -o jsonpath='{.spec.replicas}'
```

**If TTL not enforcing**:
1. Verify `ttlSecondsAfterFinished` is set in CR
2. Check operator logs for TTL enforcer errors
3. Verify `lastActivity` is being updated (Loop 2 working)
4. Restart operator to reset TTL enforcer timer

#### Troubleshooting Loop 4: Status Updater

```bash
# Check if CR status is current
AGENT_NAME="<agent-name>"
echo "CR Status:"
kubectl get rdeagent -n kuberde $AGENT_NAME -o jsonpath='{.status}'

echo "Actual Pod Count:"
kubectl get pods -n kuberde -l instance=user-testuser-$AGENT_NAME | wc -l

# Check operator logs for status updates
kubectl logs -n kuberde -l app=kuberde-operator | grep -i "status update"
```

---

## Performance & Resource Issues

### High CPU Usage

**Diagnosis**:
```bash
# Check operator CPU
kubectl top pod -n kuberde -l app=kuberde-operator

# Check if reconciliation is too frequent
kubectl logs -n kuberde -l app=kuberde-operator | grep "RDEAgent" | wc -l
# Should be < 5 per minute (unless CRs changing)

# Check for hot loops
kubectl logs -n kuberde -l app=kuberde-operator --timestamps=true | grep -E "\.[\d]{3,}" | head -5
```

**Solutions**:

1. **Reduce reconciliation frequency** (if CR changes are frequent):
   - Review automation that's modifying CRs
   - Consider batching CR updates

2. **Check for hot loops**:
   - Verify Phase 4.1 fix is deployed (false-positive changes cause loops)
   - Restart operator if stuck in loop

3. **Increase operator resources**:
```bash
kubectl patch deployment -n kuberde kuberde-operator -p '{"spec":{"template":{"spec":{"containers":[{"name":"kuberde-operator","resources":{"requests":{"cpu":"500m","memory":"512Mi"},"limits":{"cpu":"1000m","memory":"1Gi"}}}]}}}}'
```

### High Memory Usage

**Diagnosis**:
```bash
# Check memory usage
kubectl top pod -n kuberde -l app=kuberde-operator
# Normal: 50-200MB, High: > 500MB

# Check for memory leaks in logs
kubectl logs -n kuberde -l app=kuberde-operator --tail=100 | grep -i "leak\|goroutine\|panic"
```

**Solutions**:

1. **Restart operator**:
```bash
kubectl rollout restart deployment/kuberde-operator -n kuberde
```

2. **Check for resource leaks** (too many agents):
   - Count agents: `kubectl get rdeagent -n kuberde | wc -l`
   - If > 100, may need operator scaling (see SCALING.md)

3. **Increase memory limits**:
```bash
kubectl patch deployment -n kuberde kuberde-operator -p '{"spec":{"template":{"spec":{"containers":[{"name":"kuberde-operator","resources":{"limits":{"memory":"2Gi"}}}]}}}}'
```

---

## Network & Connectivity Issues

### Agent Can't Reach Server

**Symptoms**:
- Agent logs show `connection refused` or `no route to host`
- No agent appears on server's `/mgmt/agents` list

**Diagnosis**:
```bash
# From agent pod
kubectl exec -n kuberde -it <agent-pod> -- \
  curl -v -H "Authorization: Bearer <token>" \
  ws://frp-server:8080/ws

# Check DNS resolution
kubectl exec -n kuberde -it <agent-pod> -- nslookup frp-server
kubectl exec -n kuberde -it <agent-pod> -- nslookup frp-server.kuberde

# Check network policy
kubectl get networkpolicy -n kuberde
```

**Solutions**:

1. **Fix SERVER_URL**:
```bash
kubectl patch rdeagent -n kuberde <agent-name> -p \
  '{"spec":{"env":[{"name":"SERVER_URL","value":"ws://frp-server:8080/ws"}]}}'
```

2. **Fix network policy** (if blocking traffic):
```bash
# Verify policy allows traffic
kubectl get networkpolicy -n kuberde -o yaml

# Delete restrictive policy if needed
kubectl delete networkpolicy -n kuberde <policy-name>
```

3. **Debug connectivity**:
```bash
# TCP connectivity
kubectl exec -n kuberde -it <agent-pod> -- \
  nc -zv frp-server 8080

# WebSocket connectivity
kubectl exec -n kuberde -it <agent-pod> -- \
  timeout 5 curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" \
  http://frp-server:8080/ws || true
```

### Server Can't Reach Agent

**Symptoms**:
- Server shows agent connected but can't open streams
- User can't access services through agent

**Diagnosis**:
```bash
# Check agent pod IP
kubectl get pod -n kuberde -l instance=user-testuser-<agent-name> -o jsonpath='{.items[0].status.podIP}'

# Check if server can reach it
kubectl exec -n kuberde -it <server-pod> -- \
  curl http://<agent-pod-ip>:local-target-port

# Check network policy
kubectl get networkpolicy -n kuberde
```

**Solutions**: Same as "Agent Can't Reach Server"

---

## Data & State Consistency Issues

### LastActivity Not Updating

**Symptom**: `lastActivity` timestamp stays old despite active sessions

**Diagnosis**:
```bash
# Check lastActivity
kubectl get rdeagent -n kuberde <agent-name> -o jsonpath='{.status.lastActivity}'

# Check operator can reach server
kubectl exec -n kuberde -it <operator-pod> -- \
  curl http://frp-server:8080/mgmt/agents/<agent-name>

# Check server tracking sessions
kubectl exec -n kuberde -it <server-pod> -- \
  curl http://localhost:8080/mgmt/agents/<agent-name> | jq '.status.sessions'
```

**Solutions**:

1. **Verify server is running**:
```bash
kubectl get pod -n kuberde -l app=frp-server
```

2. **Check credentials**:
```bash
kubectl get secret -n kuberde user-testuser-<agent-name> -o yaml
```

3. **Restart status monitor** (via operator restart):
```bash
kubectl rollout restart deployment/kuberde-operator -n kuberde
```

### CR Status Out of Sync

**Symptom**: CR `.status` fields don't match actual pod state

**Diagnosis**:
```bash
# Check CR status
kubectl get rdeagent -n kuberde <agent-name> -o jsonpath='{.status}'

# Check actual pods
kubectl get pods -n kuberde -l instance=user-testuser-<agent-name> -o wide

# Compare counts
echo "CR says: $(kubectl get rdeagent -n kuberde <agent-name> -o jsonpath='{.status.activeReplicas}')"
echo "Actually running: $(kubectl get pods -n kuberde -l instance=user-testuser-<agent-name> | grep Running | wc -l)"
```

**Solution**:
```bash
# Force status update
kubectl rollout restart deployment/kuberde-operator -n kuberde

# Or manually patch if urgent
kubectl patch rdeagent -n kuberde <agent-name> -p \
  '{"status":{"activeReplicas":X}}' --subresource=status
```

---

## Extended Stability Test

Run this to verify the Phase 4.1 fix is working (no false-positive changes):

```bash
#!/bin/bash
AGENT_NAME="ssh-server"
NAMESPACE="kuberde"
TEST_DURATION=300  # 5 minutes

echo "=== Extended Stability Test ==="
echo "Agent: $AGENT_NAME"
echo "Duration: $TEST_DURATION seconds"
echo ""

START=$(date +%s)
PREV_HASH=""
CHANGE_COUNT=0

while [ $(($(date +%s) - START)) -lt $TEST_DURATION ]; do
  HASH=$(kubectl get deployment -n $NAMESPACE user-testuser-$AGENT_NAME \
    -o jsonpath='{.spec.template.metadata.labels.pod-template-hash}')

  if [ "$HASH" != "$PREV_HASH" ] && [ ! -z "$PREV_HASH" ]; then
    CHANGE_COUNT=$((CHANGE_COUNT + 1))
    echo "[$(date '+%H:%M:%S')] ⚠️  Pod template hash changed (change #$CHANGE_COUNT)"
  fi

  PREV_HASH="$HASH"
  sleep 10
done

echo ""
echo "=== Test Complete ==="
echo "Total hash changes: $CHANGE_COUNT"

if [ $CHANGE_COUNT -eq 0 ]; then
  echo "✅ PASS: No false-positive changes (Phase 4.1 fix working)"
  exit 0
else
  echo "❌ FAIL: Detected $CHANGE_COUNT changes (Phase 4.1 fix may not be active)"
  exit 1
fi
```

---

## Recovery Procedures

### Full Operator Reset

Use when operator is in an inconsistent state:

```bash
#!/bin/bash
echo "=== Full Operator Reset ==="

# 1. Delete operator deployment
echo "1. Stopping operator..."
kubectl delete deployment -n kuberde kuberde-operator --ignore-not-found

# 2. Wait for cleanup
echo "2. Waiting 30 seconds for cleanup..."
sleep 30

# 3. Redeploy operator
echo "3. Redeploying operator..."
kubectl apply -f deploy/k8s/03-operator.yaml

# 4. Wait for readiness
echo "4. Waiting for operator to be ready..."
kubectl rollout status deployment/kuberde-operator -n kuberde -w

# 5. Verify
echo "5. Verifying operator status..."
kubectl get pod -n kuberde -l app=kuberde-operator
kubectl logs -n kuberde -l app=kuberde-operator --tail=20

echo "=== Reset Complete ==="
```

### Clean Rebuild All Deployments

Use when you want to recreate all agent deployments from CRs:

```bash
#!/bin/bash
echo "=== Clean Rebuild All Deployments ==="

# 1. Back up CRs
echo "1. Backing up RDEAgent CRs..."
kubectl get rdeagent -n kuberde -o yaml > /tmp/rdeagent-backup-$(date +%s).yaml

# 2. Delete all deployments (CRs preserved)
echo "2. Deleting all deployments..."
kubectl delete deployment -n kuberde -l app=frp-agent

# 3. Wait for cleanup
echo "3. Waiting 10 seconds..."
sleep 10

# 4. Restart operator to trigger reconciliation
echo "4. Restarting operator to trigger reconciliation..."
kubectl rollout restart deployment/kuberde-operator -n kuberde
kubectl rollout status deployment/kuberde-operator -n kuberde -w

# 5. Wait for deployments to recreate
echo "5. Waiting for deployments to recreate..."
sleep 10
kubectl get deployment -n kuberde -l app=frp-agent

echo "=== Rebuild Complete ==="
```

### Restore Backed-up CRs

```bash
kubectl apply -f /tmp/rdeagent-backup-*.yaml
```

---

## When to Escalate

### Provide This Information

When escalating to platform team or vendor support:

```bash
#!/bin/bash
echo "=== FRP Operator Diagnostic Bundle ==="
echo "Generated: $(date)"
echo ""

echo "1. OPERATOR STATUS"
kubectl get pods -n kuberde -l app=kuberde-operator -o wide

echo ""
echo "2. OPERATOR LOGS (last 100 lines)"
kubectl logs -n kuberde -l app=kuberde-operator --tail=100

echo ""
echo "3. AGENT CRS"
kubectl get rdeagent -n kuberde -o wide

echo ""
echo "4. DEPLOYMENTS"
kubectl get deployment -n kuberde -l app=frp-agent -o wide

echo ""
echo "5. PODS"
kubectl get pods -n kuberde -l app=frp-agent -o wide

echo ""
echo "6. EVENTS"
kubectl get events -n kuberde --sort-by='.lastTimestamp' | tail -50

echo ""
echo "7. NODE STATUS"
kubectl get nodes -o wide

echo ""
echo "8. CLUSTER VERSION"
kubectl version

echo ""
echo "=== End of Bundle ==="
```

Save to file:
```bash
bash diagnostic.sh > frp-diagnostic-$(date +%s).log
```

### When to Contact Support

- Operator pod in CrashLoopBackOff lasting > 2 minutes
- All agents unable to connect to server
- TTL enforcement not working after 10+ minute wait
- Pod rebuild loops despite v2.1 operator deployment
- Unexplained resource exhaustion (CPU > 2 cores, memory > 1GB)
- Data loss or state inconsistency requiring recovery

---

## Glossary

| Term | Meaning |
|------|---------|
| **RDEAgent CR** | Kubernetes Custom Resource defining an agent (from CRD) |
| **Reconciliation** | Process of syncing CR spec to actual deployment |
| **TTL** | Time-To-Live: idle timeout for auto-scaling |
| **lastActivity** | Timestamp of last user session |
| **Pod Template Hash** | Kubernetes metadata hash identifying deployment version |
| **Replicas** | Desired number of running pods (0 = scaled down) |
| **Yamux** | Multiplexing protocol for WebSocket streams |
| **RBAC** | Role-Based Access Control for Kubernetes permissions |

---

**Document Version**: 1.0
**Last Reviewed**: 2025-12-07
**Next Review**: 2025-12-14
