# KubeRDE Helm Chart

Official Helm chart for deploying KubeRDE (Kubernetes Remote Development Environment) to Kubernetes with support for both local development and production deployments.

## TL;DR

```bash
# Local development with nip.io (HTTP)
helm install kuberde ./charts/kuberde -f charts/kuberde/values-http.yaml -n kuberde --create-namespace

# development with custom domains (HTTPS)
helm install kuberde ./charts/kuberde -f charts/kuberde/values-https.yaml \
  --set global.domain=your.domain.com \
  --set global.keycloakDomain=sso.your.domain.com \
  --set global.agentDomain=*.your.domain.com \
  --set global.protocol=https \
  --set secrets.keycloak.adminPassword=STRONG_PASSWORD \
  --set secrets.keycloak.realm.keycloak.password=STRONG_PASSWORD \
  --set secrets.database.password=STRONG_PASSWORD \
  --set secrets.keycloakClients.serverClientSecret=STRONG_SECRET \
  --set secrets.keycloakClients.agentClientSecret=STRONG_SECRET \
  --set secrets.github.enabled=true \
  --set secrets.github.clientId=STRONG_SECRET \
  --set secrets.github.clientSecret=STRONG_SECRET \
  -n kuberde --create-namespace
```

## Prerequisites

- Kubernetes 1.28+
- Helm 3.8+
- **Traefik ingress controller** (default, required for both HTTP and HTTPS modes)
- Persistent Volume provisioner support (for PostgreSQL)
- **For HTTPS mode**:
  - Traefik configured with Let's Encrypt cert resolver (named "letsencrypt" by default)
  - External DNS configured (e.g., Cloudflare) OR manual DNS records
  - Valid domain names pointing to your cluster

## Deployment Modes

This Helm chart supports two deployment modes:

### 1. Local Development Mode (HTTP + nip.io)

Perfect for local testing with k3d, kind, or minikube:

- Uses `nip.io` for automatic DNS resolution (no DNS configuration needed)
- HTTP-only (no TLS certificates needed)
- Simple passwords suitable for development
- Example: `http://127-0-0-1.nip.io`, `http://sso.127-0-0-1.nip.io`

```bash
helm install kuberde ./charts/kuberde -f charts/kuberde/values-http.yaml -n kuberde --create-namespace
```

### 2. Production Mode (HTTPS + Custom Domains)

Production-ready deployment with:

