# KubeRDE 架构设计决策记录 (ADR)

本文档记录了 KubeRDE 项目中的重要架构决策、其背景、考虑的替代方案以及权衡。

---

## ADR-001: 使用 Deployment 而非 Pod 管理 Agent

**状态**: ✅ 已接受
**日期**: 2025-12-06
**涉及组件**: Operator, Agent 生命周期管理

### 背景

KubeRDE 需要在 Kubernetes 中运行 FRP Agent，每个 Agent 需要与工作负载容器共存（Sidecar 模式）。需要决定使用什么 Kubernetes 工作负载资源：Pod 还是 Deployment？

### 决策

**使用 Deployment 而非 Pod**

### 论证

| 方面 | Deployment | Pod |
|------|-----------|-----|
| **自动故障恢复** | ✅ Pod 崩溃自动重建 | ❌ 需手动干预 |
| **副本管理** | ✅ ReplicaSet 支持 scale | ❌ 无副本概念 |
| **版本管理** | ✅ 历史版本、回滚 | ❌ 无版本追踪 |
| **滚动更新** | ✅ Zero-downtime 更新 | ❌ 必须删除后重建 |
| **TTL 缩容** | ✅ 通过 replicas 改为 0 | ❌ 需删除 Pod |
| **运维友好** | ✅ kubectl 原生支持 | ⚠️ 受限的工具支持 |
| **资源开销** | ⚠️ 多一层 ReplicaSet | ✅ 最小化 |

### 权衡

- **优势**: 获得 Kubernetes 生态的完整支持、自动故障恢复、灵活的扩缩容
- **劣势**: 多一层控制逻辑（ReplicaSet），稍微增加资源开销
- **结论**: 优势远超劣势，特别是对于长期运行的 Agent

### 关联决策

- ADR-003: TTL-based 自动缩容（基于 Deployment replicas）
- ADR-006: Operator CRD 设计（围绕 Deployment 展开）

---

## ADR-002: Operator 模式管理 RDEAgent CRD

**状态**: ✅ 已接受
**日期**: 2025-12-06
**涉及组件**: Operator, CRD

### 背景

用户需要用声明式 YAML 定义和管理 FRP Agent，而不是手动创建 Deployment。需要决定如何实现这一需求。

### 考虑的方案

1. **Operator 模式（选中）**: 编写自定义 Controller，监听 RDEAgent CR，自动创建/更新/删除 Deployment
2. **Helm Chart**: 提供 Helm 模板，用户自己值化参数
3. **Kustomize**: 用 Kustomize base/overlay 管理不同环境
4. **Manual YAML**: 用户自己写 Deployment（不现实）

### 决策

**使用 Operator 模式**

### 论证

| 方案 | Operator | Helm | Kustomize |
|------|----------|------|-----------|
| **声明式管理** | ✅ 完全 | ⚠️ 部分 | ⚠️ 部分 |
| **自动化程度** | ✅ 最高 | ⚠️ 中等 | ❌ 低 |
| **多租户隔离** | ✅ 原生支持 | ⚠️ 需手工 | ❌ 无支持 |
| **动态扩缩容** | ✅ 支持 | ❌ 不支持 | ❌ 不支持 |
| **学习曲线** | ⚠️ 陡峭 | ✅ 平缓 | ✅ 平缓 |
| **实现复杂度** | ⚠️ 高 | ✅ 低 | ✅ 低 |

### 权衡

- **优势**:
  - 完整的声明式管理
  - 支持动态扩缩容、TTL、自动化任务
  - 与 Kubernetes API 完全一致

- **劣势**:
  - Operator 代码需维护
  - 学习成本高

- **结论**: 对于一个完整的系统，Operator 是必需的

### 实现细节

- 使用 `client-go` 的 `dynamic` client（支持 unstructured 对象）
- 使用 `informer` 实现 watch 机制
- 支持 OwnerReferences 自动级联删除

---

## ADR-003: TTL-based 自动缩容机制

**状态**: ✅ 已接受
**日期**: 2025-12-06
**涉及组件**: Operator, Server

### 背景

用户的 Agent 经常空闲（没有活动），运行 Pod 白白消耗资源。需要实现自动缩容，但要避免影响用户体验。

