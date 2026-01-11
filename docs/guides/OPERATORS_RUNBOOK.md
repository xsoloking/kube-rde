# FRP Operator Production Runbook

**Status**: Production (v2.1)
**Last Updated**: 2025-12-07
**Version**: v2.1 with Phase 4.1 TTL fix

---

## Quick Reference: Emergency Commands

```bash
# Check operator health
kubectl get pods -n kuberde -l app=kuberde-operator

# View operator logs
kubectl logs -n kuberde -l app=kuberde-operator -f

# List all managed agents
kubectl get rdeagent -n kuberde

# Check specific agent status
kubectl get rdeagent -n kuberde <agent-name> -o wide

# Check deployment for an agent
kubectl get deployment -n kuberde user-<owner>-<name>

# Restart operator (force new pod)
kubectl delete pod -n kuberde -l app=kuberde-operator

# View all events in namespace
kubectl get events -n kuberde --sort-by='.lastTimestamp' | tail -20
```

---

## 1. Daily Health Check Procedures

### 1.1 Operator Pod Health

**Check every 4 hours or after any changes:**

```bash
# Check if operator is running
kubectl get pods -n kuberde -l app=kuberde-operator

# Expected output:
# NAME                            READY   STATUS    RESTARTS   AGE
# kuberde-operator-5d76c7f8fc-l9njm   1/1     Running   0          23h
```

**What to verify**:
- ✅ READY = 1/1 (not 0/1)
- ✅ STATUS = Running (not CrashLoopBackOff or Pending)
- ✅ RESTARTS = 0 or very low (not growing every minute)
- ✅ AGE = hours/days (recent restarts are concerning)

**If unhealthy**:
1. Check logs: `kubectl logs -n kuberde -l app=kuberde-operator --tail=100`
2. Look for error patterns: `kubectl logs -n kuberde -l app=kuberde-operator | grep -i error`
3. If stuck: `kubectl delete pod -n kuberde -l app=kuberde-operator` (will restart)
4. Monitor for stabilization: watch the pod for 2 minutes

---

### 1.2 Managed Agents Overview

**Check every 8 hours:**

```bash
kubectl get rdeagent -n kuberde -o wide
```

**Expected output pattern**:
```
NAME               STATUS    ONLINE   POD                                               AGE
ssh-server         Running   true     user-testuser-ssh-server-7c445d9b55-dn7ff        6m
jupyter-notebook   ScaledDown false   <none>                                           6m
code-server        Running   true     user-testuser-code-server-6787fb98f9-7lwq2      6m
```

**What to verify**:
- ✅ All agents have STATUS = Running or ScaledDown (not Error or Pending)
- ✅ ONLINE status matches expected state (Running=true, ScaledDown=false)
- ✅ No agents stuck in Pending state for > 5 minutes
- ✅ Pod names match expected pattern: `user-{owner}-{name}-{hash}`

**Red flags**:
- ❌ Agent stuck in "Pending" state > 5 minutes = Pod scheduling issue
- ❌ Agent in "Error" state = Deployment/PVC issue
- ❌ Agent "Online=true" but no pod = Network connectivity issue

---

### 1.3 Deployment Stability Check

**For any Running agents, verify pods aren't rebuilding:**

```bash
# Pick one agent and watch for 2 minutes
kubectl get deployment -n kuberde user-testuser-ssh-server -o wide

# Expected: Replicas and Ready counts stay CONSTANT
# Run this 3 times at 30-second intervals:
for i in 1 2 3; do
  echo "[$(date '+%H:%M:%S')] Replicas: $(kubectl get deployment -n kuberde user-testuser-ssh-server -o jsonpath='{.spec.replicas}')"
  sleep 30
done
```

**What to verify**:
- ✅ REPLICAS and READY stay constant (e.g., 1 and 1)
- ✅ No "pod-template-hash" changes (no forced recreations)
- ✅ AGE of pods doesn't reset frequently

**Red flags** (indicate false-positive change detection):
- ❌ REPLICAS toggling 1→0→1→0 every 30 seconds = Phase 4.1 bug NOT fixed
- ❌ Pod counts constantly changing = Operator reconciliation errors
- ❌ Same pod being restarted repeatedly = Deployment spec conflict

