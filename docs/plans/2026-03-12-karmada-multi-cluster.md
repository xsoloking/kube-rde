# KubeRDE Karmada 多租户多集群架构实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在现有 HA 架构基础上，集成 Karmada 实现多集群调度——每个团队的 Agent 工作负载可部署到独立的成员集群，Server 控制面保持单一入口，跨集群连接透明。

**Architecture:**
- **Hub 集群**：现有 KubeRDE 集群升级加入 Karmada 控制面（karmada-apiserver + etcd + controller-manager），继续运行 Server/PostgreSQL/Keycloak；Server 同时连接 Karmada apiserver 创建跨集群资源。
- **成员集群**：各业务/团队独立 Kubernetes 集群，通过 `karmadactl join` 注册；Karmada 将 RDEAgent CRD 和 CR（含 PropagationPolicy）下发到对应集群，各集群运行独立的 KubeRDE Operator 实例，Agent Pod 回连 Hub 的公网 WebSocket URL。
- **数据平面不变**：HA 的 `agent_pod_sessions` + DERP relay 机制保持原样；Team 新增 `cluster_name` 字段路由调度目标。

**Tech Stack:** Go 1.24+, Karmada v1.9+, tailscale.com (BSD-3), PostgreSQL, Kubernetes 1.28+, Helm 3

---

## 架构总览

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Hub 集群（现有）                              │
│                                                                     │
│  ┌──────────────┐  ┌─────────────────────────────────────────────┐  │
│  │ Karmada 控制面 │  │         KubeRDE 控制面（不变）                 │  │
│  │              │  │                                             │  │
│  │ karmada-     │  │  Server×3  PostgreSQL  Keycloak  Web UI    │  │
│  │ apiserver    │  │  (HA+DERP)  (共享状态)                        │  │
│  │ controller   │  │                                             │  │
│  │ scheduler    │  └─────────────────────────────────────────────┘  │
│  └──────┬───────┘                                                  │
│         │ PropagationPolicy / OverridePolicy                       │
└─────────┼───────────────────────────────────────────────────────────┘
          │
     ┌────┼─────────────────────────────────┐
     │    │                                 │
     ▼    ▼                                 ▼
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│  Cluster-A  │   │  Cluster-B  │   │  Cluster-C  │
│ (Team Alpha)│   │ (Team Beta) │   │  (GPU Pool) │
│             │   │             │   │             │
│ Operator    │   │ Operator    │   │ Operator    │
│ Agent Pods  │   │ Agent Pods  │   │ Agent Pods  │
│ (kuberde-   │   │ (kuberde-   │   │ (kuberde-   │
│  alpha)     │   │  beta)      │   │  gpu-pool)  │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       │                 │                 │
       └─────────────────┴─────────────────┘
                         │
              WebSocket + JWT（回连 Hub Server 公网 URL）
              agent_pod_sessions 记录 Hub Pod IP（不变）
```

### 关键设计决策

| 问题 | 决策 | 原因 |
|------|------|------|
| Operator 部署位置 | 每个成员集群独立部署 | 代码零修改；Karmada 下发 CR，Operator 本地 reconcile |
| 跨集群通信 | Agent 直连 Hub 公网 URL | 现有 HA 机制完全复用，`agent_pod_sessions` 不需改 |
| 命名空间管理 | Karmada ClusterPropagationPolicy 下发 NS+Secret | 统一在 Hub 声明，各集群自动同步 |
| 存储类适配 | OverridePolicy 按集群替换 storageClassName | 各集群存储不同，无需修改 RDEAgent spec |
| DERP 路由 | 保持 `/derp-pod/{podIP}` 机制不变 | Agent 在成员集群，但 Yamux/DERP session 在 Hub Pod |
| 租户隔离级别 | Namespace 隔离（现有）+ Cluster 隔离（新增） | 高价值租户可独占集群 |

---

## Phase 1：Karmada 基础设施安装

### Task 1.1：Hub 集群安装 Karmada 控制面

**Files:**
- 新建: `deploy/karmada/00-install.sh`
- 新建: `deploy/karmada/01-karmada-values.yaml`

**Step 1: 创建安装脚本**

```bash
# deploy/karmada/00-install.sh
#!/bin/bash
set -euo pipefail

NAMESPACE="karmada-system"
KARMADA_VERSION="v1.9.0"

echo "==> Installing Karmada control plane on hub cluster..."

# Install Karmada via helm
helm repo add karmada-charts https://raw.githubusercontent.com/karmada-io/karmada/master/charts
helm repo update

helm install karmada karmada-charts/karmada \
  --namespace ${NAMESPACE} \
  --create-namespace \
  --version ${KARMADA_VERSION} \
  -f deploy/karmada/01-karmada-values.yaml \
  --wait --timeout=10m

echo "==> Waiting for Karmada API server to be ready..."
kubectl wait --for=condition=Ready pod \
  -l app=karmada-apiserver \
  -n ${NAMESPACE} \
  --timeout=300s

echo "==> Extracting karmada kubeconfig..."
kubectl get secret karmada-kubeconfig \
  -n ${NAMESPACE} \
  -o jsonpath='{.data.kubeconfig}' | base64 -d > /tmp/karmada.kubeconfig

echo "✓ Karmada installed. Kubeconfig saved to /tmp/karmada.kubeconfig"
echo "  Export: export KUBECONFIG=/tmp/karmada.kubeconfig"
```

**Step 2: 创建 Karmada Helm values**

```yaml
# deploy/karmada/01-karmada-values.yaml
apiServer:
  hostNetwork: false
  serviceType: ClusterIP      # Hub 内部访问；Ingress 在后续步骤暴露