### 考虑的方案

1. **TTL + 定期检查（选中）**: 记录最后活动时间，超过 TTL 就缩容至 0
2. **Idle Timeout + 主动清理**: Server 主动关闭空闲连接
3. **用户手动触发**: 用户自己决定何时缩容
4. **Horizontal Pod Autoscaler (HPA)**: 基于 CPU/内存指标缩容

### 决策

**TTL-based 自动缩容**

### 论证

| 方案 | TTL+检查 | Idle Timeout | 手动触发 | HPA |
|------|---------|------------|---------|-----|
| **成本优化** | ✅ 最好 | ✅ 好 | ❌ 差 | ⚠️ 中等 |
| **用户体验** | ✅ 透明 | ⚠️ 可能断连 | ❌ 需手工 | ❌ 不适用 |
| **实现复杂度** | ✅ 简单 | ⚠️ 中等 | ✅ 简单 | ⚠️ 需配置 |
| **精细控制** | ✅ 高 | ⚠️ 中等 | ✅ 最高 | ❌ 低 |

### 权衡

- **优势**:
  - 简单可靠
  - 用户可配置 TTL，完全自主
  - 缩容至 0，成本最低

- **劣势**:
  - 需要 Agent 暴露 `/mgmt/agents/{id}` 端点
  - 如果 Server 故障，无法准确判断活动时间

- **结论**: 对于非关键系统，TTL 足够好

### 现状

- ✅ 已实现基本的 TTL 缩容（Operator 定期检查）
- ❌ 缺少自动扩容（缩容后用户需等待，或手动扩容）

### 后续改进

- ADR-007: 自动扩容机制（当用户尝试连接时）

---

## ADR-004: OIDC 多层认证架构

**状态**: ✅ 已接受（部分改进中）
**日期**: 2025-12-06
**涉及组件**: Server, Agent, CLI

### 背景

KubeRDE 支持三种客户端，每种有不同的认证需求：
1. **CLI 用户**：通过浏览器进行 OIDC 登录（Authorization Code Flow）
2. **HTTP 用户**：通过 Cookie 保持会话
3. **Agent**：通过 Service Account 进行 OAuth2 Client Credentials 认证

### 决策

**使用多层 OIDC 认证**：
- 用户: OIDC Authorization Code + Cookie
- Agent: OAuth2 Client Credentials
- Token 验证: JWKS 验证

### 论证

| 认证方式 | CLI 用户 | Agent | 说明 |
|---------|---------|-------|------|
| **OIDC 授权码** | ✅ 用 | ❌ 用 | 需交互，不适合 Agent |
| **Client Credentials** | ❌ 用 | ✅ 用 | 自动，适合机器客户端 |
| **Cookie 会话** | ✅ 用 | ❌ 用 | 浏览器原生，Agent 无需 |
| **JWKS 验证** | ✅ 用 | ✅ 用 | 离线验证，无需实时查询 |

### 权衡

- **优势**:
  - 每种客户端用最适合的流程
  - 安全性高（Token 短期、支持刷新）

- **劣势**:
  - 复杂度高（三种认证方式并存）
  - 需要维护 OIDC 配置

- **结论**: 必需的复杂度，为了支持三种客户端类型

### 现状

- ✅ 已实现 OIDC 授权码流
- ✅ 已实现 Client Credentials
- ⚠️ Cookie 会话有安全问题（ADR-008 改进）
- ❌ Agent Token 无自动刷新（计划改进）

### 关联决策

- ADR-008: Cookie 安全加固
- ADR-009: Token 自动刷新

---

## ADR-005: Yamux 多路复用传输

**状态**: ✅ 已接受
**日期**: 2025-12-06
**涉及组件**: Server, Agent

### 背景

FRP Agent 需要与 Server 保持单一长连接，但需要支持多个用户同时连接。需要在这个长连接上进行多路复用。

### 考虑的方案

1. **Yamux（选中）**: 多路复用库，Apache 2.0 许可证
2. **gRPC**: 完整 RPC 框架，但过重
3. **QUIC**: UDP 多路复用，但对 WebSocket 支持差
4. **Manual stream management**: 自己实现多路复用（不现实）