---

## 2. TTL Enforcement Monitoring

### 2.1 Verify TTL Configuration

**Check that TTL fields are set correctly:**

```bash
# List all agents with their TTL values
kubectl get rdeagent -n kuberde -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.ttl}{"\n"}{end}'

# Expected output:
# ssh-server      5m
# jupyter-notebook  5m
# code-server     5m
```

**What to verify**:
- ✅ All production agents have TTL defined (e.g., 5m, 10m, 30m)
- ✅ TTL values are reasonable (not 1s, not 24h)
- ✅ TTL format is valid (e.g., "5m", "300s", "1h")

---

### 2.2 Monitor TTL Enforcement Events

**Check if TTL enforcer loop is working (run every 2 hours):**

```bash
# Look for TTL enforcement in logs (last hour)
kubectl logs -n kuberde -l app=kuberde-operator --since=1h | grep -i "ttl\|scale"

# Expected patterns:
# "Agent X is scaled down by TTL, preserving replicas=0"
# "lastActivity: ... TTL not expired"
# "TTL enforcer: checking X agents"
```

**What to verify**:
- ✅ TTL enforcer loop is running (messages every ~60 seconds)
- ✅ Scale-down messages appear when TTL should have expired
- ✅ No errors in TTL logic

**Red flags**:
- ❌ No TTL messages = Loop not running or no agents with TTL
- ❌ Scale-down not happening = TTL logic broken
- ❌ "Error" messages in TTL processing = Fix needed

---

### 2.3 Test TTL Auto-Recovery

**Weekly: Test that scaled-down agents auto-recover on session:**

```bash
# 1. Find a scaled-down agent
kubectl get rdeagent -n kuberde -o wide | grep ScaledDown

# 2. Simulate a session (depends on your agent type)
# Example for ssh-server:
ssh -p 2022 user@127.0.0.1  # Will trigger scale-up

# 3. Verify agent scales back up (within 60 seconds)
watch 'kubectl get rdeagent -n kuberde ssh-server -o jsonpath="{.status.phase} - Replicas: $(kubectl get deployment -n kuberde user-testuser-ssh-server -o jsonpath=\"{.spec.replicas}\")"'

# Expected sequence:
# ScaledDown - Replicas: 0
# [30s later] Running - Replicas: 1  ✅
```

**Success criteria**:
- ✅ Agent scales to replicas=1 within 60 seconds
- ✅ Pod starts and becomes ready
- ✅ Session connects successfully after scale-up

---

## 3. Troubleshooting Guide

### Issue: Agent Not Connecting to Server

**Symptoms**:
- `kubectl get rdeagent` shows ONLINE=false
- Sessions fail with connection refused

**Diagnosis**:
```bash
# 1. Check agent deployment exists
kubectl get deployment -n kuberde user-<owner>-<name>

# 2. Check if pod is running
kubectl get pods -n kuberde -l app=frp-agent,instance=user-<owner>-<name>

# 3. Check pod logs for connection errors
kubectl logs -n kuberde -l app=frp-agent,instance=user-<owner>-<name>

# 4. Verify environment variables
kubectl get deployment -n kuberde user-<owner>-<name> -o jsonpath='{.spec.template.spec.containers[0].env[*]}'

# 5. Test server connectivity from pod
kubectl exec -it -n kuberde <pod-name> -- /bin/sh
# Inside pod:
curl -I http://frp-server.kuberde.svc:8080/ws
```

**Common causes & fixes**:

| Cause | Check | Fix |
|-------|-------|-----|
| Pod not scheduled | `kubectl describe pod <name>` | Check node resources, tolerations |
| Server not accessible | Pod logs show connection refused | Verify frp-server is running: `kubectl get pods -n kuberde -l app=frp-server` |
| Wrong env vars | Check pod env vars | Update RDEAgent CR spec.env |
| Network policy blocking | Check network policies | Review K8s NetworkPolicy resources |
| Auth token expired | Check pod logs for "401" or "403" | Token auto-refresh should handle this; check Keycloak |

---

### Issue: TTL Not Enforcing (Agent Not Scaling Down)

