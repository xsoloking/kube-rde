# Karmada Multi-Cluster Deployment Guide

This directory contains scripts and manifests to enable Karmada-based multi-cluster scheduling for KubeRDE. With Karmada, each team's Agent workloads can be scheduled to a dedicated member cluster while the control plane (Server, PostgreSQL, Keycloak) stays on the Hub cluster.

## Architecture

```
Hub Cluster (existing KubeRDE cluster)
├── KubeRDE control plane: Server × 3, PostgreSQL, Keycloak, Web UI
└── Karmada control plane: karmada-apiserver, controller-manager, scheduler
        │
        │ PropagationPolicy / OverridePolicy
        ▼
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│  Cluster-A  │   │  Cluster-B  │   │  Cluster-C  │
│ (Team Alpha)│   │ (Team Beta) │   │ (GPU Pool)  │
│  Operator   │   │  Operator   │   │  Operator   │
│  Agent Pods │   │  Agent Pods │   │  Agent Pods │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       └──────────────────┴──────────────────┘
                          │
             WebSocket + JWT → Hub Server public URL
             agent_pod_sessions records session (unchanged)
```

Agent Pods in member clusters connect back to the Hub Server via its public WebSocket URL. The `agent_pod_sessions` table on the Hub PostgreSQL records each agent's session pod IP — the HA data plane is completely unchanged.

## Prerequisites

- Hub cluster running KubeRDE (single-cluster HA version)
- Kubeconfigs for each member cluster available locally
- `karmadactl` CLI installed:
  ```bash
  curl -s https://raw.githubusercontent.com/karmada-io/karmada/master/hack/install-cli.sh | sudo bash
  ```
- Helm 3 installed

## Deployment Steps

### 1. Install Karmada control plane on Hub cluster

```bash
./deploy/karmada/00-install.sh
```

Installs Karmada v1.9.0 via Helm and saves the Karmada kubeconfig to `/tmp/karmada.kubeconfig`.

### 2. Register member clusters

```bash
./deploy/karmada/02-join-clusters.sh cluster-a ~/.kube/cluster-a.yaml
./deploy/karmada/02-join-clusters.sh cluster-b ~/.kube/cluster-b.yaml
```

### 3. Label clusters and apply propagation policies

```bash
export KUBECONFIG=/tmp/karmada.kubeconfig

# Label clusters as KubeRDE members
kubectl label cluster cluster-a kuberde.io/member=true
kubectl label cluster cluster-b kuberde.io/member=true

# Propagate RDEAgent CRD to all member clusters
kubectl apply -f deploy/karmada/03-crd-propagation.yaml

# Propagate KubeRDE Operator to all member clusters
kubectl apply -f deploy/karmada/04-operator-propagation.yaml
```

### 4. Mount Karmada kubeconfig into Server

```bash
# Save kubeconfig as a Secret in the kuberde namespace
kubectl create secret generic karmada-kubeconfig \
  --from-file=kubeconfig=/tmp/karmada.kubeconfig \
  -n kuberde

# Restart Server to pick up the new volume
make restart-server
```

The Server Deployment in `deploy/k8s/03-server.yaml` already mounts this Secret at `/etc/karmada/kubeconfig` with `optional: true` — if the Secret is absent the server starts in single-cluster mode without any error.

### 5. Run database migrations

```bash
kubectl exec -n kuberde deploy/kuberde-server -- \
  goose -dir /migrations postgres "$DATABASE_URL" up
```

Adds `cluster_name TEXT NOT NULL DEFAULT 'default'` to both `teams` and `agent_pod_sessions` tables.

### 6. (Optional) Create per-cluster storage class OverridePolicies

Edit `deploy/karmada/05-override-policies.yaml` to match each cluster's actual storage class, then apply:

```bash
kubectl apply -f deploy/karmada/05-override-policies.yaml \
  --kubeconfig=/tmp/karmada.kubeconfig
```

## End-to-End Verification

```bash
# 1. Create a team targeting cluster-a (via Web UI or API)
curl -X POST https://frp.byai.uk/api/admin/teams \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"alpha","display_name":"Alpha Team","namespace":"kuberde-alpha","cluster_name":"cluster-a"}'

# 2. Verify namespace propagated to cluster-a
KUBECONFIG=~/.kube/cluster-a.yaml kubectl get ns kuberde-alpha

# 3. Create a service for the team (via Web UI); then verify RDEAgent CR on cluster-a
KUBECONFIG=~/.kube/cluster-a.yaml kubectl get rdeagents -n kuberde-alpha

# 4. Verify Operator created the Agent Deployment on cluster-a
KUBECONFIG=~/.kube/cluster-a.yaml kubectl get deployments -n kuberde-alpha

# 5. Verify Agent connected back to Hub
kubectl logs -n kuberde -l app=kuberde-server | grep "agent registered"

# 6. Verify agent_pod_sessions on Hub PostgreSQL
psql "$DATABASE_URL" -c "SELECT agent_id, pod_ip, cluster_name FROM agent_pod_sessions;"

# 7. SSH connection (transparent — no change for end users)
ssh kuberde-agent-alpha-xxx
```

## Usage

Once deployed, administrators can assign teams to member clusters via the **Team Management** page in the Web UI. Each team has a **Target Cluster** dropdown populated from `GET /api/clusters`.

When a service is created for a team assigned to `cluster-a`:
1. Server creates the RDEAgent CR in the Karmada control plane
2. Server creates a `PropagationPolicy` routing the CR to `cluster-a`
3. Karmada propagates the CR; the local Operator creates the Agent Deployment
4. Agent Pod connects back to Hub Server via `KUBERDE_AGENT_SERVER_URL`
5. `agent_pod_sessions` records the Hub pod IP (HA routing unchanged)

## Backward Compatibility

| Scenario | Behavior |
|----------|----------|
| `KARMADA_KUBECONFIG` Secret absent | `karmadaEnabled=false`; all single-cluster paths unchanged |
| Team `cluster_name = "default"` | No PropagationPolicy; RDEAgent scheduled on Hub cluster |
| `GET /api/clusters` in single-cluster mode | Returns `[{"name":"default","status":"Ready"}]` |
| Existing `agent_pod_sessions` rows | `cluster_name` defaults to `"default"`; HA routing unchanged |
| Existing teams without `cluster_name` | Default `"default"` applied by migration; no behavior change |

## File Reference

| File | Purpose |
|------|---------|
| `00-install.sh` | Install Karmada on Hub cluster via Helm |
| `01-karmada-values.yaml` | Helm values for Karmada installation |
| `02-join-clusters.sh` | Register a member cluster with `karmadactl join` |
| `03-crd-propagation.yaml` | `ClusterPropagationPolicy`: push RDEAgent CRD to all members |
| `04-operator-propagation.yaml` | `ClusterPropagationPolicy`: push KubeRDE Operator to all members |
| `05-override-policies.yaml` | Per-cluster storage class `OverridePolicy` examples |