etcd:
  mode: internal              # 使用内嵌 etcd；生产建议外置

controllerManager:
  replicaCount: 2

scheduler:
  replicaCount: 2

webhook:
  replicaCount: 2
```

**Step 3: 运行安装**
```bash
chmod +x deploy/karmada/00-install.sh
./deploy/karmada/00-install.sh
```

期望输出：`✓ Karmada installed.`

**Step 4: 验证**
```bash
export KUBECONFIG=/tmp/karmada.kubeconfig
kubectl get clusters   # 空列表，尚未注册成员集群
kubectl api-resources | grep karmada
```

期望：有 `clusters`, `propagationpolicies`, `clusterpropagationpolicies` 等资源。

**Step 5: Commit**
```bash
git add deploy/karmada/
git commit -m "feat(karmada): add Karmada control plane installation scripts"
```

---

### Task 1.2：注册成员集群

**Files:**
- 新建: `deploy/karmada/02-join-clusters.sh`

**Step 1: 创建 join 脚本**

```bash
# deploy/karmada/02-join-clusters.sh
#!/bin/bash
# Usage: ./02-join-clusters.sh <cluster-name> <kubeconfig-path>
# Example: ./02-join-clusters.sh cluster-gpu-east ~/.kube/cluster-gpu-east.yaml
set -euo pipefail

CLUSTER_NAME="${1:?Usage: $0 <cluster-name> <kubeconfig-path>}"
CLUSTER_KUBECONFIG="${2:?Usage: $0 <cluster-name> <kubeconfig-path>}"
KARMADA_KUBECONFIG="/tmp/karmada.kubeconfig"

echo "==> Joining cluster: ${CLUSTER_NAME}"

# karmadactl 使用 Karmada kubeconfig 注册成员集群
karmadactl join "${CLUSTER_NAME}" \
  --kubeconfig="${KARMADA_KUBECONFIG}" \
  --cluster-kubeconfig="${CLUSTER_KUBECONFIG}"

echo "==> Verifying cluster registration..."
KUBECONFIG="${KARMADA_KUBECONFIG}" kubectl get cluster "${CLUSTER_NAME}"

echo "✓ Cluster ${CLUSTER_NAME} registered successfully"
```

**Step 2: 注册第一个成员集群（示例）**
```bash
chmod +x deploy/karmada/02-join-clusters.sh
./deploy/karmada/02-join-clusters.sh cluster-a ~/.kube/cluster-a.yaml
./deploy/karmada/02-join-clusters.sh cluster-b ~/.kube/cluster-b.yaml
```

**Step 3: 验证**
```bash
export KUBECONFIG=/tmp/karmada.kubeconfig
kubectl get clusters
# NAME        VERSION   MODE   READY   AGE
# cluster-a   v1.28.x   Push   True    30s
# cluster-b   v1.28.x   Push   True    15s
```

**Step 4: Commit**
```bash
git add deploy/karmada/02-join-clusters.sh
git commit -m "feat(karmada): add cluster join script"
```

---

### Task 1.3：将 RDEAgent CRD 下发到所有成员集群

**Files:**
- 新建: `deploy/karmada/03-crd-propagation.yaml`

**Step 1: 创建 ClusterPropagationPolicy**

```yaml
# deploy/karmada/03-crd-propagation.yaml
---
# 将 RDEAgent CRD 下发到所有成员集群
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: kuberde-crd-propagation
spec:
  resourceSelectors:
  - apiVersion: apiextensions.k8s.io/v1
    kind: CustomResourceDefinition
    name: rdeagents.kuberde.io
  placement:
    clusterAffinity:
      matchExpressions:
      - key: kuberde.io/member
        operator: In
        values:
        - "true"
---
# 将 kuberde-agents-auth Secret 模板下发（各命名空间由 NamespacePropagation 复制）
# 具体 Secret 下发见 Task 3.2
```

**Step 2: 给成员集群打标签并应用**
```bash
export KUBECONFIG=/tmp/karmada.kubeconfig

# 给所有成员集群打标签
kubectl label cluster cluster-a kuberde.io/member=true
kubectl label cluster cluster-b kuberde.io/member=true

# 应用 CRD 传播策略（在 hub 集群，使用本地 kubeconfig）
kubectl apply -f deploy/karmada/03-crd-propagation.yaml \
  --kubeconfig=/tmp/karmada.kubeconfig
```

**Step 3: 验证 CRD 在成员集群中可用**
```bash
kubectl get crd rdeagents.kuberde.io \
  --kubeconfig=~/.kube/cluster-a.yaml
# NAME                    CREATED AT
# rdeagents.kuberde.io    2026-03-12T...
```

**Step 4: Commit**
```bash
git add deploy/karmada/03-crd-propagation.yaml
git commit -m "feat(karmada): propagate RDEAgent CRD to all member clusters"
```

---

### Task 1.4：在各成员集群部署 KubeRDE Operator

**Files:**
- 新建: `deploy/karmada/04-operator-propagation.yaml`

**Step 1: 为 Operator 创建 ClusterPropagationPolicy**

```yaml
# deploy/karmada/04-operator-propagation.yaml
# 将 Operator Deployment、ServiceAccount、ClusterRole、ClusterRoleBinding
# 下发到所有成员集群——Operator 代码无需修改，就地 watch 本集群 RDEAgent CRs
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: kuberde-operator-propagation
spec:
  resourceSelectors:
  - apiVersion: apps/v1
    kind: Deployment
    name: kuberde-operator
    namespace: kuberde
  - apiVersion: v1
    kind: ServiceAccount
    name: kuberde-operator
    namespace: kuberde
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    name: kuberde-operator-role
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    name: kuberde-operator-binding
  placement:
    clusterAffinity:
      matchExpressions:
      - key: kuberde.io/member
        operator: In
        values:
        - "true"
