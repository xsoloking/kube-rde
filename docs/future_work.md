# KubeRDE Project - 待优化和扩展的功能

本项目已实现核心功能，但仍有许多方面可以进一步优化和扩展，以提高可用性、安全性和健壮性。

### 2.2. OIDC 认证与授权优化

*   **Server 端 OIDC 重试**: FRP Server 启动时 OIDC Provider 初始化失败后，应该实现指数退避重试机制，而不是直接禁用 Auth。这能提高系统的健壮性，避免 Keycloak 瞬时不可用导致整个 FRP Server 认证失败。
*   **Token Refresh**:
    *   **CLI (`kuberde-cli`)**: 应该实现 Access Token 的自动刷新功能，利用 Refresh Token 延长用户会话，避免频繁登录。
    *   **Agent**: Agent Sidecar 应实现 Access Token 的自动刷新，确保与 Server 的连接不会因 Token 过期而被断开。
*   **Cookie 安全**: 生产环境中，Server 设置的 OIDC Session Cookie (`frp_session`) 应该：
    *   **加密存储**：Cookie Value (目前直接是 ID Token) 应该加密，或只存储一个指向服务端会话的 ID。
    *   **Secure 属性**: 必须设置为 `Secure=true` (只通过 HTTPS 发送)。
    *   **SameSite 属性**: 设置为 `Lax` 或 `Strict` 防止 CSRF 攻击。
*   **更细粒度的授权**:
    *   除了 Owner 绑定，可以结合 Keycloak 的 Role/Group Claim 实现更灵活的权限控制。例如，只有 `frp-admin` 角色用户才能访问 `/mgmt` 管理 API。
    *   支持 Agent ID 与 CRD Name 不一致时的权限管理 (例如 Agent ID 是 UUID，但 Owner 字段仍然存在)。
*   **OIDC Discover URL / Issuer 匹配**: 优化 Server 对 Keycloak URL 的处理，减少 `SkipIssuerCheck` 的必要性。可以考虑：
    *   在 Server Pod 中配置 `etc/hosts` 或 CoreDNS，将公网 Issuer 域名解析到 Keycloak Service 的 ClusterIP。
    *   在 Keycloak 配置中强制设置其内部和外部的 Issuer URL 一致。

### 2.4. 安全性改进

*   **SSH 密码管理**: 自动生成的 SSH 密码不应直接明文存储在 Deployment 的 `Env` 变量中。应存储在 Secret 中，并通过 `valueFrom.secretKeyRef` 引用。Operator 负责创建和管理这些 Secret。
*   **K8s Service Account Token**: FRP Agent 使用 K8s Service Account Token 进行 Keycloak 认证。应确保 K8s Service Account 具备最小权限，并且只绑定到 FRP Agent Deployment。
*   **Server Management API 鉴权**: `/mgmt/agents/{id}` API 目前没有鉴权。应添加鉴权，例如通过 OIDC Token 检查用户是否是 `admin` 角色或拥有特定权限。
*   **日志敏感信息**: 确保日志中不包含敏感信息（如 Token、Secret）。

---