- Custom domain names (e.g., `frp.byai.uk`, `sso.byai.uk`)
- HTTPS with TLS certificates (auto-generated via Traefik + Let's Encrypt)
- External DNS integration for automatic DNS record creation
- Strong passwords and secrets
- High availability configuration (multiple replicas)

```bash
helm install kuberde ./charts/kuberde -f charts/kuberde/values-production.yaml \
  --set secrets.keycloak.adminPassword=STRONG_PASSWORD \
  --set secrets.database.password=STRONG_PASSWORD \
  --set secrets.keycloakClients.serverClientSecret=STRONG_SECRET \
  --set secrets.keycloakClients.agentClientSecret=STRONG_SECRET
```

## Installing the Chart

### Quick Install (Local Development)

For local testing with default values using nip.io:

```bash
helm install kuberde ./charts/kuberde -f charts/kuberde/values-local-dev.yaml
```

Access URLs:
- Web UI: `http://127-0-0-1.nip.io`
- Keycloak SSO: `http://sso.127-0-0-1.nip.io`
- Agents: `http://*.127-0-0-1.nip.io`

Default credentials:
- Username: `admin`
- Password: `password`

### Install (Https + Custom Domains)

**Prerequisites for Production:**
1. Traefik ingress controller with Let's Encrypt cert resolver configured
2. DNS records pointing to your cluster (or External DNS configured)
3. Strong passwords generated

**Step 1: Configure DNS (if not using External DNS)**

Create DNS A records pointing to your cluster's ingress IP:
```
frp.byai.uk       A  <your-ingress-ip>
sso.byai.uk       A  <your-ingress-ip>
*.frp.byai.uk     A  <your-ingress-ip>
```

**Step 2: Install with production values**

```bash
helm install kuberde ./charts/kuberde -f charts/kuberde/values-https.yaml \
  --set global.domain=your.domain.com \
  --set global.keycloakDomain=sso.your.domain.com \
  --set global.agentDomain=*.your.domain.com \
  --set global.protocol=https \
  --set secrets.keycloak.adminPassword=STRONG_PASSWORD \
  --set secrets.keycloak.realm.keycloak.password=STRONG_PASSWORD \
  --set secrets.database.password=STRONG_PASSWORD \
  --set secrets.keycloakClients.serverClientSecret=STRONG_SECRET \
  --set secrets.keycloakClients.agentClientSecret=STRONG_SECRET \
  --set secrets.github.enabled=true \
  --set secrets.github.clientId=STRONG_SECRET \
  --set secrets.github.clientSecret=STRONG_SECRET \
  -n kuberde --create-namespace
```

**Step 3: Wait for certificate provisioning**

Certificate provisioning via Let's Encrypt may take 5-10 minutes:

```bash
# Watch certificate status
watch kubectl get certificate -n kuberde

# Check Traefik logs if certificates fail
kubectl logs -n traefik deploy/traefik
```

Access URLs:
- Web UI: `https://frp.byai.uk`
- Keycloak SSO: `https://sso.byai.uk`
- Agents: `https://*.frp.byai.uk`

### Custom Installation

Create a `my-values.yaml` file:

```yaml
global:
  domain: "dev.example.com"
  keycloakDomain: "sso.example.com"
  agentDomain: "dev.example.com"
  protocol: https

server:
  replicaCount: 2

ingress:
  enabled: true
  className: traefik
  http:
    enabled: false  # Disable HTTP
  https:
    enabled: true   # Enable HTTPS
    annotations:
      traefik.ingress.kubernetes.io/router.tls.certresolver: letsencrypt
      external-dns.alpha.kubernetes.io/hostname: dev.example.com,sso.example.com
    tls:
      enabled: true

secrets:
  keycloak:
    adminPassword: "your-secure-password"
  database:
    password: "your-secure-password"
  keycloakClients:
    serverClientSecret: "your-server-secret"
    agentClientSecret: "your-agent-secret"
```

Install:

```bash
helm install kuberde ./charts/kuberde -f my-values.yaml
```

## Configuration

### Global Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.domain` | Main domain for KubeRDE | `kuberde.local` |
| `global.keycloakDomain` | Keycloak SSO domain | `sso.kuberde.local` |
| `global.agentDomain` | Agent wildcard domain | `kuberde.local` |
| `global.protocol` | Protocol (http or https) | `http` |

### Image Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image repository | `soloking` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `image.tag` | Image tag | `latest` |

### Server Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `server.enabled` | Enable server component | `true` |
| `server.replicaCount` | Number of replicas | `1` |
| `server.service.port` | HTTP/WebSocket port | `8080` |
| `server.resources.limits.cpu` | CPU limit | `1000m` |
| `server.resources.limits.memory` | Memory limit | `1Gi` |
| `server.healthCheck.enabled` | Enable health checks | `true` |

### Operator Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.enabled` | Enable operator | `true` |
| `operator.replicaCount` | Number of replicas | `1` |
| `operator.rbac.create` | Create RBAC resources | `true` |
| `operator.resources.limits.cpu` | CPU limit | `500m` |
| `operator.resources.limits.memory` | Memory limit | `512Mi` |
| `operator.healthCheck.enabled` | Enable health checks | `true` |

### Web UI Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `web.enabled` | Enable Web UI | `true` |
| `web.replicaCount` | Number of replicas | `1` |
| `web.service.port` | Service port | `80` |
| `web.healthCheck.enabled` | Enable health checks | `true` |

### Keycloak Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `keycloak.enabled` | Enable Keycloak | `true` |
| `keycloak.replicaCount` | Number of replicas | `1` |
| `keycloak.image` | Keycloak image | `quay.io/keycloak/keycloak` |
| `keycloak.tag` | Keycloak version | `23.0.0` |
| `keycloak.hostname.strict` | Strict hostname checking | `false` |
| `keycloak.hostname.strictHttps` | Require HTTPS | `false` |
| `keycloak.hostname.httpEnabled` | Enable HTTP | `true` |
| `keycloak.healthCheck.enabled` | Enable health checks | `true` |

### PostgreSQL Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.enabled` | Enable PostgreSQL | `true` |
| `postgresql.replicaCount` | Number of replicas | `1` |
| `postgresql.image` | PostgreSQL image | `postgres` |
| `postgresql.tag` | PostgreSQL version | `15-alpine` |
| `postgresql.persistence.enabled` | Enable persistence | `true` |
| `postgresql.persistence.size` | PVC size | `10Gi` |
| `postgresql.persistence.storageClass` | Storage class | `""` (default) |
| `postgresql.healthCheck.enabled` | Enable health checks | `true` |

### Ingress Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Enable Ingress | `true` |
| `ingress.className` | Ingress class name | `traefik` |
| `ingress.http.enabled` | Enable HTTP ingress | `false` |
| `ingress.http.entrypoint` | Traefik entrypoint for HTTP | `web` |
| `ingress.https.enabled` | Enable HTTPS ingress | `false` |
| `ingress.https.entrypoint` | Traefik entrypoint for HTTPS | `websecure` |
| `ingress.https.tls.enabled` | Enable TLS | `false` |
| `ingress.wildcard.enabled` | Enable wildcard routing | `true` |
| `ingress.priority.main` | Main ingress priority | `50` |
| `ingress.priority.keycloak` | Keycloak ingress priority | `100` |

### Secrets Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `secrets.create` | Create secrets from values | `true` |
| `secrets.keycloak.adminUser` | Keycloak admin username | `kuberde-admin` |
| `secrets.keycloak.adminPassword` | Keycloak admin password | `""` |
| `secrets.keycloak.realm.keycloak.username` | Keycloak realm username | `kuberde` |
| `secrets.keycloak.realm.keycloak.password` | Keycloak realm password | `""` |
| `secrets.database.user` | Database user | `kuberde` |
| `secrets.database.password` | Database password | `""` |
| `secrets.database.name` | Database name | `kuberde` |
| `secrets.keycloakClients.serverClientSecret` | Server client secret | `""` |
| `secrets.keycloakClients.agentClientSecret` | Agent client secret | `""` |
| `secrets.github.enabled` | Enable GitHub OAuth | `false` |
| `secrets.github.clientId` | GitHub OAuth client ID | `""` |
| `secrets.github.clientSecret` | GitHub OAuth client secret | `""` |

### CRD Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `crds.install` | Install CRDs | `true` |
| `crds.keep` | Keep CRDs on uninstall | `true` |

See [values.yaml](values.yaml) for the complete list of configuration options.

## Uninstalling the Chart

```bash
helm uninstall kuberde -n kuberde
```

This removes all Kubernetes components associated with the chart and deletes the release.

**Note:** Persistent Volume Claims (PVCs) are not deleted automatically. To delete them:

```bash
kubectl delete pvc -l app.kubernetes.io/instance=kuberde -n kuberde
```

## Upgrading

### Upgrading the Chart

```bash
helm upgrade kuberde ./charts/kuberde -f my-values.yaml
```

### Upgrading to a New Version

```bash
helm repo update
helm upgrade kuberde kuberde/kuberde --version 0.2.0
```

## Troubleshooting

### Check Release Status

```bash
helm status kuberde
helm get values kuberde
helm get manifest kuberde
```

### Common Issues

#### Pods Not Starting

```bash
kubectl get pods -n kuberde -l app.kubernetes.io/instance=kuberde
kubectl describe pod <pod-name> -n kuberde
kubectl logs <pod-name> -n kuberde
```

#### Ingress Not Working

```bash
kubectl get ingress -n kuberde
kubectl describe ingress -n kuberde
```

#### Certificate Issues (HTTPS Mode)

```bash
# Check Traefik configuration
kubectl get ingressroute -A
kubectl logs -n traefik deploy/traefik

# Check if cert resolver is configured
kubectl describe ingressroute -n kuberde
```

#### Database Connection Issues

```bash
kubectl logs -n kuberde -l app.kubernetes.io/name=kuberde-server
kubectl exec -it <server-pod> -n kuberde -- env | grep DB_
```

#### Health Check Failures

```bash
# Test health endpoints
kubectl port-forward -n kuberde svc/kuberde-server 8080:8080
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz

# Check operator health
kubectl port-forward -n kuberde deploy/kuberde-operator 8080:8080
curl http://localhost:8080/healthz
```

## Security Considerations

**Before deploying to production:**

1. **Change default passwords**:
   ```yaml
   secrets:
     keycloak:
       adminPassword: "strong-random-password"
       realm:
         keycloak:
           password: "strong-random-password"
     database:
       password: "strong-random-password"
     keycloakClients:
       serverClientSecret: "strong-random-secret"
       agentClientSecret: "strong-random-secret"
   ```

2. **Use Kubernetes Secrets** (recommended for production):
   ```bash
   # Create secrets manually
   kubectl create secret generic keycloak-admin-client-secret \
     --from-literal=client-id=kuberde-server-admin \
     --from-literal=client-secret=YOUR_SECRET \
     -n kuberde

   # Reference existing secrets
   secrets:
     create: false
     existingSecrets:
       keycloakAdmin: keycloak-admin-client-secret
       postgresql: postgresql-secret
   ```

3. **Enable TLS/HTTPS**:
   ```yaml
   global:
     protocol: https
   ingress:
     http:
       enabled: false
     https:
       enabled: true
       tls:
         enabled: true
   ```

4. **Configure Traefik cert resolver**:
   Ensure Traefik is configured with Let's Encrypt:
   ```yaml
   # In Traefik's values
   additionalArguments:
     - --certificatesresolvers.letsencrypt.acme.email=your-email@example.com
     - --certificatesresolvers.letsencrypt.acme.storage=/data/acme.json
     - --certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web
   ```

5. **Use external database for production**:
   ```yaml
   postgresql:
     enabled: false

   server:
     env:
       DB_HOST: "your-rds-endpoint.amazonaws.com"
       DB_PORT: "5432"
   ```

6. **Configure resource limits** appropriately for your workload

7. **Enable monitoring and alerting**:
   ```yaml
   serviceMonitor:
     enabled: true
   ```

## Development

### Testing the Chart Locally

```bash
# Lint the chart
helm lint ./charts/kuberde

# Dry run to check generated manifests
helm install kuberde ./charts/kuberde --dry-run --debug -f charts/kuberde/values-http.yaml

# Template to see rendered manifests
helm template kuberde ./charts/kuberde -f charts/kuberde/values-http.yaml > manifests.yaml
```

### Packaging the Chart

```bash
helm package ./charts/kuberde
```

## Examples

### Minimal Local Setup (nip.io + HTTP)

```bash
helm install kuberde ./charts/kuberde -f charts/kuberde/values-http.yaml
```

### Production with Custom Domains (HTTPS)

```bash
helm install kuberde ./charts/kuberde -f charts/kuberde/values-https.yaml \
  --set global.domain=your.domain.com \
  --set global.keycloakDomain=sso.your.domain.com \
  --set global.agentDomain=*.your.domain.com \
  --set global.protocol=https \
  --set secrets.keycloak.adminPassword=STRONG_PASSWORD \
  --set secrets.keycloak.realm.keycloak.password=STRONG_PASSWORD \
  --set secrets.database.password=STRONG_PASSWORD \
  --set secrets.keycloakClients.serverClientSecret=STRONG_SECRET \
  --set secrets.keycloakClients.agentClientSecret=STRONG_SECRET \
  --set secrets.github.enabled=true \
  --set secrets.github.clientId=STRONG_SECRET \
  --set secrets.github.clientSecret=STRONG_SECRET \
  -n kuberde --create-namespace
```

### Production with External Database

```yaml
# custom-values.yaml
postgresql:
  enabled: false  # Using external database

external:
  postgresql:
    enabled: true  # Using external database
    host: "rds.example.com"
    port: 5432
    user: "kuberde"
    password: "STRONG_PASSWORD"
    database: "kuberde"
    existingSecrets: "existing-database-secret"  # Reference existing secret 
```

```bash
helm install kuberde ./charts/kuberde -f custom-values.yaml
```

## Support

- **Documentation**: https://github.com/xsoloking/kube-rde/tree/main/docs
- **Issues**: https://github.com/xsoloking/kube-rde/issues
- **Discussions**: https://github.com/xsoloking/kube-rde/discussions

## License

This Helm chart is licensed under the MIT License. See [LICENSE](../../LICENSE) for details.