```

**Step 2: 配置 Operator 的 SERVER_BASE_URL 指向 Hub**

Operator 只需将其 `SERVER_BASE_URL` 指向 Hub 公网地址，以便轮询 `/mgmt/agents/{id}`。
修改 `deploy/k8s/04-operator.yaml`，确保该环境变量已配置（一般已有）：
```yaml
env:
- name: SERVER_BASE_URL
  value: "https://frp.byai.uk"   # Hub 公网地址
```

**Step 3: 应用**
```bash
kubectl apply -f deploy/karmada/04-operator-propagation.yaml \
  --kubeconfig=/tmp/karmada.kubeconfig
```

**Step 4: 验证**
```bash
# 成员集群 cluster-a 上看到 operator pod
kubectl get pods -n kuberde -l app=kuberde-operator \
  --kubeconfig=~/.kube/cluster-a.yaml
```

**Step 5: Commit**
```bash
git add deploy/karmada/04-operator-propagation.yaml
git commit -m "feat(karmada): propagate operator to all member clusters"
```

---

## Phase 2：数据模型扩展

### Task 2.1：Team 模型添加 cluster_name 字段

**Files:**
- 修改: `pkg/models/models.go:171-184`（Team struct）
- 新建: `deploy/migrations/004_team_cluster.sql`

**Step 1: 创建数据库迁移**

```sql
-- deploy/migrations/004_team_cluster.sql
-- +goose Up
ALTER TABLE teams
    ADD COLUMN IF NOT EXISTS cluster_name TEXT NOT NULL DEFAULT 'default';

-- 更新索引注释：cluster_name 表示 Karmada 成员集群名称
-- 'default' = 使用 Hub 集群本地调度（向后兼容）
COMMENT ON COLUMN teams.cluster_name IS 'Karmada member cluster name; "default" = hub cluster local scheduling';

-- +goose Down
ALTER TABLE teams DROP COLUMN IF EXISTS cluster_name;
```

**Step 2: 更新 Team 模型**

修改 `pkg/models/models.go` 中的 `Team` struct（当前 line 171）：

```go
// Team represents an organizational unit that owns a Kubernetes namespace
type Team struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    Name        string    `gorm:"uniqueIndex;not null" json:"name"`
    DisplayName string    `json:"display_name"`
    Namespace   string    `gorm:"uniqueIndex;not null" json:"namespace"`
    ClusterName string    `gorm:"not null;default:'default'" json:"cluster_name"` // Karmada member cluster
    Status      string    `gorm:"default:'active'" json:"status"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

**Step 3: 验证编译**
```bash
go build ./pkg/...
go build ./cmd/server/...
```
期望：编译成功，无错误。

**Step 4: Commit**
```bash
git add pkg/models/models.go deploy/migrations/004_team_cluster.sql
git commit -m "feat(karmada): add cluster_name to Team model for multi-cluster scheduling"
```

---

### Task 2.2：agent_pod_sessions 添加 cluster_name（可观测性）

**Files:**
- 新建: `deploy/migrations/005_session_cluster.sql`
- 修改: `pkg/models/models.go`（AgentPodSession struct）
- 修改: `pkg/repositories/agent_pod_session_repository.go`

**Step 1: 创建迁移**

```sql
-- deploy/migrations/005_session_cluster.sql
-- +goose Up
ALTER TABLE agent_pod_sessions
    ADD COLUMN IF NOT EXISTS cluster_name TEXT NOT NULL DEFAULT 'default';

-- +goose Down
ALTER TABLE agent_pod_sessions DROP COLUMN IF EXISTS cluster_name;
```

**Step 2: 更新 AgentPodSession 模型**

在 `pkg/models/models.go` 中更新 `AgentPodSession` struct：

```go
type AgentPodSession struct {
    AgentID     string    `gorm:"primaryKey" json:"agent_id"`
    PodIP       string    `gorm:"not null" json:"pod_ip"`
    PodPort     int       `gorm:"not null" json:"pod_port"`
    ClusterName string    `gorm:"not null;default:'default'" json:"cluster_name"` // 新增
    UpdatedAt   time.Time `json:"updated_at"`
}
```

**Step 3: 更新 Upsert 以包含 cluster_name**

修改 `pkg/repositories/agent_pod_session_repository.go` 中的 `Upsert`：

```go
func (r *agentPodSessionRepository) Upsert(s *models.AgentPodSession) error {
    return r.db.Clauses(clause.OnConflict{
        Columns:   []clause.Column{{Name: "agent_id"}},
        DoUpdates: clause.AssignmentColumns([]string{"pod_ip", "pod_port", "cluster_name", "updated_at"}),
    }).Create(s).Error
}
```

**Step 4: 编译并验证**
```bash
go build ./...
```

**Step 5: Commit**
```bash
git add deploy/migrations/005_session_cluster.sql \
        pkg/models/models.go \
        pkg/repositories/agent_pod_session_repository.go