### 决策

**使用 Yamux 进行多路复用**

### 论证

| 方案 | Yamux | gRPC | QUIC |
|------|-------|------|------|
| **轻量级** | ✅ 是 | ❌ 否 | ✅ 是 |
| **WebSocket 兼容** | ✅ 是 | ⚠️ 需配置 | ❌ 否 |
| **多路复用** | ✅ 支持 | ✅ 支持 | ✅ 支持 |
| **生产成熟度** | ✅ 成熟 | ✅ 成熟 | ⚠️ 新兴 |
| **学习曲线** | ✅ 平缓 | ⚠️ 陡峭 | ⚠️ 陡峭 |

### 权衡

- **优势**:
  - 轻量级，易集成
  - 与 WebSocket 无缝配合
  - 低开销

- **劣势**:
  - 不如 gRPC 功能完整
  - 缺少自动序列化/反序列化

- **结论**: 对于 FRP 这种简单的转发场景，Yamux 足够

### 实现细节

- 在 WebSocket 上包装 `wsConn` adapter
- 支持 `io.Reader/Writer` 接口
- Agent 端 `session.Accept()` 接收流，Server 端 `session.Open()` 创建流

---

## ADR-006: CRD 设计的演进策略

**状态**: ✅ 已接受（计划扩展到 v1beta1）
**日期**: 2025-12-06
**涉及组件**: Operator

### 背景

RDEAgent CRD 当前功能较简单，但需要支持更复杂的场景（GPU、存储、调度）。需要决定如何演进 CRD 而不破坏现有的 CR。

### 考虑的方案

1. **版本控制（选中）**: v1 → v1beta1 → v2，支持转换和多版本存储
2. **直接修改**: 在 v1 上继续添加字段，背负包袱
3. **新 CRD**: 定义 RDEAgentAdvanced，并存
4. **完全重写**: 废弃 v1，强制用户迁移

### 决策

**实现 CRD 版本控制和转换**

### 论证

| 方案 | 版本控制 | 直接修改 | 新 CRD | 重写 |
|------|--------|--------|--------|------|
| **向后兼容** | ✅ 完全 | ✅ 完全 | ⚠️ 需配置 | ❌ 否 |
| **设计清晰** | ✅ 是 | ❌ 否 | ⚠️ 分散 | ✅ 是 |
| **迁移成本** | ⚠️ 中等 | ❌ 无（继续用 v1） | ⚠️ 需工具 | ❌ 高 |
| **长期可维护** | ✅ 是 | ❌ 否 | ❌ 否 | ✅ 是 |

### 权衡

- **优势**:
  - 清晰的功能迭代
  - 长期可维护性好
  - 支持多个版本并存

- **劣势**:
  - 实现转换器复杂
  - 初期投入较大

- **结论**: 虽然初期投入大，但长期收益大

### 时间表

- **v1** (现状): 基础 Agent 和工作负载配置
- **v1beta1** (计划): 添加 resources/volumes/nodeSelector/tolerations
- **v2** (未来): 完整 Pod 配置，支持 Sidecar 注入等高级特性

### 实现方案

- Kubernetes CRD 支持原生多版本（`apiVersion: kuberde.io/v1beta1`）
- 编写 Conversion Webhook（v1 → v1beta1）
- Operator 支持两个版本的 CR

---

## ADR-007: 自动扩容触发机制

**状态**: 🔄 计划中
**日期**: 2025-12-06
**涉及组件**: Server, Operator

### 背景

当前 Operator 仅支持 TTL 缩容，但缩容后，用户需要等待 Pod 重新启动才能使用 Agent。为了改善用户体验，需要实现自动扩容。

### 考虑的方案

1. **Server Webhook（选中）**: Server 在发现 Agent 离线时，调用 Operator Webhook 触发扩容
2. **Kubernetes Event**: 监听 Deployment 事件，自动扩容
3. **外部触发器**: 提供 API，由用户或外部系统触发
4. **预热池**: 始终保持几个 Pod 就绪（成本太高）

### 决策

**Server 主动调用 Operator Webhook 触发扩容**

### 论证

