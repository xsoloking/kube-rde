# FRP Security Operations Guide

**Version**: 1.0
**Last Updated**: 2025-12-08
**Audience**: Security engineers, platform operators

---

## Table of Contents

1. [Security Overview](#security-overview)
2. [RBAC Best Practices](#rbac-best-practices)
3. [Secret Management](#secret-management)
4. [Network Policies](#network-policies)
5. [Multi-Tenancy Isolation](#multi-tenancy-isolation)
6. [Authentication & Authorization](#authentication--authorization)
7. [Audit Logging](#audit-logging)
8. [Security Hardening](#security-hardening)
9. [Incident Response](#incident-response)

---

## Security Overview

### Security Model

FRP implements defense-in-depth with multiple security layers:

```
┌─────────────────────────────────────────────┐
│          Layer 1: Network Security          │
│  - Ingress TLS termination                  │
│  - Network policies for pod isolation       │
│  - Internal cluster DNS only                │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│     Layer 2: Authentication (OIDC/OAuth2)   │
│  - Keycloak identity provider               │
│  - JWT token validation via JWKS            │
│  - Client credentials for agents            │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│     Layer 3: Authorization (Claims-Based)   │
│  - Owner-based agent access control         │
│  - Realm roles for admin access (planned)   │
│  - Resource isolation via CRD ownership     │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│       Layer 4: Runtime Isolation            │
│  - Kubernetes RBAC for operators            │
│  - Pod security standards                   │
│  - Resource limits and quotas               │
└─────────────────────────────────────────────┘
```

### Threat Model

**Protected Against**:
- Unauthorized agent access (multi-tenant isolation)
- Token theft/replay (short-lived JWTs, HTTPS-only)
- Privilege escalation (RBAC, owner validation)
- Resource exhaustion (limits, quotas)
- Network-level attacks (policies, ingress WAF)

**Planned Improvements**:
- Session hijacking (server-side sessions)
- Credential leakage (Secrets instead of env vars)
- Man-in-the-middle (mTLS for server-agent)
- Advanced persistent threats (audit logging)

### Security Responsibilities

| Role | Responsibilities |
|------|------------------|
| **Platform Team** | K8s RBAC, network policies, secret management |
| **Security Team** | Audit reviews, compliance, incident response |
| **Developers** | Secure code, dependency updates, secrets handling |
| **Users** | SSH key management, token protection, reporting issues |

---

## RBAC Best Practices

### Current RBAC Configuration

**Operator Service Account**:

```yaml
# File: deploy/k8s/03-operator.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kuberde-operator
  namespace: kuberde

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kuberde-operator-role
  namespace: kuberde
rules:
- apiGroups: ["kuberde.io"]
  resources: ["rdeagents"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["kuberde.io"]
  resources: ["rdeagents/status"]
  verbs: ["get", "update", "patch"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

### Principle of Least Privilege

**Current State**:
- Operator can manage Deployments in `kuberde` namespace only
- Cannot access Secrets (improvement needed)
- Cannot modify CRDs (only instances)

**Recommended Improvements**:

1. **Add Secret Management**:
```yaml
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "create", "update", "patch"]
  # Restrict to frp-agent-* secrets only (via admission webhook)
```

2. **Separate Read/Write Roles**:
```yaml
# Read-only role for monitoring
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: frp-readonly
rules:
- apiGroups: ["kuberde.io"]
  resources: ["rdeagents"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch"]
```

3. **User RBAC for CRD Management**:
```yaml
# Allow users to manage their own agents only
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: frp-user
rules:
- apiGroups: ["kuberde.io"]
  resources: ["rdeagents"]
  verbs: ["get", "list", "create", "delete"]
  # Note: Update restricted to prevent changing owner
```

### RBAC Verification

**Check current permissions**:
```bash
# What can operator service account do?
kubectl auth can-i --list --as=system:serviceaccount:kuberde:kuberde-operator -n kuberde

# Check specific action
kubectl auth can-i create deployments \
  --as=system:serviceaccount:kuberde:kuberde-operator \
  -n kuberde
```

**Audit RBAC changes**:
```bash
# List all roles and bindings
kubectl get role,rolebinding -n kuberde

# Describe specific role
kubectl describe role kuberde-operator-role -n kuberde

# Check for overly permissive rules
kubectl get role -A -o json | jq '.items[] | select(.rules[] | .verbs[] == "*")'
```

### User Access Control

**Create user-specific namespace** (recommended for larger deployments):

```bash
# Create namespace for user
kubectl create namespace frp-user-alice

# Bind user to manage RDEAgents in their namespace
kubectl create rolebinding alice-frp-admin \
  --clusterrole=frp-user \
  --user=alice \
  --namespace=frp-user-alice
```

**Prevent cross-namespace access**:
```yaml
# Admission webhook to validate owner matches namespace
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: frp-agent-validator
webhooks:
- name: rdeagent.kuberde.io
  rules:
  - apiGroups: ["kuberde.io"]
    resources: ["rdeagents"]
    operations: ["CREATE", "UPDATE"]
  clientConfig:
    service:
      name: frp-admission-webhook
      namespace: kuberde
```

---

## Secret Management

### Current Issues

**Problem**: SSH passwords and OAuth2 credentials stored in plain environment variables

```yaml
# INSECURE - Current implementation
env:
- name: USER_PASSWORD
  value: "password-testuser"  # Visible in pod spec!
- name: AUTH_CLIENT_SECRET
  value: "supersecret"  # Stored in etcd unencrypted!
```

### Recommended Solution: Kubernetes Secrets

**Step 1: Create Secret**

```bash
# Create auth secret for agent
kubectl create secret generic frp-agent-testuser-auth \
  --from-literal=client-id=frp-agent \
  --from-literal=client-secret=$(openssl rand -base64 32) \
  -n kuberde

# Create SSH credentials secret
kubectl create secret generic frp-agent-testuser-ssh \
  --from-literal=password=$(openssl rand -base64 24) \
  --from-literal=public-key="ssh-rsa AAAAB3..." \
  -n kuberde
```

**Step 2: Reference in RDEAgent CRD**

```yaml
apiVersion: kuberde.io/v1
kind: RDEAgent
metadata:
  name: agent-001
spec:
  owner: testuser
  authSecret: frp-agent-testuser-auth
  sshSecretRef:  # New field (planned)
    name: frp-agent-testuser-ssh
```

**Step 3: Operator Updates Deployment**

```yaml
# Generated by operator
env:
- name: AUTH_CLIENT_ID
  valueFrom:
    secretKeyRef:
      name: frp-agent-testuser-auth
      key: client-id
- name: AUTH_CLIENT_SECRET
  valueFrom:
    secretKeyRef:
      name: frp-agent-testuser-auth
      key: client-secret
- name: USER_PASSWORD
  valueFrom:
    secretKeyRef:
      name: frp-agent-testuser-ssh
      key: password
```

### Sealed Secrets (Production)

**Install Sealed Secrets Controller**:

```bash
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.0/controller.yaml

# Install kubeseal CLI
brew install kubeseal
```

**Create Sealed Secret**:

```bash
# Create regular secret
kubectl create secret generic my-secret \
  --from-literal=password=supersecret \
  --dry-run=client -o yaml > secret.yaml

# Seal it (encrypted, safe for Git)
kubeseal < secret.yaml > sealed-secret.yaml

# Commit sealed-secret.yaml to Git
git add deploy/k8s/secrets/sealed-secret.yaml
```

**Sealed Secret Example**:

```yaml
# File: deploy/k8s/secrets/frp-agent-auth-sealed.yaml
apiVersion: bitnami.com/v1alpha1
kind: SealedSecret
metadata:
  name: frp-agent-testuser-auth
  namespace: kuberde
spec:
  encryptedData:
    client-id: AgBQ8F7h3...  # Encrypted with cluster key
    client-secret: AgCx9Mn2k...
```

### External Secrets (Enterprise)

**HashiCorp Vault Integration**:

```yaml
# File: deploy/k8s/secrets/external-secret.yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: frp-agent-vault
spec:
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: frp-agent-auth
  data:
  - secretKey: client-secret
    remoteRef:
      key: secret/frp/agents/testuser
      property: client_secret
```

### Secret Rotation

**Automated Rotation Script**:

```bash
#!/bin/bash
# File: scripts/rotate-agent-secrets.sh

NAMESPACE="kuberde"
AGENT_ID="$1"

if [ -z "$AGENT_ID" ]; then
  echo "Usage: $0 <agent-id>"
  exit 1
fi

# Generate new credentials
NEW_SECRET=$(openssl rand -base64 32)

# Update in Keycloak
# (Implementation depends on Keycloak admin API)

# Update Kubernetes secret
kubectl patch secret "frp-agent-${AGENT_ID}-auth" \
  -n "$NAMESPACE" \
  --type=json \
  -p="[{\"op\": \"replace\", \"path\": \"/data/client-secret\", \"value\": \"$(echo -n $NEW_SECRET | base64)\"}]"

# Restart agent pod to pick up new secret
kubectl rollout restart deployment "user-${AGENT_ID}" -n "$NAMESPACE"

echo "Secret rotated for agent: $AGENT_ID"
```

**Rotation Schedule**:
- **Production**: Every 90 days
- **Staging**: Every 30 days
- **Emergency**: On-demand (suspected compromise)

### Secret Access Audit

**Check who can access secrets**:

```bash
# List all service accounts with secret access
kubectl get rolebinding -n kuberde -o json | \
  jq -r '.items[] | select(.roleRef.name | contains("secret")) | .metadata.name'

# Check specific secret
kubectl get secret frp-agent-auth -n kuberde -o yaml | \
  grep "kubernetes.io/service-account.name"
```

**Detect secret leakage**:

```bash
# Check if secrets appear in logs (should be none!)
kubectl logs -n kuberde --all-containers=true --tail=10000 | \
  grep -i "client-secret\|password" | \
  grep -v "secretKeyRef"  # Ignore references
```

---

## Network Policies

### Default Deny Policy

**Deny all ingress by default**:

```yaml
# File: deploy/k8s/network-policies/default-deny.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-ingress
  namespace: kuberde
spec:
  podSelector: {}
  policyTypes:
  - Ingress
```

### Allow Server Ingress

```yaml
# File: deploy/k8s/network-policies/frp-server-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: frp-server-ingress
  namespace: kuberde
spec:
  podSelector:
    matchLabels:
      app: frp-server
  policyTypes:
  - Ingress
  ingress:
  # Allow from ingress controller
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 80
  # Allow from agents within namespace
  - from:
    - podSelector:
        matchLabels:
          component: frp-agent
    ports:
    - protocol: TCP
      port: 80
```

### Isolate Agent Pods

```yaml
# File: deploy/k8s/network-policies/agent-isolation.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: agent-isolation
  namespace: kuberde
spec:
  podSelector:
    matchLabels:
      component: frp-agent
  policyTypes:
  - Ingress
  - Egress
  egress:
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
  # Allow to FRP server
  - to:
    - podSelector:
        matchLabels:
          app: frp-server
    ports:
    - protocol: TCP
      port: 80
  # Allow to Keycloak
  - to:
    - podSelector:
        matchLabels:
          app: keycloak
    ports:
    - protocol: TCP
      port: 8080
  # Allow to internet (for workload needs)
  - to:
    - namespaceSelector: {}
```

### Keycloak Isolation

```yaml
# File: deploy/k8s/network-policies/keycloak.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: keycloak-policy
  namespace: kuberde
spec:
  podSelector:
    matchLabels:
      app: keycloak
  policyTypes:
  - Ingress
  ingress:
  # Allow from ingress
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
  # Allow from server and agents
  - from:
    - podSelector:
        matchLabels:
          app: frp-server
  - from:
    - podSelector:
        matchLabels:
          component: frp-agent
    ports:
    - protocol: TCP
      port: 8080
```

### Applying Network Policies

```bash
# Apply all policies
kubectl apply -f deploy/k8s/network-policies/

# Verify policies
kubectl get networkpolicy -n kuberde

# Test connectivity (should fail if policy correct)
kubectl run test-pod --rm -i --tty --image=busybox -n kuberde -- \
  wget -O- http://frp-server.kuberde.svc
```

### Troubleshooting Network Policies

```bash
# Check if CNI supports network policies
kubectl get nodes -o yaml | grep "networkPlugin"

# Describe policy
kubectl describe networkpolicy frp-server-ingress -n kuberde

# Test from allowed pod
kubectl exec -it -n kuberde user-testuser-agent-001 -c frp-agent -- \
  wget -O- http://frp-server.kuberde.svc/health
```

---

## Multi-Tenancy Isolation

### Owner-Based Isolation

**How It Works**:
1. Each RDEAgent CRD has `spec.owner` field
2. Agent ID is `user-{owner}-{name}`
3. Server validates JWT `preferred_username` matches owner
4. Only matching user can access agent

**Example Authorization Check** (server code):

```go
func canAccessAgent(idToken *oidc.IDToken, agentID string) bool {
    var claims struct {
        PreferredUsername string `json:"preferred_username"`
    }
    if err := idToken.Claims(&claims); err != nil {
        return false
    }

    // Extract owner from agent ID
    // Format: user-{owner}-{name}
    parts := strings.SplitN(agentID, "-", 3)
    if len(parts) < 3 || parts[0] != "user" {
        return false
    }
    owner := parts[1]

    return claims.PreferredUsername == owner
}
```

### Namespace Isolation (Recommended for Scale)

**Multi-Namespace Architecture**:

```
frp-system/          ← Server, Operator, Keycloak
├── frp-server
├── kuberde-operator
└── keycloak

frp-user-alice/      ← Alice's agents
├── user-alice-dev
└── user-alice-prod

frp-user-bob/        ← Bob's agents
└── user-bob-test
```

**Operator Modifications** (for multi-namespace):

```yaml
# Operator needs cluster-wide permissions
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kuberde-operator-cluster
rules:
- apiGroups: ["kuberde.io"]
  resources: ["rdeagents"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

**Namespace Provisioning**:

```bash
#!/bin/bash
# File: scripts/provision-user-namespace.sh

USERNAME="$1"
NAMESPACE="frp-user-${USERNAME}"

# Create namespace
kubectl create namespace "$NAMESPACE"

# Label for network policies
kubectl label namespace "$NAMESPACE" kuberde.io/user="$USERNAME"

# Create user role binding
kubectl create rolebinding "${USERNAME}-frp-admin" \
  --clusterrole=frp-user \
  --user="$USERNAME" \
  --namespace="$NAMESPACE"

# Create resource quota
kubectl apply -f - <<EOF
apiVersion: v1
kind: ResourceQuota
metadata:
  name: frp-quota
  namespace: $NAMESPACE
spec:
  hard:
    requests.cpu: "10"
    requests.memory: "20Gi"
    persistentvolumeclaims: "5"
    pods: "20"
EOF

echo "Namespace $NAMESPACE provisioned for $USERNAME"
```

### Resource Quotas

**Per-User Limits**:

```yaml
# File: deploy/k8s/resource-quota.yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: frp-user-quota
  namespace: kuberde
spec:
  hard:
    # Limit total resources per namespace
    requests.cpu: "20"
    requests.memory: "40Gi"
    limits.cpu: "40"
    limits.memory: "80Gi"

    # Limit number of agents
    count/rdeagents.kuberde.io: "10"

    # Limit storage
    persistentvolumeclaims: "10"
    requests.storage: "100Gi"
```

**Apply Quotas**:

```bash
kubectl apply -f deploy/k8s/resource-quota.yaml

# Check quota usage
kubectl describe resourcequota frp-user-quota -n kuberde
```

### Preventing Privilege Escalation

**Pod Security Standards**:

```yaml
# File: deploy/k8s/pod-security.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: kuberde
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

**Deployment Security Context**:

```yaml
# In agent deployment
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: frp-agent
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
```

---

## Authentication & Authorization

### OIDC Configuration

**Keycloak Client Settings** (frp-agent):

```json
{
  "clientId": "frp-agent",
  "enabled": true,
  "clientAuthenticatorType": "client-secret",
  "secret": "***",
  "serviceAccountsEnabled": true,
  "standardFlowEnabled": false,
  "directAccessGrantsEnabled": true,
  "attributes": {
    "access.token.lifespan": "900",  // 15 minutes
    "client.secret.rotation.period": "7776000"  // 90 days
  }
}
```

**Server JWKS Validation**:

```go
// Initialize once at startup
provider, err := oidc.NewProvider(ctx, keycloakURL)
verifier := provider.Verifier(&oidc.Config{
    ClientID: "kuberde-cli",
    SkipIssuerCheck: false,  // Validate issuer in production
})

// Validate every request
func validateToken(rawToken string) (*oidc.IDToken, error) {
    token, err := verifier.Verify(context.Background(), rawToken)
    if err != nil {
        return nil, fmt.Errorf("invalid token: %w", err)
    }
    return token, nil
}
```

### Token Security Best Practices

**Short Token Lifetimes**:
- Access tokens: 15 minutes
- Refresh tokens: 8 hours (planned)
- Session cookies: 24 hours

**Token Storage**:
- CLI: `~/.frp/token.json` with 0600 permissions
- Web: HttpOnly, Secure, SameSite cookies
- Agents: Memory only, never persist

**Revocation**:

```bash
# Revoke user sessions in Keycloak
curl -X POST http://keycloak:8080/admin/realms/frp/users/{user-id}/logout \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Force agent reconnect (will get new token)
kubectl rollout restart deployment user-testuser-agent-001 -n kuberde
```

### Future: Role-Based Access Control

**Planned Realm Roles**:

```yaml
# Keycloak role mapping
roles:
  - name: frp-admin
    description: "Full access to all agents and management APIs"
  - name: frp-user
    description: "Access to own agents only"
  - name: frp-readonly
    description: "Read-only access for monitoring"
```

**Server Authorization Logic** (planned):

```go
func checkPermission(token *oidc.IDToken, action string, agentID string) bool {
    var claims struct {
        PreferredUsername string `json:"preferred_username"`
        RealmAccess struct {
            Roles []string `json:"roles"`
        } `json:"realm_access"`
    }
    token.Claims(&claims)

    // Admin can do anything
    if contains(claims.RealmAccess.Roles, "frp-admin") {
        return true
    }

    // User can only access own agents
    if action == "access" && isOwner(claims.PreferredUsername, agentID) {
        return true
    }

    return false
}
```

---

## Audit Logging

### Current Logging

**Server Logs** (structured format):

```json
{
  "timestamp": "2025-12-08T12:34:56Z",
  "level": "info",
  "event": "user_connection",
  "user": "testuser",
  "agent_id": "user-testuser-agent-001",
  "source_ip": "203.0.113.42",
  "auth_method": "oidc"
}
```

**Operator Logs**:

```json
{
  "timestamp": "2025-12-08T12:35:01Z",
  "level": "info",
  "event": "agent_scaled_down",
  "agent_id": "user-testuser-agent-001",
  "reason": "ttl_exceeded",
  "idle_duration": "8h15m"
}
```

### Audit Events to Log

**Authentication Events**:
- Login attempts (success/failure)
- Token validation failures
- Session creation/destruction

**Authorization Events**:
- Agent access attempts
- Permission denials
- Role changes (future)

**Resource Events**:
- RDEAgent creation/deletion
- Deployment scaling
- Secret access

**Security Events**:
- Repeated auth failures (potential brute force)
- Unusual access patterns
- Configuration changes

### Audit Log Retention

```bash
# Elasticsearch retention policy (example)
curl -X PUT "elasticsearch:9200/_ilm/policy/frp-audit-policy" \
  -H 'Content-Type: application/json' -d'
{
  "policy": {
    "phases": {
      "hot": {
        "actions": { "rollover": { "max_age": "7d" } }
      },
      "warm": {
        "min_age": "30d",
        "actions": { "readonly": {} }
      },
      "delete": {
        "min_age": "365d",
        "actions": { "delete": {} }
      }
    }
  }
}'
```

### Compliance Requirements

**SOC 2 / ISO 27001**:
- Log all authentication events
- Retain for 1 year minimum
- Tamper-proof storage
- Regular review process

**GDPR**:
- Log personal data access
- Support data deletion requests
- Anonymize logs after retention period

---

## Security Hardening

### Container Image Security

**Scan images for vulnerabilities**:

```bash
# Using Trivy
trivy image ghcr.io/yourorg/frp-server:latest

# Using Grype
grype ghcr.io/yourorg/frp-agent:latest
```

**Multi-stage builds** (already implemented):

```dockerfile
# Build stage
FROM golang:1.24 AS builder
# ... build steps

# Runtime stage (minimal)
FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/server /server
USER 1000
ENTRYPOINT ["/server"]
```

**Image signing** (recommended):

```bash
# Sign with Cosign
cosign sign --key cosign.key ghcr.io/yourorg/frp-server:latest

# Verify signature
cosign verify --key cosign.pub ghcr.io/yourorg/frp-server:latest
```

### Ingress Security

**TLS Configuration**:

```yaml
# File: deploy/k8s/06-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: frp-ingress
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
spec:
  tls:
  - hosts:
    - "*.frp.byai.uk"
    - frp.byai.uk
    secretName: frp-tls-cert
```

**Rate Limiting**:

```yaml
annotations:
  nginx.ingress.kubernetes.io/limit-rps: "100"
  nginx.ingress.kubernetes.io/limit-connections: "10"
```

**WAF Integration** (ModSecurity):

```yaml
annotations:
  nginx.ingress.kubernetes.io/enable-modsecurity: "true"
  nginx.ingress.kubernetes.io/modsecurity-snippet: |
    SecRuleEngine On
    SecRule ARGS "@contains <script" "id:1,deny,status:403"
```

### Dependency Updates

**Automated scanning** (Dependabot):

```yaml
# File: .github/dependabot.yml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 10
```

**Manual check**:

```bash
# Check for outdated dependencies
go list -u -m all | grep '\['

# Update dependencies
go get -u ./...
go mod tidy
```

---

## Incident Response

### Suspected Compromise

**Immediate Actions**:

1. **Isolate affected agents**:
```bash
kubectl scale deployment user-{compromised}-{agent} --replicas=0 -n kuberde
```

2. **Revoke credentials**:
```bash
# In Keycloak admin console or via API
# Revoke all sessions for user
```

3. **Rotate secrets**:
```bash
./scripts/rotate-agent-secrets.sh {agent-id}
```

4. **Collect forensics**:
```bash
# Save logs
kubectl logs user-{compromised}-{agent} -c frp-agent > agent.log
kubectl logs user-{compromised}-{agent} -c workload > workload.log

# Save events
kubectl get events -n kuberde --sort-by='.lastTimestamp' > events.log
```

### Brute Force Attack

**Detection**:
```promql
# Alert if auth failures > 50 in 5 minutes
rate(frp_auth_failures_total[5m]) > 10
```

**Response**:
1. Enable rate limiting on ingress
2. Add IP to blocklist
3. Require MFA for affected users (Keycloak setting)

### Data Breach

**Checklist**:
- [ ] Identify scope (which agents, which users)
- [ ] Preserve evidence (logs, pod specs)
- [ ] Notify affected users (GDPR requirement)
- [ ] Rotate all secrets
- [ ] Review and update security policies
- [ ] Post-mortem analysis

### Communication Plan

**Internal**:
- Slack channel: `#frp-incidents`
- PagerDuty escalation
- Security team notification

**External** (if applicable):
- User notification via email
- Status page update
- Regulatory reporting (if required)

---

## Related Documentation

- [Monitoring Guide](MONITORING.md) - Security metrics and alerts
- [Operators Runbook](../guides/OPERATORS_RUNBOOK.md) - Daily operations
- [Architecture](../ARCHITECTURE.md) - System design and threat model
- [Deployment Guide](../guides/DEPLOYMENT.md) - Production deployment security

---

**Document Version**: 1.0
**Maintainer**: Security Team
**Next Review**: 2025-03-08