**Symptoms**:
- Agent past TTL expiration but still at replicas=1
- lastActivity not updating

**Diagnosis**:
```bash
# 1. Check if lastActivity is updating
kubectl get rdeagent -n kuberde <name> -o jsonpath='{.status.lastActivity}'

# 2. Check TTL configuration
kubectl get rdeagent -n kuberde <name> -o jsonpath='{.spec.ttl}'

# 3. Calculate: is now > lastActivity + TTL?
# Example: if TTL=5m and lastActivity=12:00:00, should scale down at 12:05:00

# 4. Check operator logs for TTL messages
kubectl logs -n kuberde -l app=kuberde-operator | grep -A5 -B5 "TTL"

# 5. Check deployment replicas directly
kubectl get deployment -n kuberde user-<owner>-<name> -o jsonpath='{.spec.replicas}'
```

**Common causes & fixes**:

| Cause | Check | Fix |
|-------|-------|-----|
| Active sessions present | Check server for active sessions to agent | Sessions prevent scale-down; terminate/wait |
| TTL too short | `kubectl get rdeagent <name> -o jsonpath='{.spec.ttl}'` | Increase TTL in RDEAgent CR |
| Server monitor updating lastActivity | Logs show constant updates | Active session detected; check server session tracking |
| TTL enforcer loop not running | Operator logs | Restart operator: `kubectl delete pod -n kuberde -l app=kuberde-operator` |
| Replicas reset to 1 immediately | Logs show "deploymentSpecChanged: true" | Phase 4.1 bug; verify operator image is v2.1 |

---

### Issue: Constant Pod Rebuilds (30-second restarts)

**Symptoms**:
- Pod keeps restarting every 20-50 seconds
- Agent marked as "Running" but pods keep cycling

**Diagnosis** (CRITICAL - indicates Phase 4.1 bug not applied):
```bash
# 1. Check operator image version
kubectl get deployment -n kuberde kuberde-operator -o jsonpath='{.spec.template.spec.containers[0].image}'
# Should show: soloking/kuberde-operator:v2.1 (or later)

# 2. Check reconciliation logs for false-positive changes
kubectl logs -n kuberde -l app=kuberde-operator | grep "deploymentSpecChanged"
# Should show: "deploymentSpecChanged returned: false"
# If shows "true", there's a false-positive change detection bug

# 3. Watch pod restarts in real-time
kubectl get pods -n kuberde -l app=frp-agent,instance=user-<owner>-<name> -w

# 4. Verify pod-template-hash doesn't change
kubectl get deployment -n kuberde user-<owner>-<name> -o jsonpath='{.spec.template.metadata.labels.pod-template-hash}'
# Run this 3 times 30 seconds apart; hash should be CONSTANT
```

**Root cause**: Kubernetes comparison bug in reconciliation loop
- Operator code comparing `nil` vs empty containers
- Results in false "change detected"
- Each false update triggers pod restart

**Solution**:
```bash
# 1. Verify you're running v2.1
kubectl get deployment -n kuberde kuberde-operator -o jsonpath='{.spec.template.spec.containers[0].image}'

# 2. If v2.0 or earlier:
kubectl set image deployment/kuberde-operator -n kuberde \
  operator=soloking/kuberde-operator:v2.1

# 3. Force pod restart to pick up new image
kubectl delete pod -n kuberde -l app=kuberde-operator

# 4. Verify logs show fix is working
kubectl logs -n kuberde -l app=kuberde-operator --tail=50 | grep "deploymentSpecChanged"
# Should show: "deploymentSpecChanged returned: false"

# 5. Monitor agent for stable pod (should not restart)
kubectl get pods -n kuberde -l app=frp-agent,instance=user-<owner>-<name> -w
# After fix, pods should be stable (not restarting)
```

---

### Issue: Operator Pod CrashLoopBackOff