| 方案 | Webhook | K8s Event | 外部触发 | 预热池 |
|------|---------|----------|---------|--------|
| **自动化程度** | ✅ 最高 | ⚠️ 中等 | ❌ 低 | ✅ 完全 |
| **延迟** | ⚠️ 秒级 | ⚠️ 秒级 | ❌ 高 | ✅ 无 |
| **成本** | ✅ 低 | ✅ 低 | ✅ 低 | ❌ 高 |
| **复杂度** | ⚠️ 中等 | ✅ 低 | ✅ 低 | ⚠️ 中等 |

### 权衡

- **优势**:
  - 完全自动化，用户无感知
  - Server 已有 Agent 状态信息，易于判断

- **劣势**:
  - 需要 Server ↔ Operator 通信
  - 需要 Webhook 认证机制

- **结论**: 虽然复杂性增加，但 UX 提升明显

### 实现计划

1. Operator 暴露 `/api/webhooks/scale-up` 端点
2. Server 在 Agent 请求时，检查是否离线
3. 如果离线且 Deployment replicas=0，调用 Webhook
4. Operator 更新 Deployment replicas 为 1
5. Pod 启动后，Agent 自动重连 Server

### 时间线

- Phase 1: Server Webhook 端点实现
- Phase 2: Operator Webhook 处理
- Phase 3: 集成测试和性能验证

---

## ADR-008: Cookie 安全加固

**状态**: 🔄 计划中（安全改进）
**日期**: 2025-12-06
**涉及组件**: Server

### 背景

当前 Cookie 直接存储 JWT Token，存在以下安全问题：
- 非 HTTPS 环境可被中间人窃取
- CSRF 攻击风险
- Cookie 过期检查不完整

### 考虑的方案

1. **Session ID + Server Store（选中）**: Cookie 仅包含 Session ID，Token 存在服务器
2. **加密 Cookie**: JWT 经过加密后存在 Cookie
3. **Secure Flag Only**: 仅添加 Secure 和 SameSite
4. **完全无状态**: 保持当前设计，依赖 JWT 过期时间

### 决策

**实现 Session ID + Server Store 模式**

### 论证

| 方案 | Session+Store | 加密 Cookie | Secure 标记 | 无状态 |
|------|--------------|-----------|-----------|--------|
| **安全性** | ✅ 最高 | ✅ 高 | ⚠️ 中等 | ❌ 低 |
| **隐私** | ✅ 最好 | ✅ 好 | ⚠️ 信息暴露 | ❌ Token 在 Cookie |
| **实现复杂度** | ⚠️ 高 | ✅ 低 | ✅ 极低 | ✅ 极低 |
| **扩展性** | ❌ 有状态 | ✅ 无状态 | ✅ 无状态 | ✅ 无状态 |

### 权衡

- **优势**:
  - 最高的安全标准
  - Token 不在网络上传输
  - 支持实时失效（删除 Session Store 中的条目）

- **劣势**:
  - 增加服务器内存占用
  - 多实例部署需要 Session 共享（Redis）

- **结论**: 安全性是首要考虑，成本可接受

### 实现细节

- Session Store: `map[sessionID]*Session`
- Session TTL: 可配置，默认 24 小时
- Session 自动过期清理：后台定时任务
- 多实例部署：使用 Redis/Memcached 共享 Session

### 时间线

- Phase 1: Session Store 实现
- Phase 2: Cookie 更新（Secure, SameSite）
- Phase 3: 多实例支持（Redis）

---

## ADR-009: Agent Token 自动刷新

**状态**: 🔄 计划中
**日期**: 2025-12-06
**涉及组件**: Agent

### 背景

当前 Agent 在启动时获取一次 OAuth2 Token，然后永久使用。当 Token 过期（通常 5 分钟）后，Agent 无法通信，需要重启。

### 考虑的方案

1. **定期刷新（选中）**: 使用 Refresh Token，定期更新 Access Token
2. **按需刷新**: 在 Token 过期时触发刷新
3. **增加 Token 过期时间**: 配置 Keycloak 发更长期 Token（最多 24h）
4. **无状态（不刷新）**: 每次请求都重新认证（成本高）

### 决策

**实现 TokenSource + 定期刷新**

### 论证

