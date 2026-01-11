# KubeRDE - 已实现功能清单

本项目是一个完整的 Kubernetes 原生远程开发环境平台，提供安全的多租户工作区管理、资源配额、审计日志和 Web 管理界面。

## 1. 核心组件

### 1.1 KubeRDE Server (`cmd/server` - ~6500 行代码)

**基础功能**:
- WebSocket 服务器，监听 `:8080/ws` 端口接收 Agent 连接
- 用户代理服务器，监听 `:2022` 端口提供 SSH/HTTP 隧道
- Yamux 多路复用协议管理与 Agent 的连接

**数据库集成**:
- PostgreSQL 数据持久化 (GORM ORM)
- 自动数据库迁移
- 数据模型: Users, Workspaces, Services, AgentTemplates, AuditLogs, ResourceConfigs, UserQuotas, SSHKeys

**REST API (50+ 端点)**:
- `/api/users/*` - 用户管理 (CRUD, SSH 密钥, 配额)
- `/api/workspaces/*` - 工作区管理
- `/api/services/*` - 服务管理 (创建、编辑、删除)
- `/api/agent-templates/*` - 代理模板管理 (导入/导出)
- `/api/audit-logs/*` - 审计日志查询
- `/api/resource-config/*` - 系统资源默认配置 (CPU, 内存, GPU, 存储类)
- `/api/user-quotas/*` - 用户配额管理
- `/mgmt/agents/{id}` - Agent 状态和 TTL 追踪
- `/auth/*` - OIDC 认证流程

**认证与授权**:
- OIDC/JWT 令牌验证 (通过 Keycloak)
- JWKS 端点缓存
- 基于角色的访问控制 (Admin/Developer)
- 多租户所有权验证 (Agent ID: `user-{owner}-{name}`)

**用户管理**:
- 用户 CRUD 操作
- 与 Keycloak 同步 (通过 Admin API)
- SSH 公钥管理和指纹生成
- 用户配额分配

**审计日志**:
- 记录所有 CRUD 操作
- 包含用户、操作、资源类型、资源 ID、变更前后数据
- 支持搜索和过滤

### 1.2 KubeRDE Agent (`cmd/agent`)

**功能**:
- 建立与 Server 的持久 WebSocket 连接
- OAuth2 Client Credentials 流程获取令牌 (自动刷新)
- Yamux 客户端接受来自 Server 的流
- 将流量桥接到本地服务 (SSH, Jupyter, Coder, File browser)

**配置**:
- 环境变量配置: SERVER_URL, LOCAL_TARGET, AGENT_ID, AUTH_CLIENT_ID, AUTH_CLIENT_SECRET, AUTH_TOKEN_URL

### 1.3 KubeRDE CLI (`cmd/cli`)

**命令**:
- `login` - 通过浏览器进行 OIDC 认证，保存令牌到 `~/.frp/token.json`
- `connect` - 读取令牌，建立 WebSocket 连接，作为 SSH ProxyCommand

### 1.4 KubeRDE Operator (`cmd/operator` - ~2500 行代码)

**CRD 管理**:
- 监听 `RDEAgent` CRD (v1beta1, group: `kuberde.io`)
- 自动创建/更新/删除 Kubernetes Deployments

**工作区存储**:
- PVC (Persistent Volume Claim) 自动创建和绑定
- 支持多种存储类选择
- 卷挂载到工作负载容器

**TTL 自动缩容**:
- 轮询 Server 的 `/mgmt/agents/{id}` API
- 检测 Agent 空闲时间
- 超过 TTL 后自动缩容 Deployment 到 0 副本
- 用户连接时自动扩容

**状态管理**:
- Agent 在线/离线/错误状态追踪
- 状态更新到 CRD Status 字段
- 关联 ID 追踪并发更新

**安全上下文**:
- UID/GID 配置
- 卷挂载权限
- SecurityContext 注入

**资源管理**:
- CPU/内存 requests 和 limits
- GPU 分配和节点选择器
- 资源配额验证

### 1.5 Web UI (`web/` - React + TypeScript)

**技术栈**:
- React 18+
- TypeScript
- Vite 构建工具
- Tailwind CSS 样式
- React Router 路由