**Symptoms**:
- Operator pod shows STATUS = CrashLoopBackOff
- All agents show ONLINE=false (they can't connect to operator)

**Diagnosis**:
```bash
# 1. Get detailed pod status
kubectl describe pod -n kuberde -l app=kuberde-operator

# 2. Check pod logs for crash reason
kubectl logs -n kuberde -l app=kuberde-operator
# Look for: panic, fatal error, or last log message before crash

# 3. Check previous logs (if pod has restarted multiple times)
kubectl logs -n kuberde -l app=kuberde-operator --previous

# 4. Check for resource constraints
kubectl top pod -n kuberde -l app=kuberde-operator

# 5. Check for permission issues
kubectl get rolebinding,clusterrolebinding -n kuberde | grep kuberde-operator
kubectl auth can-i get rdeagents --as=system:serviceaccount:kuberde:kuberde-operator
```

**Common causes & fixes**:

| Cause | Symptom | Fix |
|-------|---------|-----|
| OOM (Out of Memory) | "killed" in logs | Increase memory: `kubectl set resources deployment/kuberde-operator -n kuberde --limits=memory=1Gi` |
| RBAC insufficient | "forbidden" errors | Verify ClusterRole includes all needed permissions |
| Bad config mount | "no such file" errors | Check configmap/secret volumes are created |
| Code panic | "panic" in logs | Upgrade to fix if known bug; report if unknown |

**Recovery procedure**:
```bash
# 1. Investigate root cause from logs

# 2. If temporary issue, restart operator
kubectl delete pod -n kuberde -l app=kuberde-operator

# 3. Monitor logs while it restarts
kubectl logs -n kuberde -l app=kuberde-operator -f

# 4. Once stable (2+ minutes without errors), verify agents reconnect
kubectl get rdeagent -n kuberde
# Should show ONLINE=true within 30 seconds
```

---

## 4. Performance & Resource Management

### 4.1 Operator Resource Usage

**Monitor every day:**

```bash
# Check actual resource usage
kubectl top pod -n kuberde -l app=kuberde-operator

# Expected (for 4-10 agents):
# NAME                            CPU     MEMORY
# kuberde-operator-5d76c7f8fc-l9njm   15m     180Mi
```

**Limits defined in deployment**:
```yaml
resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

**If approaching limits**:
```bash
# Check number of agents
kubectl get rdeagent -n kuberde | wc -l

# If > 50 agents, consider increasing limits:
kubectl patch deployment kuberde-operator -n kuberde --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/memory", "value":"1Gi"}]'
```

---

### 4.2 Loop Performance Metrics

**Check loop execution times in logs:**

```bash
# Extract timing information
kubectl logs -n kuberde -l app=kuberde-operator --since=1h | grep -E "TTL|monitor|status"

# Look for lines like:
# "TTL enforcer completed 4 agents in 250ms"
# "Server monitor checked in 120ms"
# "Status updater processed in 90ms"
```

**Expected times** (for 4-10 agents):
- TTL enforcer: < 500ms
- Server monitor: < 300ms
- Status updater: < 200ms

**If exceeding limits**:
- Check if agents have large workloads (high CPU/memory)
- Check network latency to server API
- Monitor server API response times

---

## 5. Backup & Recovery Procedures

### 5.1 Backup RDEAgent CRs (Agent Definitions)

**Before any operator upgrade:**

```bash
# Backup all RDEAgent resources
kubectl get rdeagent -n kuberde -o yaml > frpagent_backup_$(date +%Y%m%d_%H%M%S).yaml

# Verify backup is valid
kubectl apply -f frpagent_backup_*.yaml --dry-run=client

# Store backup securely (git, S3, etc.)
git add frpagent_backup_*.yaml && git commit -m "Backup RDEAgent CRs before upgrade"
```

---

### 5.2 Restore from Backup

**If RDEAgent resources get corrupted:**

```bash
# 1. List available backups
ls -la frpagent_backup_*.yaml

# 2. Verify backup content
cat frpagent_backup_<timestamp>.yaml | grep -E "name:|status:"

# 3. Delete corrupted agents (careful!)
kubectl delete rdeagent -n kuberde --all

# 4. Restore from backup
kubectl apply -f frpagent_backup_<timestamp>.yaml

# 5. Verify restoration
kubectl get rdeagent -n kuberde
```

---

## 6. Upgrade & Rollback Procedures

### 6.1 Upgrade Operator Image

```bash
# 1. Check current version
kubectl get deployment kuberde-operator -n kuberde -o jsonpath='{.spec.template.spec.containers[0].image}'

# 2. Pull new image (pre-test in staging)
docker pull soloking/kuberde-operator:v2.2

# 3. Update deployment
kubectl set image deployment/kuberde-operator \
  -n kuberde \
  operator=soloking/kuberde-operator:v2.2

# 4. Monitor rollout
kubectl rollout status deployment/kuberde-operator -n kuberde --timeout=5m

# 5. Verify pod is healthy
kubectl logs -n kuberde -l app=kuberde-operator --tail=30

# 6. Verify agents still connect
kubectl get rdeagent -n kuberde
# All should have ONLINE=true within 30 seconds
```

### 6.2 Rollback to Previous Version

```bash
# 1. If new version has problems, immediately rollback
kubectl set image deployment/kuberde-operator \
  -n kuberde \
  operator=soloking/kuberde-operator:v2.1

# 2. Force new pod to start
kubectl delete pod -n kuberde -l app=kuberde-operator

# 3. Monitor recovery
kubectl logs -n kuberde -l app=kuberde-operator -f

# 4. Verify agents reconnect
watch 'kubectl get rdeagent -n kuberde -o wide'
```

---

## 7. Incident Response Checklist

### For Production Incidents

**When agents are not connecting:**

- [ ] Check operator pod status: `kubectl get pods -n kuberde -l app=kuberde-operator`
- [ ] Check operator logs: `kubectl logs -n kuberde -l app=kuberde-operator --tail=100`
- [ ] Check server pod: `kubectl get pods -n kuberde -l app=frp-server`
- [ ] Check network connectivity: `kubectl exec <agent-pod> -- ping frp-server.kuberde.svc`
- [ ] Check auth tokens (check Keycloak)
- [ ] Restart operator if needed: `kubectl delete pod -n kuberde -l app=kuberde-operator`
- [ ] Wait 2 minutes for recovery
- [ ] Document incident: what happened, duration, root cause, fix applied

**For scaling issues:**

- [ ] Check TTL configuration: `kubectl get rdeagent -n kuberde -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.ttl}{"\n"}{end}'`
- [ ] Check lastActivity: `kubectl get rdeagent -n kuberde <name> -o jsonpath='{.status.lastActivity}'`
- [ ] Check deployment replicas: `kubectl get deployment -n kuberde user-<owner>-<name>`
- [ ] Check operator logs for TTL messages: `kubectl logs -n kuberde -l app=kuberde-operator | grep TTL`
- [ ] Check if sessions are active: verify with server

