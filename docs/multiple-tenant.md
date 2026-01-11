  1. 决策上下文分析

  核心决策问题

  主要决策：KubeRDE应该采用什么样的多租户隔离架构？

  当前状态：
  - 单一namespace（kuberde）部署模式
  - 所有用户的workspaces在同一namespace
  - 缺乏namespace级别的资源配额控制
  - 用户间隔离仅依赖RBAC和网络策略

  决策目标：
  1. 提供更强的租户隔离（安全性）
  2. 支持namespace级别的资源配额管理
  3. 支持不同团队/组织的独立部署
  4. 保持系统复杂度可控

  利益相关者分析

  | 角色       | 关注点                         | 优先级 |
  |------------|--------------------------------|--------|
  | 平台管理员 | 部署复杂度、运维成本、故障隔离 | 高     |
  | 企业租户   | 数据隔离、资源配额、成本分摊   | 高     |
  | 开发团队   | 实现复杂度、测试成本、维护负担 | 中     |
  | 最终用户   | 使用体验、性能、稳定性         | 中     |
  | 安全团队   | 租户隔离、合规性、审计         | 高     |

  2. 隔离架构方案梳理

  方案A：Namespace级隔离（推荐）

  架构设计：
  ┌─────────────────────────────────────────┐
  │         Kubernetes Cluster              │
  │                                         │
  │  ┌─────────────────┐  ┌──────────────┐ │
  │  │ Namespace:      │  │ Namespace:   │ │
  │  │ kuberde-team-a  │  │ kuberde-team-b│ │
  │  │                 │  │              │ │
  │  │ - Server (opt)  │  │ - Server     │ │
  │  │ - Operator(opt) │  │ - Operator   │ │
  │  │ - Agents        │  │ - Agents     │ │
  │  │ - PostgreSQL    │  │ - PostgreSQL │ │
  │  │ - Keycloak      │  │ - Keycloak   │ │
  │  │                 │  │              │ │
  │  │ ResourceQuota   │  │ ResourceQuota│ │
  │  │ NetworkPolicy   │  │ NetworkPolicy│ │
  │  └─────────────────┘  └──────────────┘ │
  │                                         │
  │  ┌─────────────────┐                   │
  │  │ Namespace:      │                   │
  │  │ kuberde-system  │ (共享组件)        │
  │  │ - Ingress       │                   │
  │  │ - Cert-manager  │                   │
  │  │ - Monitoring    │                   │
  │  └─────────────────┘                   │
  └─────────────────────────────────────────┘

  子方案A1：完全隔离模式（每租户独立组件）

  每个租户namespace包含：
  - 独立的Server实例
  - 独立的Operator实例
  - 独立的PostgreSQL
  - 独立的Keycloak（或共享）
  - 租户的所有Agents

  优点：
  - ✅ 完全隔离，故障不传播
  - ✅ 独立升级和配置
  - ✅ 清晰的资源归属
  - ✅ 支持不同版本并存

  缺点：
  - ❌ 资源占用高（每租户一套组件）
  - ❌ 运维复杂度增加
  - ❌ 成本较高

  子方案A2：共享控制平面模式（推荐）

  架构：
  ┌─────────────────────────────────────────┐
  │         Kubernetes Cluster              │
  │                                         │
  │  ┌─────────────────┐                   │
  │  │ kuberde-system  │ (共享控制平面)    │
  │  │ - Server (1个)  │                   │
  │  │ - Operator (1个)│                   │
  │  │ - PostgreSQL    │                   │
  │  │ - Keycloak      │                   │
  │  │ - Ingress       │                   │
  │  └─────────────────┘                   │
  │          ↓                              │
  │  ┌─────────────────┐  ┌──────────────┐ │
  │  │ team-a          │  │ team-b       │ │
  │  │ - Agent Pods    │  │ - Agent Pods │ │
  │  │ - PVCs          │  │ - PVCs       │ │
  │  │ ResourceQuota   │  │ ResourceQuota│ │
  │  │ LimitRange      │  │ LimitRange   │ │
  │  └─────────────────┘  └──────────────┘ │
  └─────────────────────────────────────────┘

  实现要点：
  1. Server通过tenant标识区分不同租户
  2. Operator watch所有租户namespace的RDEAgent CRD
  3. PostgreSQL使用schema或database隔离不同租户数据
  4. Keycloak使用realm隔离不同租户用户

  优点：
  - ✅ 资源利用率高
  - ✅ 运维成本低（1套控制平面）
  - ✅ 升级统一管理
  - ✅ 支持namespace级别配额

  缺点：
  - ⚠️ 控制平面单点故障
  - ⚠️ 租户间可能相互影响
  - ⚠️ 需要良好的租户标识设计

  子方案A3：混合模式

  - 小租户：共享控制平面
  - 大租户/VIP：独立namespace+独立组件

  方案B：Cluster级隔离

  架构：
  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐
  │   Cluster A    │  │   Cluster B    │  │   Cluster C    │
  │   (Team A)     │  │   (Team B)     │  │   (Team C)     │
  │                │  │                │  │                │
  │  kuberde ns    │  │  kuberde ns    │  │  kuberde ns    │
  │  - Server      │  │  - Server      │  │  - Server      │
  │  - Operator    │  │  - Operator    │  │  - Operator    │
  │  - Agents      │  │  - Agents      │  │  - Agents      │
  │  - PostgreSQL  │  │  - PostgreSQL  │  │  - PostgreSQL  │
  │  - Keycloak    │  │  - Keycloak    │  │  - Keycloak    │
  └────────────────┘  └────────────────┘  └────────────────┘

  优点：
  - ✅ 最强隔离性（网络、存储、计算完全隔离）
  - ✅ 故障完全隔离
  - ✅ 独立的K8s版本和配置
  - ✅ 符合某些合规要求

  缺点：
  - ❌ 成本最高（每租户一个集群）
  - ❌ 运维复杂度最高
  - ❌ 资源利用率低
  - ❌ 不适合小租户

  方案C：虚拟集群（vCluster）

  使用vCluster在一个物理集群上创建虚拟集群：

  ┌─────────────────────────────────────────┐
  │       Physical Kubernetes Cluster       │
  │                                         │
  │  ┌──────────────┐  ┌──────────────┐   │
  │  │  vCluster A  │  │  vCluster B  │   │
  │  │  (Team A)    │  │  (Team B)    │   │
  │  │              │  │              │   │
  │  │  kuberde ns  │  │  kuberde ns  │   │
  │  │  - Server    │  │  - Server    │   │
  │  │  - Agents    │  │  - Agents    │   │
  │  └──────────────┘  └──────────────┘   │
  │         ↓                  ↓           │
  │  ┌──────────────────────────────────┐ │
  │  │    Host Namespace (Workload)     │ │
  │  │    - Agent Pods                  │ │
  │  │    - PVCs                        │ │
  │  └──────────────────────────────────┘ │
  └─────────────────────────────────────────┘

  优点：
  - ✅ 接近cluster级隔离
  - ✅ 成本低于真实集群
  - ✅ 租户有完整的K8s API

  缺点：
  - ⚠️ 新兴技术，成熟度待验证
  - ⚠️ 额外的复杂度
  - ⚠️ 性能开销

  3. 决策树分析

  多租户隔离架构决策树：

  [起点] 需要多租户隔离
  │
  ├─[Q1] 租户规模和数量？
  │  ├─ 大量小租户（>50个）
  │  │  └─→ 方案A2: 共享控制平面 [概率: 60%]
  │  │      成本: 低 | 复杂度: 中 | 隔离: 中
  │  │      EV = 0.6 × 8分 = 4.8分
  │  │
  │  └─ 少量大租户（<10个）
  │     ├─→ 方案A1: 完全隔离 [概率: 25%]
  │     │   成本: 中 | 复杂度: 中 | 隔离: 高
  │     │   EV = 0.25 × 7分 = 1.75分
  │     │
  │     └─→ 方案B: Cluster隔离 [概率: 15%]
  │         成本: 高 | 复杂度: 高 | 隔离: 最高
  │         EV = 0.15 × 6分 = 0.9分
  │
  ├─[Q2] 合规要求严格程度？
  │  ├─ 高度合规（金融、医疗）
  │  │  └─→ 方案B: Cluster隔离 [必选]
  │  │
  │  └─ 一般合规
  │     └─→ 方案A: Namespace隔离 [可选]
  │
  ├─[Q3] 运维团队规模？
  │  ├─ 小团队（<5人）
  │  │  └─→ 方案A2: 共享控制平面 [优先]
  │  │
  │  └─ 大团队（>10人）
  │     └─→ 所有方案可选
  │
  └─[Q4] 预算约束？
     ├─ 紧张预算
     │  └─→ 方案A2: 共享控制平面 [唯一选择]
     │
     └─ 充足预算
        └─→ 根据需求选择

  综合期望值：
  - 方案A2（共享控制平面）: 4.8分 ⭐ 推荐
  - 方案A1（完全隔离）: 1.75分
  - 方案B（Cluster隔离）: 0.9分

  4. 多维度评分对比

  | 维度       | 方案A1 完全隔离 | 方案A2 共享控制平面 ⭐ | 方案B Cluster隔离 | 方案C vCluster |
  |------------|-----------------|------------------------|-------------------|----------------|
  | 隔离性     | 8/10            | 6/10                   | 10/10             | 9/10           |
  | 资源效率   | 5/10            | 9/10                   | 3/10              | 7/10           |
  | 成本       | 6/10            | 9/10                   | 3/10              | 7/10           |
  | 运维复杂度 | 6/10            | 8/10                   | 4/10              | 5/10           |
  | 实现复杂度 | 7/10            | 8/10                   | 6/10              | 5/10           |
  | 扩展性     | 7/10            | 9/10                   | 5/10              | 7/10           |
  | 故障隔离   | 9/10            | 6/10                   | 10/10             | 8/10           |
  | 配额管理   | 9/10            | 9/10                   | 10/10             | 9/10           |
  | 综合得分   | 7.1             | 8.0 ⭐                 | 6.4               | 7.1            |

  权重说明：
  - 隔离性：20%
  - 资源效率：15%
  - 成本：15%
  - 运维复杂度：15%
  - 实现复杂度：15%
  - 扩展性：10%
  - 故障隔离：5%
  - 配额管理：5%

  5. 推荐方案：A2 共享控制平面模式

  架构设计详细说明

  5.1 租户模型设计

  // pkg/models/tenant.go
  type Tenant struct {
      ID               uint      `gorm:"primaryKey" json:"id"`
      Name             string    `gorm:"uniqueIndex;not null" json:"name"` // team-a, org-b
      DisplayName      string    `json:"displayName"`
      Namespace        string    `gorm:"uniqueIndex;not null" json:"namespace"` // kuberde-team-a
      Type             string    `json:"type"` // shared, dedicated

      // 资源配额
      QuotaEnabled     bool      `json:"quotaEnabled"`
      CPUQuota         string    `json:"cpuQuota"`         // "100"
      MemoryQuota      string    `json:"memoryQuota"`      // "200Gi"
      StorageQuota     string    `json:"storageQuota"`     // "1Ti"
      PodsQuota        int       `json:"podsQuota"`        // 100
      ServicesQuota    int       `json:"servicesQuota"`    // 50

      // 限制范围
      DefaultCPURequest    string `json:"defaultCpuRequest"`    // "100m"
      DefaultMemoryRequest string `json:"defaultMemoryRequest"` // "128Mi"
      MaxCPULimit          string `json:"maxCpuLimit"`          // "8000m"
      MaxMemoryLimit       string `json:"maxMemoryLimit"`       // "32Gi"

      // Keycloak集成
      RealmName        string    `json:"realmName"`

      // 状态
      Status           string    `json:"status"` // active, suspended, deleted
      CreatedAt        time.Time `json:"createdAt"`
      UpdatedAt        time.Time `json:"updatedAt"`
  }

  5.2 Workspace与Tenant关联

  // pkg/models/models.go
  type Workspace struct {
      ID          uint      `gorm:"primaryKey" json:"id"`
      Name        string    `gorm:"not null;uniqueIndex:idx_tenant_owner_name" json:"name"`
      Owner       string    `gorm:"not null;index;uniqueIndex:idx_tenant_owner_name" json:"owner"`
      TenantID    uint      `gorm:"not null;index;uniqueIndex:idx_tenant_owner_name" json:"tenantId"` // 新增
      Tenant      Tenant    `gorm:"foreignKey:TenantID" json:"tenant"`

      // Namespace自动从Tenant继承
      Namespace   string    `gorm:"-" json:"namespace"` // 运行时填充

      StorageSize string    `gorm:"default:'10Gi'" json:"storageSize"`
      // ... 其他字段
  }

  5.3 Namespace资源配额配置

  # deploy/k8s/tenant/resourcequota.yaml
  apiVersion: v1
  kind: ResourceQuota
  metadata:
    name: tenant-quota
    namespace: {{ .TenantNamespace }}
  spec:
    hard:
      # 计算资源
      requests.cpu: "{{ .CPUQuota }}"              # 总CPU请求量
      requests.memory: "{{ .MemoryQuota }}"        # 总内存请求量
      limits.cpu: "{{ .CPUQuota }}"                # 总CPU限制
      limits.memory: "{{ .MemoryQuota }}"          # 总内存限制

      # 存储资源
      requests.storage: "{{ .StorageQuota }}"      # 总存储请求
      persistentvolumeclaims: "{{ .PVCQuota }}"    # PVC数量

      # 对象数量
      pods: "{{ .PodsQuota }}"                     # Pod数量
      services: "{{ .ServicesQuota }}"             # Service数量
      configmaps: "100"                            # ConfigMap数量
      secrets: "100"                               # Secret数量

      # GPU资源（如果需要）
      requests.nvidia.com/gpu: "{{ .GPUQuota }}"

  ---
  # deploy/k8s/tenant/limitrange.yaml
  apiVersion: v1
  kind: LimitRange
  metadata:
    name: tenant-limits
    namespace: {{ .TenantNamespace }}
  spec:
    limits:
    # Pod级别限制
    - type: Pod
      max:
        cpu: "{{ .MaxCPULimit }}"          # 单Pod最大CPU
        memory: "{{ .MaxMemoryLimit }}"     # 单Pod最大内存
      min:
        cpu: "10m"
        memory: "16Mi"

    # Container级别限制
    - type: Container
      default:  # 默认limits
        cpu: "1000m"
        memory: "1Gi"
      defaultRequest:  # 默认requests
        cpu: "{{ .DefaultCPURequest }}"
        memory: "{{ .DefaultMemoryRequest }}"
      max:  # 最大limits
        cpu: "{{ .MaxCPULimit }}"
        memory: "{{ .MaxMemoryLimit }}"
      min:  # 最小requests
        cpu: "10m"
        memory: "16Mi"

    # PVC级别限制
    - type: PersistentVolumeClaim
      max:
        storage: "{{ .MaxPVCSize }}"       # 单个PVC最大容量
      min:
        storage: "1Gi"

  5.4 网络隔离策略

  # deploy/k8s/tenant/networkpolicy.yaml
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    name: tenant-isolation
    namespace: {{ .TenantNamespace }}
  spec:
    podSelector: {}  # 应用于namespace中所有Pods
    policyTypes:
    - Ingress
    - Egress

    ingress:
    # 允许同namespace内通信
    - from:
      - podSelector: {}

    # 允许来自kuberde-system的控制平面流量
    - from:
      - namespaceSelector:
          matchLabels:
            name: kuberde-system
      ports:
      - protocol: TCP
        port: 8080  # Server API

    # 允许来自ingress-nginx的流量
    - from:
      - namespaceSelector:
          matchLabels:
            name: ingress-nginx

    egress:
    # 允许同namespace内通信
    - to:
      - podSelector: {}

    # 允许访问kuberde-system的控制平面
    - to:
      - namespaceSelector:
          matchLabels:
            name: kuberde-system
      ports:
      - protocol: TCP
        port: 5432  # PostgreSQL
      - protocol: TCP
        port: 8080  # Keycloak

    # 允许DNS查询
    - to:
      - namespaceSelector:
          matchLabels:
            name: kube-system
      ports:
      - protocol: UDP
        port: 53

    # 允许访问外部（可选，根据需求限制）
    - to:
      - namespaceSelector: {}
      ports:
      - protocol: TCP
        port: 443  # HTTPS
      - protocol: TCP
        port: 80   # HTTP

  6. 实现改动范围评估

  6.1 代码改动（中等）

  Server改动

  // cmd/server/main.go - 改动点

  // 1. 租户管理API（新增）
  // 新增约300行代码
  http.HandleFunc("/api/tenants", authMiddleware(handleTenants))
  http.HandleFunc("/api/tenants/{id}", authMiddleware(handleTenantDetail))
  http.HandleFunc("/api/tenants/{id}/quota", authMiddleware(handleTenantQuota))

  // 2. Workspace创建逻辑修改
  // 修改约50行代码
  func handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
      // 原有代码
      var workspace models.Workspace
      json.NewDecoder(r.Body).Decode(&workspace)

      // 新增：从JWT获取租户信息
      claims := r.Context().Value("claims").(*Claims)
      tenant, err := getTenantByUser(claims.PreferredUsername)
      if err != nil {
          http.Error(w, "Tenant not found", http.StatusNotFound)
          return
      }

      workspace.TenantID = tenant.ID
      workspace.Namespace = tenant.Namespace  // 设置namespace

      // 新增：检查租户配额
      if !checkTenantQuota(tenant.ID) {
          http.Error(w, "Tenant quota exceeded", http.StatusForbidden)
          return
      }

      // 原有创建逻辑...
  }

  // 3. Agent连接处理修改
  // 修改约30行代码
  func handleAgentConnection(w http.ResponseWriter, r *http.Request) {
      // 原有代码...

      // 新增：从Agent ID解析tenant和namespace
      // 格式：tenant-{tenantName}-user-{owner}-{name}
      parts := strings.Split(agentID, "-")
      if len(parts) < 5 {
          http.Error(w, "Invalid agent ID format", http.StatusBadRequest)
          return
      }

      tenantName := parts[1]
      owner := parts[3]

      // 验证租户和权限...
  }

  Server改动估算：
  - 新增文件：pkg/models/tenant.go, pkg/repositories/tenant.go, pkg/services/tenant.go
  - 修改文件：cmd/server/main.go（约200行）
  - 新增API：租户CRUD（约500行）
  - 总计：约800-1000行代码

  Operator改动

  // cmd/operator/controller.go - 改动点

  // 1. Reconcile逻辑修改
  // 修改约100行代码
  func (r *RDEAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
      // 原有代码...

      // 新增：从CRD获取tenant和namespace
      rdeAgent := &kuberdeiov1beta1.RDEAgent{}
      if err := r.Get(ctx, req.NamespacedName, rdeAgent); err != nil {
          return ctrl.Result{}, client.IgnoreNotFound(err)
      }

      // 新增：确定目标namespace
      targetNamespace := rdeAgent.Spec.Namespace
      if targetNamespace == "" {
          // 从annotations或tenant配置获取
          targetNamespace = getNamespaceForTenant(rdeAgent.Spec.Tenant)
      }

      // 修改：在目标namespace创建Deployment
      deployment := r.buildDeployment(rdeAgent, targetNamespace)

      // 原有创建/更新逻辑...
  }

  // 2. 权限配置
  // 修改RBAC以watch多个namespace

  Operator改动估算：
  - 修改文件：cmd/operator/controller.go（约150行）
  - 修改RBAC：支持多namespace watch
  - 总计：约200-300行代码

  CLI改动

  // cmd/cli/cmd/connect.go - 改动点

  // 1. 连接命令修改
  // 修改约50行代码
  func connectCmd() {
      // 原有代码...

      // 新增：支持tenant前缀
      // kuberde-cli connect workspace-name --tenant team-a
      // 或自动从配置推断

      agentID := buildAgentID(tenant, owner, workspace)
      // 格式：tenant-{tenant}-user-{owner}-{workspace}
  }

  CLI改动估算：
  - 修改文件：cmd/cli/cmd/*.go（约100行）
  - 总计：约100-150行代码

  Web UI改动

  // web/src/services/api.ts - 改动点

  // 1. 新增Tenant API
  export const fetchTenants = async (): Promise<Tenant[]> => {
    const response = await fetch(`${API_BASE_URL}/api/tenants`, {
      credentials: 'include',
    });
    return response.json();
  };

  export const createTenant = async (tenant: CreateTenantRequest): Promise<Tenant> => {
    const response = await fetch(`${API_BASE_URL}/api/tenants`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(tenant),
    });
    return response.json();
  };

  // 2. Workspace API修改
  export const createWorkspace = async (workspace: CreateWorkspaceRequest): Promise<Workspace> => {
    // 新增：包含tenantId
    const response = await fetch(`${API_BASE_URL}/api/workspaces`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        ...workspace,
        tenantId: getCurrentTenantId(),  // 新增
      }),
    });
    return response.json();
  };

  // web/src/pages/TenantManagement.tsx - 新增页面

  export const TenantManagement: React.FC = () => {
    const [tenants, setTenants] = useState<Tenant[]>([]);

    useEffect(() => {
      fetchTenants().then(setTenants);
    }, []);

    return (
      <div>
        <h1>Tenant Management</h1>
        <TenantList tenants={tenants} />
        <CreateTenantButton />
      </div>
    );
  };

  Web UI改动估算：
  - 新增页面：TenantManagement, TenantDetail, TenantQuota（约800行）
  - 修改页面：Dashboard, WorkspaceCreate（约200行）
  - 新增组件：TenantSelector, QuotaDisplay（约300行）
  - 总计：约1300-1500行代码

  6.2 配置改动（较大）

  Helm Chart改动

  # charts/kuberde/values.yaml - 新增部分

  # 多租户配置
  multiTenant:
    enabled: true
    mode: "shared-control-plane"  # shared-control-plane, dedicated, hybrid

    # 默认租户配置
    defaultTenant:
      name: "default"
      namespace: "kuberde-default"
      quota:
        cpu: "100"
        memory: "200Gi"
        storage: "1Ti"
        pods: 100
        services: 50
      limits:
        defaultCpuRequest: "100m"
        defaultMemoryRequest: "128Mi"
        maxCpuLimit: "8000m"
        maxMemoryLimit: "32Gi"

    # 预定义租户列表
    tenants:
      - name: "team-a"
        namespace: "kuberde-team-a"
        displayName: "Team A"
        quota:
          cpu: "50"
          memory: "100Gi"
          storage: "500Gi"
          pods: 50

      - name: "team-b"
        namespace: "kuberde-team-b"
        displayName: "Team B"
        quota:
          cpu: "30"
          memory: "60Gi"
          storage: "300Gi"
          pods: 30

  # Operator配置
  operator:
    # 新增：多namespace watch
    watchNamespaces: []  # 空则watch所有namespace
    # 或指定：["kuberde-team-a", "kuberde-team-b"]

    # 新增：Cluster角色权限
    clusterWide: true

  # Server配置
  server:
    # 新增：租户相关环境变量
    env:
      - name: MULTI_TENANT_ENABLED
        value: "true"
      - name: DEFAULT_TENANT_NAMESPACE
        value: "kuberde-default"

  # charts/kuberde/templates/tenant-namespace.yaml - 新增模板

  {{- if .Values.multiTenant.enabled }}
  {{- range .Values.multiTenant.tenants }}
  ---
  apiVersion: v1
  kind: Namespace
  metadata:
    name: {{ .namespace }}
    labels:
      app.kubernetes.io/name: kuberde
      app.kubernetes.io/component: tenant
      kuberde.io/tenant: {{ .name }}

  ---
  apiVersion: v1
  kind: ResourceQuota
  metadata:
    name: tenant-quota
    namespace: {{ .namespace }}
  spec:
    hard:
      requests.cpu: {{ .quota.cpu | quote }}
      requests.memory: {{ .quota.memory | quote }}
      requests.storage: {{ .quota.storage | quote }}
      pods: {{ .quota.pods | quote }}
      services: {{ .quota.services | quote }}

  ---
  apiVersion: v1
  kind: LimitRange
  metadata:
    name: tenant-limits
    namespace: {{ .namespace }}
  spec:
    limits:
    - type: Pod
      max:
        cpu: {{ .limits.maxCpuLimit | default "8000m" | quote }}
        memory: {{ .limits.maxMemoryLimit | default "32Gi" | quote }}
    - type: Container
      default:
        cpu: "1000m"
        memory: "1Gi"
      defaultRequest:
        cpu: {{ .limits.defaultCpuRequest | default "100m" | quote }}
        memory: {{ .limits.defaultMemoryRequest | default "128Mi" | quote }}

  ---
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    name: tenant-isolation
    namespace: {{ .namespace }}
  spec:
    podSelector: {}
    policyTypes:
    - Ingress
    - Egress
    ingress:
    - from:
      - podSelector: {}
    - from:
      - namespaceSelector:
          matchLabels:
            name: kuberde-system
    egress:
    - to:
      - podSelector: {}
    - to:
      - namespaceSelector:
          matchLabels:
            name: kuberde-system
    - to:
      - namespaceSelector:
          matchLabels:
            name: kube-system
      ports:
      - protocol: UDP
        port: 53

  {{- end }}
  {{- end }}

  Helm Chart改动估算：
  - 新增模板：tenant-namespace.yaml, tenant-rbac.yaml（约300行）
  - 修改values.yaml（约150行）
  - 修改其他模板以支持tenant变量（约100行）
  - 总计：约550-600行YAML

  RBAC改动

  # deploy/k8s/04-operator.yaml - 修改部分

  ---
  # 原来的Namespace级Role改为ClusterRole
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: kuberde-operator
  rules:
  # RDEAgent CRD权限（所有namespace）
  - apiGroups: ["kuberde.io"]
    resources: ["rdeagents"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["kuberde.io"]
    resources: ["rdeagents/status"]
    verbs: ["get", "update", "patch"]

  # Deployment权限（所有namespace）
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # PVC权限（所有namespace）
  - apiGroups: [""]
    resources: ["persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # Namespace权限
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "watch"]

  # ResourceQuota权限
  - apiGroups: [""]
    resources: ["resourcequotas"]
    verbs: ["get", "list", "watch"]

  ---
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRoleBinding
  metadata:
    name: kuberde-operator
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: kuberde-operator
  subjects:
  - kind: ServiceAccount
    name: kuberde-operator
    namespace: kuberde-system

  6.3 数据库迁移

  -- deploy/migrations/XXXXXX_add_multi_tenant_support.sql

  -- 创建tenants表
  CREATE TABLE tenants (
      id SERIAL PRIMARY KEY,
      name VARCHAR(255) UNIQUE NOT NULL,
      display_name VARCHAR(255) NOT NULL,
      namespace VARCHAR(255) UNIQUE NOT NULL,
      type VARCHAR(50) NOT NULL DEFAULT 'shared',

      -- 资源配额
      quota_enabled BOOLEAN DEFAULT true,
      cpu_quota VARCHAR(50),
      memory_quota VARCHAR(50),
      storage_quota VARCHAR(50),
      pods_quota INTEGER,
      services_quota INTEGER,

      -- 限制范围
      default_cpu_request VARCHAR(50),
      default_memory_request VARCHAR(50),
      max_cpu_limit VARCHAR(50),
      max_memory_limit VARCHAR(50),

      -- Keycloak集成
      realm_name VARCHAR(255),

      -- 状态
      status VARCHAR(50) DEFAULT 'active',
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );

  -- 为workspaces表添加tenant_id
  ALTER TABLE workspaces ADD COLUMN tenant_id INTEGER;
  ALTER TABLE workspaces ADD CONSTRAINT fk_workspace_tenant
      FOREIGN KEY (tenant_id) REFERENCES tenants(id);

  -- 修改唯一索引以包含tenant_id
  DROP INDEX IF EXISTS idx_owner_name;
  CREATE UNIQUE INDEX idx_tenant_owner_name ON workspaces(tenant_id, owner, name);

  -- 创建默认租户
  INSERT INTO tenants (name, display_name, namespace, cpu_quota, memory_quota, storage_quota, pods_quota, services_quota)
  VALUES ('default', 'Default Tenant', 'kuberde-default', '100', '200Gi', '1Ti', 100, 50);

  -- 将现有workspaces分配给默认租户
  UPDATE workspaces SET tenant_id = (SELECT id FROM tenants WHERE name = 'default')
  WHERE tenant_id IS NULL;

  -- 为users表添加tenant关联（可选）
  ALTER TABLE users ADD COLUMN default_tenant_id INTEGER;
  ALTER TABLE users ADD CONSTRAINT fk_user_default_tenant
      FOREIGN KEY (default_tenant_id) REFERENCES tenants(id);

  7. 改动范围总结

  代码改动统计

  | 组件     | 新增代码 | 修改代码 | 删除代码 | 总计   |
  |----------|----------|----------|----------|--------|
  | Server   | 800行    | 200行    | 50行     | 1050行 |
  | Operator | 200行    | 100行    | 30行     | 330行  |
  | CLI      | 100行    | 50行     | 20行     | 170行  |
  | Web UI   | 1300行   | 200行    | 50行     | 1550行 |
  | Models   | 300行    | 100行    | 0行      | 400行  |
  | 总计     | 2700行   | 650行    | 150行    | 3500行 |

  配置改动统计

  | 类型               | 新增  | 修改  | 总计  |
  |--------------------|-------|-------|-------|
  | Helm Templates     | 300行 | 100行 | 400行 |
  | Values YAML        | 150行 | 50行  | 200行 |
  | RBAC               | 100行 | 80行  | 180行 |
  | Database Migration | 80行  | 0行   | 80行  |
  | 总计               | 630行 | 230行 | 860行 |

  工作量估算

  | 任务              | 难度 | 工作量       | 风险 |
  |-------------------|------|--------------|------|
  | 1. 数据模型设计   | 中   | 3天          | 低   |
  | 2. Server API开发 | 中   | 5天          | 中   |
  | 3. Operator改造   | 高   | 4天          | 中   |
  | 4. CLI改造        | 低   | 2天          | 低   |
  | 5. Web UI开发     | 中   | 6天          | 低   |
  | 6. Helm Chart改造 | 中   | 3天          | 中   |
  | 7. 测试和文档     | 中   | 5天          | 低   |
  | 8. 数据迁移       | 高   | 2天          | 高   |
  | 总计              | -    | 30天 (约6周) | -    |

  人力配置建议：
  - 后端开发：1人（Server + Operator + Database）
  - 前端开发：1人（Web UI）
  - DevOps工程师：0.5人（Helm + 部署）
  - 测试工程师：0.5人（测试 + 文档）

  8. 实施计划建议

  Phase 1: 核心架构（2周）

  1. Week 1: 数据模型 + Database Migration
    - 设计Tenant模型
    - 创建migration脚本
    - 修改Workspace模型
  2. Week 2: Server核心功能
    - Tenant CRUD API
    - Workspace创建逻辑修改
    - 配额检查逻辑

  Phase 2: Operator和部署（2周）

  3. Week 3: Operator改造
    - 多namespace watch
    - Tenant感知的资源创建
    - RBAC修改
  4. Week 4: Helm Chart改造
    - Multi-tenant values配置
    - Namespace自动创建
    - ResourceQuota模板

  Phase 3: 用户界面（1周）

  5. Week 5: CLI和Web UI
    - CLI tenant支持
    - Web UI租户管理页面
    - Dashboard租户切换

  Phase 4: 测试和文档（1周）

  6. Week 6: 集成测试和文档
    - 端到端测试
    - 升级测试
    - 用户文档更新

  9. 风险和缓解措施

  高风险项

  | 风险                          | 影响 | 概率 | 缓解措施                                                  |
  |-------------------------------|------|------|-----------------------------------------------------------|
  | 数据迁移失败                  | 高   | 中   | 1. 充分测试迁移脚本2. 提供回滚方案3. 生产环境备份         |
  | Operator多namespace watch性能 | 中   | 低   | 1. 使用namespace过滤2. 性能测试3. 添加cache层             |
  | 现有用户升级兼容性            | 高   | 中   | 1. 保持向后兼容2. 自动迁移到default tenant3. 详细升级文档 |
  | 配额计算复杂度                | 中   | 中   | 1. 使用K8s原生ResourceQuota2. 定期同步和校验3. 告警机制   |

  缓解策略

  1. 向后兼容性：
    - 不指定tenant时自动使用default tenant
    - 保持单namespace部署模式作为选项
    - 提供migration工具
  2. 渐进式rollout：
    - 先在dev/staging环境测试
    - 部分用户beta测试
    - 逐步开放给所有用户
  3. 监控和可观测性：
    - 添加tenant相关的metrics
    - 配额使用监控
    - 异常告警

  10. 最终推荐

  ⭐ 推荐方案：A2 共享控制平面模式

  理由：
  1. ✅ 成本效益最高：单套控制平面，资源利用率高
  2. ✅ 实现复杂度适中：约3500行代码，6周开发周期
  3. ✅ 运维负担可控：统一管理，不需要多集群
  4. ✅ 满足大多数场景：namespace级隔离足够安全
  5. ✅ 扩展性好：未来可升级到dedicated模式

  适用场景：
  - 中小型团队（<50个租户）
  - SaaS模式部署
  - 预算和运维资源有限
  - 需要快速上线

  实施建议

  第一阶段（MVP）：
  - 实现基础多租户支持
  - Namespace隔离
  - ResourceQuota管理
  - 基础Web UI

  第二阶段（增强）：
  - 租户自助管理
  - 详细配额监控
  - 计费功能
  - NetworkPolicy优化

  第三阶段（高级）：
  - 支持hybrid模式（大租户dedicated）
  - vCluster集成
  - 跨集群租户
  - 高级安全特性