**页面 (15 个)**:
1. **Dashboard** - 资源分配可视化、最近工作区
2. **Workspaces** - 工作区列表和管理
3. **WorkspaceCreate** - 创建工作区 (存储配置)
4. **WorkspaceDetail** - 工作区详情和服务列表
5. **ServiceCreate** - 创建服务 (CPU/内存/GPU/TTL 配置)
6. **ServiceDetail** - 服务详情和日志
7. **ServiceEdit** - 编辑服务资源 (CPU/内存滑块, GPU 开关, TTL 预设)
8. **UserManagement** - 用户管理 (管理员功能, 批量删除, 过滤)
9. **UserEdit** - 编辑用户资料、SSH 密钥、配额
10. **AdminWorkspaces** - 管理员查看所有工作区和资源指标
11. **AuditLogs** - 审计日志查看器 (搜索和过滤)
12. **AgentTemplates** - 代理模板管理 (导入/导出)
13. **ResourceManagement** - 系统资源配置 (默认值, GPU 类型)
14. **Help** - 用户文档和帮助
15. **Login** - OIDC 登录界面

**组件**:
- Header - 顶部导航、用户菜单、登出
- Sidebar - 主导航 (基于角色显示菜单)
- ProtectedRoute - 路由保护和角色检查

**API 客户端** (`web/services/api.ts`):
- 完整的 CRUD 操作
- 错误处理
- 认证令牌管理

### 1.6 PostgreSQL 数据库

**数据模型**:
- **User**: ID, username, email, fullName, role (admin/developer), SSHKeys[]
- **Workspace**: ID, name, owner_id, storage_size, storage_class, pvc_name
- **Service**: ID, agent_id, workspace_id, name, local_target, status, resources (CPU, 内存, GPU), ttl, is_pinned
- **AgentTemplate**: name, agent_type (ssh/file/coder/jupyter), docker_image, env_vars, security_context, volume_mounts
- **AuditLog**: user_id, action, resource_type, resource_id, old_data, new_data, timestamp
- **ResourceConfig**: default_cpu_cores, default_memory_gi, storage_classes[], gpu_types[]
- **UserQuota**: user_id, cpu_cores, memory_gi, storage_quota[], gpu_quota[]
- **SSHKey**: id, user_id, name, public_key, fingerprint, added_at

**特性**:
- GORM 自动迁移
- 外键关联
- 索引优化
- 关系完整性

## 2. 核心功能

### 2.1 多租户工作区管理

- 每个用户拥有独立的工作区
- 工作区绑定 PVC 提供持久化存储
- 存储类可配置 (standard, ssd, fast)
- 工作区可包含多个服务

### 2.2 服务类型支持

**SSH** - 终端访问开发环境:
- 基于 OpenSSH Server
- SSH 公钥认证
- 用户自定义 UID/GID

**File** - Web 文件浏览器:
- 文件上传/下载
- 在线编辑
- 目录管理

**Coder** - VS Code Server:
- 完整 VS Code 体验
- 扩展支持
- 终端集成

**Jupyter** - JupyterLab:
- 数据科学工作流
- Notebook 管理
- Python/R 支持

**Custom** - 自定义模板:
- 通过 AgentTemplate 扩展
- 支持任意 Docker 镜像
- 灵活的环境变量和挂载配置

### 2.3 资源管理

**服务级资源分配**:
- CPU 核心数 (可通过滑块调整)
- 内存 GiB (可通过滑块调整)
- GPU 数量和型号选择
- GPU 节点选择器

**系统默认配置**:
- 默认 CPU 核心数
- 默认内存大小
- 可用存储类列表
- 可用 GPU 类型列表

**用户配额**:
- CPU 核心总配额
- 内存总配额
- 存储配额 (按存储类)
- GPU 配额 (按 GPU 类型)
- 配额验证和执行

### 2.4 生命周期管理

**TTL 自动缩容**:
- 服务空闲超时配置 (1h, 8h, 24h, 7d)
- 自动检测空闲状态
- 自动缩容到 0 副本节省资源
- Operator 定期轮询执行

**按需扩容**:
- 用户连接时自动扩容
- 从 0 副本恢复到 1 副本
- 快速启动工作负载

**固定服务**:
- is_pinned 标志防止自动缩容
- 适用于始终在线的服务

### 2.5 安全和审计

**认证**:
- OIDC/OAuth2 via Keycloak
- JWT 令牌验证
- JWKS 端点缓存

**授权**:
- 基于角色的访问控制 (RBAC)
- Admin 角色: 用户管理、所有工作区、配额、模板、资源配置
- Developer 角色: 自己的工作区和服务

**SSH 密钥管理**:
- 每个用户管理多个 SSH 密钥
- 自动生成指纹 (SHA256)
- 密钥注入到工作负载