| 方案 | 定期刷新 | 按需刷新 | 长期 Token | 无状态 |
|------|--------|--------|----------|--------|
| **连接稳定性** | ✅ 最好 | ✅ 好 | ⚠️ 有窗口期 | ✅ 稳定 |
| **安全性** | ✅ 高 | ✅ 高 | ❌ 长期凭证 | ✅ 高 |
| **实现复杂度** | ✅ 低 | ⚠️ 中等 | ✅ 极低 | ❌ 高 |
| **刷新开销** | ✅ 低 | ⚠️ 中等 | ✅ 无 | ❌ 高 |

### 权衡

- **优势**:
  - 完全自动，无需手工干预
  - Token 短期有效，安全性高
  - 实现简单

- **劣势**:
  - 依赖 Refresh Token 不过期
  - 需要定期与 Keycloak 通信

- **结论**: 标准做法，必需实现

### 实现细节

- 使用 `oauth2.TokenSource` 接口
- Client Credentials Config 内置 `TokenSource()`
- 定期刷新间隔：Token TTL 的 80%（如 Token 有效 5 分钟，每 4 分钟刷新一次）
- 刷新失败重试：指数退避，最多重试 3 次

### 时间线

- Phase 1: TokenSource 实现和定时器
- Phase 2: 错误处理和重试机制
- Phase 3: 日志和监控

---

## ADR-010: SSH 凭证管理策略

**状态**: 🔄 计划中（安全改进）
**日期**: 2025-12-06
**涉及组件**: Operator, Workload Container

### 背景

当前 Operator 将 SSH 密码直接写入 Deployment 的 Env 变量，导致密码在多个地方暴露（YAML、Pod Spec、容器日志）。

### 考虑的方案

1. **Secret 存储（选中）**: 密码存在 K8s Secret，通过 `valueFrom.secretKeyRef` 引用
2. **ConfigMap**: 存公开信息（用户名等），密码分开
3. **Sealed Secrets**: 加密 Secret，存在版本控制
4. **External Secrets**: 从外部系统（Vault、AWS Secrets Manager）动态获取
5. **生成后注入**: Pod 启动时动态生成密码

### 决策

**使用 K8s Secret + valueFrom 引用**

### 论证

| 方案 | Secret | ConfigMap | Sealed | External | 动态生成 |
|------|--------|-----------|--------|----------|---------|
| **即开即用** | ✅ 是 | ✅ 是 | ⚠️ 需配置 | ❌ 否 | ✅ 是 |
| **安全性** | ✅ 高 | ❌ 否 | ✅ 最高 | ✅ 最高 | ✅ 高 |
| **运维简化** | ✅ 是 | ✅ 是 | ❌ 否 | ⚠️ 复杂 | ✅ 是 |
| **版本控制** | ❌ 不建议 | ✅ 可以 | ✅ 可以 | ❌ 否 | ✅ 可以 |

### 权衡

- **优势**:
  - K8s 原生，无需额外工具
  - 密码对容器进程可见，但不暴露在 Pod Spec
  - Operator 负责创建和轮换

- **劣势**:
  - Secret 仍在 etcd 中未加密（需要 at-rest encryption）
  - 不支持版本控制敏感信息

- **结论**: Phase 1 用 Secret + RBAC，Phase 2 可升级到 Sealed Secrets 或 External Secrets

### 实现细节

- Operator 创建 Secret: `frp-agent-{agentID}-credentials`
- Secret 内容：SSH 密码、公钥等
- Deployment 引用：`valueFrom.secretKeyRef`
- 密码生成：随机生成或从用户指定的 Secret 读取

### 时间线

- Phase 1: 基础 Secret 支持
- Phase 2: Secret 轮换机制
- Phase 3: Sealed Secrets/External Secrets 集成

---

## ADR-011: Management API 权限模型

**状态**: 🔄 计划中
**日期**: 2025-12-06
**涉及组件**: Server, Operator

### 背景

当前 Management API (`/mgmt/agents/{id}`) 无权限控制，任何人都能访问所有 Agent 的统计信息。需要设计细粒度的权限模型。

### 权限级别设计