git commit -m "feat(karmada): track cluster_name in agent_pod_sessions for observability"
```

---

### Task 2.3：Karmada API 客户端初始化（Server）

**Files:**
- 修改: `cmd/server/main.go`（全局变量和 init）

**Step 1: 添加 Karmada Go 依赖**

```bash
go get github.com/karmada-io/karmada@v1.9.0
```

**Step 2: 在 main.go 全局变量区新增 Karmada 客户端**

在现有 `dynamicClient` 声明附近（约 line 68）添加：

```go
var (
    // karmadaClient 用于在 Karmada 控制面创建/删除 PropagationPolicy 和 OverridePolicy
    // 若 KARMADA_APISERVER_URL 未设置则为 nil（单集群模式，向后兼容）
    karmadaClient dynamic.Interface

    // karmadaEnabled 标识是否启用 Karmada 多集群模式
    karmadaEnabled bool
)

// Karmada GVR 常量
var (
    propagationPolicyGVR = schema.GroupVersionResource{
        Group:    "policy.karmada.io",
        Version:  "v1alpha1",
        Resource: "propagationpolicies",
    }
    overridePolicyGVR = schema.GroupVersionResource{
        Group:    "policy.karmada.io",
        Version:  "v1alpha1",
        Resource: "overridepolicies",
    }
    clusterPropagationPolicyGVR = schema.GroupVersionResource{
        Group:    "policy.karmada.io",
        Version:  "v1alpha1",
        Resource: "clusterpropagationpolicies",
    }
)
```

**Step 3: 在 `init()` 中初始化 Karmada 客户端**

在现有 `init()` 函数中，k8sClientset 初始化之后添加：

```go
// 初始化 Karmada 客户端（可选，向后兼容）
karmadaAPIServerURL := os.Getenv("KARMADA_APISERVER_URL")
karmadaKubeconfig := os.Getenv("KARMADA_KUBECONFIG")

if karmadaAPIServerURL != "" || karmadaKubeconfig != "" {
    var karmadaCfg *rest.Config
    var err error
    if karmadaKubeconfig != "" {
        karmadaCfg, err = clientcmd.BuildConfigFromFlags("", karmadaKubeconfig)
    } else {
        karmadaCfg, err = clientcmd.BuildConfigFromFlags(karmadaAPIServerURL, "")
    }
    if err != nil {
        log.Printf("WARNING: Failed to build Karmada config: %v — running in single-cluster mode", err)
    } else {
        karmadaClient, err = dynamic.NewForConfig(karmadaCfg)
        if err != nil {
            log.Printf("WARNING: Failed to create Karmada dynamic client: %v", err)
        } else {
            karmadaEnabled = true
            log.Printf("✓ Karmada multi-cluster mode enabled")
        }
    }
} else {
    log.Printf("ℹ  KARMADA_APISERVER_URL not set — running in single-cluster mode")
}
```

**Step 4: 编译验证**
```bash
go build ./cmd/server/...
```

**Step 5: Commit**
```bash
git add cmd/server/main.go go.mod go.sum
git commit -m "feat(karmada): add Karmada dynamic client initialization to server"
```

---

## Phase 3：核心业务逻辑——PropagationPolicy 管理

### Task 3.1：团队命名空间下发（创建团队时）

**Files:**
- 修改: `cmd/server/main.go`（`createTeamNamespace` 函数，约 line 8396）

**当前行为**：`createTeamNamespace` 调用 `k8sClientset.CoreV1().Namespaces().Create()` 只在 Hub 本地创建命名空间。

**新行为**：若 `karmadaEnabled` 且 `team.ClusterName != "default"`，额外在 Karmada 控制面创建 `ClusterPropagationPolicy` 将命名空间下发到成员集群。

**Step 1: 在 `createTeamNamespace` 末尾添加 Karmada 传播逻辑**

```go
// createTeamNamespace 创建 K8s 命名空间，并在多集群模式下通过 Karmada 传播到成员集群
func createTeamNamespace(team *models.Team) error {
    if k8sClientset == nil {
        return fmt.Errorf("kubernetes client not initialized")
    }

    ns := &corev1.Namespace{
        ObjectMeta: metav1.ObjectMeta{
            Name: team.Namespace,
            Labels: map[string]string{
                "kuberde.io/team":      team.Name,
                "kuberde.io/component": "team-namespace",
            },
        },
    }
    _, err := k8sClientset.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
    if err != nil && !strings.Contains(err.Error(), "already exists") {
        return err
    }

    // 复制 agent auth secret
    if err := copyAgentAuthSecretToNamespace(team.Namespace); err != nil {
        log.Printf("WARNING: Failed to copy agent auth secret to team namespace: %v", err)
    }

    // Karmada 多集群模式：将命名空间和 Secret 传播到成员集群
    if karmadaEnabled && team.ClusterName != "" && team.ClusterName != "default" {
        if err := propagateTeamNamespaceToCluster(team); err != nil {
            log.Printf("WARNING: Failed to propagate namespace %s to cluster %s: %v",
                team.Namespace, team.ClusterName, err)
            // 不返回错误：命名空间已在 Hub 创建，传播失败不阻断主流程
        }
    }
    return nil
}