**审计日志**:
- 记录所有 CRUD 操作
- 包含完整的变更历史
- 支持搜索和过滤
- 符合合规要求

### 2.6 模板管理

**AgentTemplate**:
- 预定义服务类型 (SSH, File, Coder, Jupyter)
- Docker 镜像配置
- 默认环境变量
- SecurityContext 配置
- VolumeMounts 配置

**导入/导出**:
- JSON 格式批量导入模板
- 导出现有模板
- 模板共享和复用

## 3. Kubernetes 集成

### 3.1 自定义资源定义 (CRD)

**RDEAgent (v1beta1)**:
```yaml
spec:
  serverUrl: ws://kuberde-server:8080/ws
  owner: username
  authSecret: agent-credentials
  localTarget: localhost:22
  workloadContainer:
    image: docker.io/user/image:tag
    imagePullPolicy: IfNotPresent
    command: ["/bin/sh"]
    args: ["-c", "start.sh"]
    ports:
      - containerPort: 22
    env:
      - name: KEY
        value: VALUE
    resources:
      requests:
        cpu: "1"
        memory: "2Gi"
      limits:
        cpu: "2"
        memory: "4Gi"
        nvidia.com/gpu: "1"
    securityContext:
      runAsUser: 1000
      runAsGroup: 1000
    volumeMounts:
      - name: workspace
        mountPath: /workspace
  ttl: "8h"
```

### 3.2 自动化部署

- Operator 自动创建 Deployment
- 包含 frp-agent (sidecar) 和 workload 容器
- 自动注入认证凭据 (从 Secret)
- PVC 自动创建和挂载
- OwnerReference 级联删除

### 3.3 存储管理

- PVC 自动创建
- 存储类可选 (standard/ssd/fast)
- 存储大小可配置
- 挂载到 /workspace

## 4. 开发体验

### 4.1 CLI 工具

- 简单的登录流程 (浏览器 OIDC)
- SSH ProxyCommand 无缝集成
- 令牌自动保存和重用

### 4.2 Web UI

- 直观的管理界面
- 资源配置可视化 (滑块控件)
- 实时状态更新
- 内置帮助文档
- 响应式设计

### 4.3 模板系统

- 预定义服务类型
- 一键创建服务
- 模板参数化配置

## 5. 运维功能

### 5.1 监控

- Agent 在线状态追踪
- 最后活跃时间记录
- 连接统计
- 资源使用可视化

### 5.2 日志

- 服务器端综合日志
- Operator 操作日志
- Agent 连接日志
- 审计日志持久化

### 5.3 成本优化

- TTL 自动缩容节省资源
- 按需扩容减少浪费
- 资源配额防止过度使用
- 存储类选择优化成本

## 6. 部署配置

### 6.1 Kubernetes 清单

- `00-namespace.yaml` - kuberde 命名空间
- `01-crd.yaml` - RDEAgent CRD 定义
- `02-keycloak*.yaml` - Keycloak 部署
- `02-web.yaml` - Web UI 部署
- `03-server.yaml` - KubeRDE Server 部署
- `03-agent.yaml` - 示例 Agent
- `04-operator.yaml` - Operator 部署
- `05-ingress.yaml` - Ingress 配置
- `06-postgresql.yaml` - PostgreSQL 数据库
- `all-in-one.yaml` - 一键部署清单

### 6.2 配置管理

- 环境变量配置
- Kubernetes Secrets
- ConfigMaps
- Ingress 规则

## 7. 近期完成的功能

- ✅ Web UI 完整 CRUD 操作
- ✅ 帮助页面集成
- ✅ 审计日志 UI (搜索和过滤)
- ✅ 服务编辑 (CPU/内存滑块, GPU 配置)
- ✅ Agent 模板导入/导出
- ✅ PostgreSQL 数据库集成
- ✅ 用户配额系统
- ✅ SSH 密钥管理
- ✅ GPU 支持和节点选择器
- ✅ CRD v1beta1 (资源限制, 卷, 安全上下文)

## 8. 技术栈总结

**后端**:
- Go 1.24+
- Gorilla WebSocket
- Hashicorp Yamux
- GORM (PostgreSQL)
- go-oidc
- Kubernetes client-go
- Gocloak

**前端**:
- React 18+
- TypeScript
- Vite
- Tailwind CSS
- React Router

**基础设施**:
- Kubernetes 1.28+
- PostgreSQL 12+
- Keycloak (OIDC)
- Docker

---

**项目状态**: 生产就绪 v1.0
**许可证**: MIT