```
用户权限:
├── 普通用户 (frp-user)
│   └── 可访问自己的 Agent: user-{owner}-*
├── Admin (frp-admin)
│   └── 可访问所有 Agent 和管理 API
└── 系统 (System Account)
    └── 用于 Operator、Webhook 等内部服务
```

### 决策

**基于 OIDC Realm Roles 的分级权限**

### 考虑的方案

1. **Realm Roles（选中）**: Keycloak 原生支持，简单
2. **Custom Claims**: 在 Token 中添加自定义 claim
3. **ABAC (Attribute-Based)**: 基于对象属性的访问控制
4. **无权限**: 当前状态（不可接受）

### 论证

| 方案 | Realm Roles | Custom Claims | ABAC | 无权限 |
|------|-----------|--------------|------|--------|
| **易实现** | ✅ 是 | ✅ 是 | ❌ 否 | ✅ 是 |
| **灵活性** | ⚠️ 中等 | ✅ 高 | ✅ 最高 | ❌ 无 |
| **运维成本** | ✅ 低 | ⚠️ 中等 | ❌ 高 | ✅ 低 |
| **标准性** | ✅ 标准 | ⚠️ 非标准 | ⚠️ 非标准 | ❌ 不安全 |

### 权衡

- **优势**:
  - 标准做法，易维护
  - Keycloak 原生支持
  - 简单且足够

- **劣势**:
  - 灵活性有限（两个角色不足以应对复杂场景）
  - 需要预先定义好角色

- **结论**: Phase 1 用 Realm Roles，Phase 2 可升级到自定义 Claims

### 实现细节

- Keycloak 中创建角色：`frp-admin`, `frp-user`
- Server 检查 `realm_access.roles` claim
- Operator 使用专用 Service Account（带 `frp-admin` 角色）

### 权限检查函数

```go
func canAccess(idToken *oidc.IDToken, agentID string) bool {
    // 提取用户信息
    var claims struct {
        PreferredUsername string `json:"preferred_username"`
        RealmAccess struct {
            Roles []string `json:"roles"`
        } `json:"realm_access"`
    }
    idToken.Claims(&claims)

    // Admin 可访问所有
    for _, role := range claims.RealmAccess.Roles {
        if role == "frp-admin" {
            return true
        }
    }

    // 普通用户仅能访问自己的 Agent
    // Agent ID 格式: user-{owner}-{name}
    expectedOwner := fmt.Sprintf("user-%s-", claims.PreferredUsername)
    return strings.HasPrefix(agentID, expectedOwner)
}
```

---

## 总结：架构演进路线

```
当前状态 (v0.1)
├── ✅ Deployment + Operator 管理
├── ✅ OIDC 多层认证
├── ✅ Yamux 多路复用
├── ✅ TTL 缩容
├── ❌ Deployment 无法更新
├── ❌ Agent Token 无自动刷新
├── ❌ API 无权限控制
└── ❌ Cookie 存在安全问题

↓

近期改进 (v0.2 - 4-6 周)
├── ✅ Deployment 更新逻辑
├── ✅ Agent Token 自动刷新
├── ✅ Management API 鉴权
├── ✅ SSH 密码 Secret 管理
├── ✅ Cookie 安全加固
└── ✅ CRD Status 实现

↓

中期扩展 (v1.0 - 8-12 周)
├── ✅ CRD v1beta1 版本（GPU/存储支持）
├── ✅ 自动扩容机制
├── ✅ Webhook 认证
├── ✅ Session 共享（多实例）
└── ✅ 完整的 E2E 测试

↓

长期愿景 (v2.0 - 6+ 月)
├── CRD v2 版本（完整 Pod 配置）
├── Sidecar 自动注入
├── 高可用 Operator（多副本）
├── 集成 Sealed Secrets/Vault
├── OpenTelemetry 可观测性
└── Operator Hub 发布
```

---

## 参考资源

- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [OIDC 规范](https://openid.net/connect/)
- [OAuth2 RFC 6749](https://tools.ietf.org/html/rfc6749)
- [Yamux 文档](https://github.com/hashicorp/yamux)
- [Kubernetes CRD 最佳实践](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)

---

最后更新: 2025-12-06
维护者: KubeRDE 项目团队