// propagateTeamNamespaceToCluster 创建 Karmada ClusterPropagationPolicy 将团队命名空间
// 及其 auth secret 下发到指定成员集群
func propagateTeamNamespaceToCluster(team *models.Team) error {
    policyName := "team-" + team.Name + "-ns"
    policy := &unstructured.Unstructured{
        Object: map[string]interface{}{
            "apiVersion": "policy.karmada.io/v1alpha1",
            "kind":       "ClusterPropagationPolicy",
            "metadata": map[string]interface{}{
                "name": policyName,
                "labels": map[string]interface{}{
                    "kuberde.io/team": team.Name,
                },
            },
            "spec": map[string]interface{}{
                "resourceSelectors": []interface{}{
                    map[string]interface{}{
                        "apiVersion": "v1",
                        "kind":       "Namespace",
                        "name":       team.Namespace,
                    },
                    map[string]interface{}{
                        "apiVersion": "v1",
                        "kind":       "Secret",
                        "name":       os.Getenv("KUBERDE_AGENT_AUTH_SECRET"),
                        "namespace":  team.Namespace,
                    },
                },
                "placement": map[string]interface{}{
                    "clusterAffinity": map[string]interface{}{
                        "clusterNames": []interface{}{team.ClusterName},
                    },
                },
            },
        },
    }

    _, err := karmadaClient.Resource(clusterPropagationPolicyGVR).
        Create(context.Background(), policy, metav1.CreateOptions{})
    if err != nil && !strings.Contains(err.Error(), "already exists") {
        return fmt.Errorf("create ClusterPropagationPolicy: %w", err)
    }
    log.Printf("✓ Propagated namespace %s to cluster %s", team.Namespace, team.ClusterName)
    return nil
}
```

**Step 2: 编译验证**
```bash
go build ./cmd/server/...
```

**Step 3: Commit**
```bash
git add cmd/server/main.go
git commit -m "feat(karmada): propagate team namespace and auth secret to member cluster on team creation"
```

---

### Task 3.2：RDEAgent CR 创建时附加 PropagationPolicy

**Files:**
- 修改: `cmd/server/main.go`（`createRDEAgentFromTemplate` 函数，约 line 6577）

**当前行为**：只在 Karmada 控制面（或 Hub 本地）创建 RDEAgent CR。
**新行为**：同时创建同名 `PropagationPolicy`，将 CR 路由到团队所在成员集群。

**Step 1: 在 `createRDEAgentFromTemplate` 末尾（创建 CR 成功后）添加**

```go
// 若 Karmada 已启用且团队指定了成员集群，创建 PropagationPolicy
if karmadaEnabled && team != nil && team.ClusterName != "" && team.ClusterName != "default" {
    if err := createRDEAgentPropagationPolicy(ctx, crName, targetNamespace, team.ClusterName); err != nil {
        // 非致命：CR 已创建；传播失败时 Agent 仍可在 Hub 本地调度
        log.Printf("WARNING: Failed to create PropagationPolicy for %s: %v", crName, err)
    }
}
```

**Step 2: 实现 `createRDEAgentPropagationPolicy`**

```go
// createRDEAgentPropagationPolicy 在 Karmada 控制面创建 PropagationPolicy，
// 将指定 RDEAgent CR 路由到目标成员集群
func createRDEAgentPropagationPolicy(ctx context.Context, agentName, namespace, clusterName string) error {
    policy := &unstructured.Unstructured{
        Object: map[string]interface{}{
            "apiVersion": "policy.karmada.io/v1alpha1",
            "kind":       "PropagationPolicy",
            "metadata": map[string]interface{}{
                "name":      agentName, // 与 RDEAgent 同名，便于一一对应
                "namespace": namespace,
                "labels": map[string]interface{}{
                    "kuberde.io/agent": agentName,
                },
            },
            "spec": map[string]interface{}{
                "resourceSelectors": []interface{}{
                    map[string]interface{}{
                        "apiVersion": "kuberde.io/v1beta1",
                        "kind":       "RDEAgent",
                        "name":       agentName,
                    },
                },
                "placement": map[string]interface{}{
                    "clusterAffinity": map[string]interface{}{
                        "clusterNames": []interface{}{clusterName},
                    },
                },
                // 传播时保留 CR 中的 spec 不变；OverridePolicy 处理存储类差异
                "propagateDeps": true,
            },
        },
    }

    _, err := karmadaClient.Resource(propagationPolicyGVR).
        Namespace(namespace).
        Create(ctx, policy, metav1.CreateOptions{})
    if err != nil && !strings.Contains(err.Error(), "already exists") {
        return fmt.Errorf("create PropagationPolicy: %w", err)
    }
    log.Printf("✓ Created PropagationPolicy for agent %s → cluster %s", agentName, clusterName)
    return nil
}
```

**Step 3: 编译**
```bash
go build ./cmd/server/...
```

**Step 4: Commit**
```bash
git add cmd/server/main.go
git commit -m "feat(karmada): create PropagationPolicy alongside RDEAgent CR for member cluster routing"
```

---

### Task 3.3：删除 Agent 时清理 PropagationPolicy

**Files:**
- 修改: `cmd/server/main.go`（删除 RDEAgent 的相关函数，约 line 5308）

**Step 1: 在删除 RDEAgent CR 之后添加 PropagationPolicy 清理**

找到 `dynamicClient.Resource(frpAgentGVR).Namespace(targetNamespace).Delete(...)` 调用处，在其后添加：

```go
// 清理对应的 Karmada PropagationPolicy
if karmadaEnabled {
    err := karmadaClient.Resource(propagationPolicyGVR).
        Namespace(targetNamespace).
        Delete(ctx, service.AgentID, metav1.DeleteOptions{})
    if err != nil && !errors.IsNotFound(err) {
        log.Printf("WARNING: Failed to delete PropagationPolicy %s: %v", service.AgentID, err)
    } else {
        log.Printf("✓ Deleted PropagationPolicy %s", service.AgentID)
    }
}
```

**Step 2: 同理，删除工作区所有服务时（约 line 3330）也添加相同清理逻辑。**

**Step 3: 编译**
```bash
go build ./cmd/server/...
```

**Step 4: Commit**
```bash
git add cmd/server/main.go
git commit -m "feat(karmada): cleanup PropagationPolicy on agent/service deletion"
```

---

### Task 3.4：OverridePolicy——按集群适配存储类

**Files:**
- 新建: `deploy/karmada/05-override-policies.yaml`

**Step 1: 创建存储类适配 OverridePolicy 模板**

每个成员集群部署时，管理员根据集群实际存储类创建对应 OverridePolicy：

```yaml
# deploy/karmada/05-override-policies.yaml
# 示例：cluster-a 使用 premium-ssd 存储类
---
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: cluster-a-storage-override
  namespace: kuberde-alpha   # 只影响该团队命名空间