---

## 8. Quick Command Reference

```bash
# Status checks
kubectl get rdeagent -n kuberde                          # List all agents
kubectl get rdeagent -n kuberde <name> -o yaml           # Full agent definition
kubectl describe rdeagent -n kuberde <name>              # Agent details
kubectl get deployment -n kuberde                        # List deployments

# Troubleshooting
kubectl logs -n kuberde -l app=kuberde-operator -f           # Stream operator logs
kubectl logs -n kuberde -l app=frp-agent -f              # Stream agent logs
kubectl get events -n kuberde --sort-by='.lastTimestamp' # Recent events
kubectl describe pod -n kuberde <pod-name>               # Pod details

# Updates
kubectl set image deployment/kuberde-operator -n kuberde operator=<image>  # Update image
kubectl rollout status deployment/kuberde-operator -n kuberde               # Monitor rollout
kubectl rollout undo deployment/kuberde-operator -n kuberde                 # Quick rollback

# Debugging
kubectl exec -it -n kuberde <pod-name> -- /bin/sh       # Shell into pod
kubectl port-forward -n kuberde <pod-name> 8080:8080     # Port forward
kubectl top pod -n kuberde                               # Resource usage
```

---

## Document Versions

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-12-07 | Initial production runbook for v2.1 |

---

## Related Documentation

- [PHASE_4_1_FINAL_COMPLETION_REPORT.md](../PHASE_4_1_FINAL_COMPLETION_REPORT.md) - Technical details of v2.1 release
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) - Deep troubleshooting guide
- [MONITORING.md](../operations/MONITORING.md) - Production monitoring setup
- [CLAUDE.md](../CLAUDE.md) - Architecture and design reference