spec:
  resourceSelectors:
  - apiVersion: v1
    kind: PersistentVolumeClaim
  targetCluster:
    clusterNames:
    - cluster-a
  overriders:
    plaintext:
    - path: "/spec/storageClassName"
      op: replace
      value: "premium-ssd"   # cluster-a 实际存储类名
---
# cluster-b（GPU 集群）使用 local-nvme 存储类
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: cluster-b-storage-override
  namespace: kuberde-beta
spec:
  resourceSelectors:
  - apiVersion: v1
    kind: PersistentVolumeClaim
  targetCluster:
    clusterNames:
    - cluster-b
  overriders:
    plaintext:
    - path: "/spec/storageClassName"
      op: replace
      value: "local-nvme"
```

**Step 2: 说明**

OverridePolicy 仅在特定集群上生效，不影响其他集群的 PVC。当 PropagationPolicy 将 PVC 下发到 cluster-a 时，Karmada 自动应用 `cluster-a-storage-override`，将 storageClassName 替换为集群实际支持的值。

**Step 3: Commit**
```bash
git add deploy/karmada/05-override-policies.yaml
git commit -m "feat(karmada): add per-cluster storage class OverridePolicy examples"
```

---

## Phase 4：Server 部署配置更新

### Task 4.1：更新 Server Deployment 添加 Karmada 环境变量

**Files:**
- 修改: `deploy/k8s/03-server.yaml`

**Step 1: 在 Server Deployment env 段添加**

```yaml
# 在现有环境变量后添加（约 line 98）
- name: KARMADA_APISERVER_URL
  value: "https://karmada-apiserver.karmada-system.svc.cluster.local:5443"
  # 留空则单集群模式（向后兼容）
# 或使用 Secret 挂载 kubeconfig：
# - name: KARMADA_KUBECONFIG
#   value: "/etc/karmada/kubeconfig"
```

**Step 2: 若使用 kubeconfig 文件方式（推荐生产环境），添加 Secret + Volume**

```yaml
# 在 spec.template.spec.volumes 添加：
volumes:
- name: karmada-kubeconfig
  secret:
    secretName: karmada-kubeconfig   # 由 Task 1.1 生成

# 在 container.volumeMounts 添加：
volumeMounts:
- name: karmada-kubeconfig
  mountPath: /etc/karmada
  readOnly: true

# 对应环境变量：
- name: KARMADA_KUBECONFIG
  value: "/etc/karmada/kubeconfig"
```

**Step 3: 将 Karmada kubeconfig 保存为 Secret**
```bash
kubectl create secret generic karmada-kubeconfig \
  --from-file=kubeconfig=/tmp/karmada.kubeconfig \
  -n kuberde
```

**Step 4: Commit**
```bash
git add deploy/k8s/03-server.yaml
git commit -m "feat(karmada): add KARMADA_APISERVER_URL env and kubeconfig volume to server deployment"
```

---

### Task 4.2：Ingress 更新——Karmada apiserver 不对外暴露

**Files:**
- 修改: `deploy/karmada/00-install.sh`（说明 Karmada apiserver 仅集群内访问）
- 无需修改 `deploy/k8s/05-ingress.yaml`

Karmada apiserver 只在 Hub 集群内部访问（`karmada-apiserver.karmada-system.svc:5443`），不需要通过外部 Ingress 暴露。这是正确的安全设计。

---

## Phase 5：Web UI 更新

### Task 5.1：团队管理页面添加集群选择

**Files:**
- 修改: `web/pages/` 中的团队管理相关页面（如 `TeamManagement.tsx` 或 `UserManagement.tsx`）

**Step 1: 在团队创建/编辑表单中添加集群选择下拉框**

在团队创建 API 请求体中新增 `cluster_name` 字段。前端从 `/api/clusters`（新 API，见 Task 5.2）获取可用集群列表。

**Team 创建表单新增字段**（在现有 DisplayName、Namespace 之后）：

```tsx
{/* Cluster Selection */}
<div>
  <label className="block text-xs font-bold uppercase tracking-widest text-text-secondary mb-2">
    Target Cluster
  </label>
  <select
    value={formData.cluster_name || 'default'}
    onChange={(e) => setFormData({ ...formData, cluster_name: e.target.value })}
    className="w-full bg-background-dark border border-border-dark rounded-lg px-3 py-2 text-sm text-text-foreground"
  >
    <option value="default">Hub Cluster (default)</option>
    {clusters.map((c) => (
      <option key={c.name} value={c.name}>{c.name} — {c.status}</option>
    ))}
  </select>
  <p className="mt-1 text-xs text-text-secondary/60">
    Agent workloads for this team will be scheduled to this cluster.
  </p>
</div>
```

**Step 2: 在 Workspace/Service 详情页显示集群信息**

在 Service 状态卡片中添加集群标签（从 team 信息中读取）：

```tsx
{team?.cluster_name && team.cluster_name !== 'default' && (
  <div className="flex items-center gap-1.5 text-xs text-text-secondary/70">
    <span className="material-symbols-outlined text-[14px]">hub</span>
    <span>Cluster: <code className="font-mono">{team.cluster_name}</code></span>
  </div>
)}
```

**Step 3: Commit**
```bash
git add web/
git commit -m "feat(karmada): add cluster selection to team management UI"
```

---

### Task 5.2：新增 /api/clusters 端点（Server）

**Files:**
- 修改: `cmd/server/main.go`（新增 handler）

**Step 1: 实现 `handleListClusters`**

```go
// GET /api/clusters — 返回已注册的 Karmada 成员集群列表
// 管理员专用；用于 Web UI 团队创建时选择目标集群
func handleListClusters(w http.ResponseWriter, r *http.Request) {
    if !isAdmin(r) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    if !karmadaEnabled {
        // 单集群模式：返回只有 "default" 的列表
        writeJSON(w, []map[string]interface{}{
            {"name": "default", "status": "Ready", "version": ""},
        })
        return
    }

    clusterGVR := schema.GroupVersionResource{
        Group: "cluster.karmada.io", Version: "v1alpha1", Resource: "clusters",
    }
    clusterList, err := karmadaClient.Resource(clusterGVR).List(context.Background(), metav1.ListOptions{})
    if err != nil {
        http.Error(w, "Failed to list clusters: "+err.Error(), http.StatusInternalServerError)
        return
    }

    var result []map[string]interface{}
    for _, c := range clusterList.Items {
        status := "Unknown"
        if conditions, ok := c.Object["status"].(map[string]interface{}); ok {
            if conds, ok := conditions["conditions"].([]interface{}); ok {
                for _, cond := range conds {
                    if cm, ok := cond.(map[string]interface{}); ok &&
                        cm["type"] == "Ready" && cm["status"] == "True" {
                        status = "Ready"
                    }
                }
            }
        }
        result = append(result, map[string]interface{}{
            "name":   c.GetName(),
            "status": status,
        })
    }

    writeJSON(w, result)
}
```

**Step 2: 在路由中注册**

在 `routeMainDomain()` 中添加（与其他 `/api/` 路由并列）：

```go
case path == "/api/clusters":
    handleListClusters(w, r)
```

**Step 3: 编译**
```bash
go build ./cmd/server/...
```

**Step 4: Commit**
```bash
git add cmd/server/main.go
git commit -m "feat(karmada): add GET /api/clusters endpoint for cluster selection UI"
```

---

## Phase 6：部署与验证

### Task 6.1：完整部署清单

**Files:**
- 新建: `deploy/karmada/README.md`

```markdown
# Karmada 多集群部署指南

## 前置条件
- Hub 集群已部署 KubeRDE（单集群 HA 版本）
- 各成员集群 kubeconfig 可访问
- karmadactl CLI 已安装：`curl -s https://raw.githubusercontent.com/karmada-io/karmada/master/hack/install-cli.sh | sudo bash`

## 部署步骤

### 1. Hub 集群安装 Karmada 控制面
\`\`\`bash
./deploy/karmada/00-install.sh
\`\`\`

### 2. 注册成员集群
\`\`\`bash
./deploy/karmada/02-join-clusters.sh cluster-a ~/.kube/cluster-a.yaml
./deploy/karmada/02-join-clusters.sh cluster-b ~/.kube/cluster-b.yaml
\`\`\`

### 3. 下发 CRD 和 Operator 到成员集群
\`\`\`bash
export KUBECONFIG=/tmp/karmada.kubeconfig
kubectl label cluster cluster-a kuberde.io/member=true
kubectl label cluster cluster-b kuberde.io/member=true
kubectl apply -f deploy/karmada/03-crd-propagation.yaml
kubectl apply -f deploy/karmada/04-operator-propagation.yaml
\`\`\`

### 4. 保存 Karmada kubeconfig 为 Secret 并重部署 Server
\`\`\`bash
kubectl create secret generic karmada-kubeconfig \
  --from-file=kubeconfig=/tmp/karmada.kubeconfig \
  -n kuberde
make deploy-server
\`\`\`

### 5. 运行数据库迁移
\`\`\`bash
kubectl exec -n kuberde deploy/kuberde-server -- goose -dir /migrations postgres $DATABASE_URL up
\`\`\`

### 6. （可选）为各集群创建存储类 OverridePolicy
\`\`\`bash
kubectl apply -f deploy/karmada/05-override-policies.yaml --kubeconfig=/tmp/karmada.kubeconfig
\`\`\`
```

**Step 1: 端到端验证流程**

```bash
# 1. 创建团队，指定 cluster_name=cluster-a
curl -X POST https://frp.byai.uk/api/teams \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"alpha","display_name":"Alpha Team","cluster_name":"cluster-a"}'

# 期望：团队创建成功，Karmada 下发 namespace kuberde-alpha 到 cluster-a
KUBECONFIG=~/.kube/cluster-a.yaml kubectl get ns kuberde-alpha

# 2. 创建工作区并添加 SSH 服务
# （通过 Web UI 或 API）

# 3. 验证 RDEAgent CR 已下发到 cluster-a
KUBECONFIG=~/.kube/cluster-a.yaml kubectl get rdeagents -n kuberde-alpha

# 4. 验证 Operator 已在 cluster-a 创建 Deployment
KUBECONFIG=~/.kube/cluster-a.yaml kubectl get deployments -n kuberde-alpha

# 5. 验证 Agent Pod 已连接回 Hub
kubectl logs -n kuberde -l app=kuberde-server | grep "agent registered"

# 6. 验证 agent_pod_sessions 记录（Hub PostgreSQL）
psql $DATABASE_URL -c "SELECT agent_id, pod_ip, cluster_name FROM agent_pod_sessions;"

# 7. SSH 连接（不感知集群差异，透明）
ssh kuberde-agent-alpha-xxx
```

**Step 2: Commit**
```bash
git add deploy/karmada/README.md
git commit -m "docs(karmada): add multi-cluster deployment guide and verification steps"
```

---

## Phase 7：文档更新

### Task 7.1：更新 ARCHITECTURE.md 增加 Karmada 架构章节

**Files:**
- 修改: `docs/ARCHITECTURE.md`
- 修改: `docs/architecture.py`（添加多集群图）

**Step 1: 在 ARCHITECTURE.md 新增 Section 7: Multi-Cluster Architecture**

涵盖：
- Hub + Member 集群拓扑
- PropagationPolicy 路由决策
- OverridePolicy 存储适配
- 向后兼容（`cluster_name=default` 保持单集群行为）
- ADR-014: Karmada 多集群选型（vs. KubeFed、自研）
- ADR-015: Operator 下发到成员集群（vs. 中央化 Operator）

**Step 2: 在 architecture.py 新增 Diagram 4**

添加 `kuberde_multi_cluster` 图：Hub 集群 + 2-3 个成员集群，显示 Karmada 传播路径和 Agent 回连路径。

**Step 3: Commit**
```bash
git add docs/ARCHITECTURE.md docs/architecture.py
git commit -m "docs(karmada): add multi-cluster architecture section and diagram"
```

---

## 关键文件路径总览

| 文件 | 变更类型 | Phase | 核心内容 |
|------|---------|-------|---------|
| `deploy/karmada/00-install.sh` | **新建** | P1 | Karmada 控制面安装脚本 |
| `deploy/karmada/01-karmada-values.yaml` | **新建** | P1 | Helm values |
| `deploy/karmada/02-join-clusters.sh` | **新建** | P1 | 成员集群注册脚本 |
| `deploy/karmada/03-crd-propagation.yaml` | **新建** | P1 | CRD ClusterPropagationPolicy |
| `deploy/karmada/04-operator-propagation.yaml` | **新建** | P1 | Operator ClusterPropagationPolicy |
| `deploy/karmada/05-override-policies.yaml` | **新建** | P3 | 存储类 OverridePolicy 示例 |
| `deploy/karmada/README.md` | **新建** | P6 | 部署指南 |
| `deploy/migrations/004_team_cluster.sql` | **新建** | P2 | teams 表添加 cluster_name |
| `deploy/migrations/005_session_cluster.sql` | **新建** | P2 | agent_pod_sessions 添加 cluster_name |
| `pkg/models/models.go` | **修改** | P2 | Team + AgentPodSession 新增字段 |
| `pkg/repositories/agent_pod_session_repository.go` | **修改** | P2 | Upsert 包含 cluster_name |
| `cmd/server/main.go` | **修改** | P2,P3,P5 | Karmada 客户端 + PropagationPolicy 管理 + /api/clusters |
| `go.mod` / `go.sum` | **修改** | P2 | 添加 karmada-io/karmada 依赖 |
| `deploy/k8s/03-server.yaml` | **修改** | P4 | KARMADA_APISERVER_URL 环境变量 |
| `web/pages/` | **修改** | P5 | 团队管理添加集群选择 |
| `docs/ARCHITECTURE.md` | **修改** | P7 | ADR-014/015 + 多集群架构章节 |
| `docs/architecture.py` | **修改** | P7 | 多集群架构图 |

---

## 向后兼容保证

| 场景 | 行为 |
|------|------|
| `KARMADA_APISERVER_URL` 未设置 | `karmadaEnabled=false`，完全走原有单集群路径 |
| 现有团队 `cluster_name=default` | 不创建 PropagationPolicy，RDEAgent 在 Hub 本地调度 |
| `agent_pod_sessions.cluster_name` 缺失 | 默认 `'default'`，不影响现有 HA 路由 |
| 单集群 `GET /api/clusters` | 返回 `[{"name":"default","status":"Ready"}]` |

---

## 实施优先级

| 优先级 | Phase | 任务 | 风险 |
|--------|-------|------|------|
| **P0** | Phase 1 | Karmada 安装 + 集群注册 + CRD 传播 | 低（独立基础设施） |
| **P1** | Phase 2 | 数据模型 + Karmada 客户端 | 低（向后兼容） |
| **P2** | Phase 3.1-3.3 | PropagationPolicy CRUD | 中（核心业务逻辑） |
| **P3** | Phase 4-5 | 部署配置 + Web UI | 低 |
| **P4** | Phase 3.4 | OverridePolicy 存储适配 | 低（按需配置） |
| **P5** | Phase 6-7 | 端到端验证 + 文档 | 低 |
