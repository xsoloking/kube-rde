# KubeRDE Quick Start Guide

Get started with KubeRDE in minutes. Choose your deployment path:

- **Option 1: Try It Locally (5 minutes)** - Perfect for testing and development
- **Option 2: Deploy to Production (15 minutes)** - Full deployment to cloud or on-premises

## Option 1: Try It Locally (Recommended for First-Time Users)

The fastest way to experience KubeRDE without any cloud setup or domain configuration.

### Prerequisites
- Docker installed
- 8GB+ RAM available
- 20GB+ free disk space

### Quick Start with kind

```bash
# Install kind (if not already installed)
# macOS
brew install kind

# Linux
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind

# Create local cluster with KubeRDE
curl -sSL https://raw.githubusercontent.com/xsoloking/kube-rde/main/scripts/quick-start-kind.sh | bash

# Or manually:
kind create cluster --name kuberde --config=deploy/kind/config.yaml
kubectl apply -f deploy/k8s/all-in-one-local.yaml

# Access KubeRDE
echo "127.0.0.1 kuberde.local" | sudo tee -a /etc/hosts
open http://kuberde.local
```

**Default login**: admin / admin

### Quick Start with minikube

```bash
# Start minikube
minikube start --cpus=4 --memory=8192 --addons=ingress

# Deploy KubeRDE
kubectl apply -f deploy/k8s/all-in-one-local.yaml

# Access KubeRDE
echo "$(minikube ip) kuberde.local" | sudo tee -a /etc/hosts
open http://kuberde.local
```

**Next steps after local setup:**
1. Login with admin / admin
2. Create a workspace
3. Launch a development environment
4. When ready, proceed to Option 2 for production deployment

For detailed local deployment options (kind, minikube, k3d, Docker Desktop), see [Local Kubernetes Guide](platforms/local-k8s.md).

---

## Option 2: Deploy to Production

Full production deployment with custom domain, TLS certificates, and cloud integration.

### Choose Your Platform

- [Google Cloud (GKE)](platforms/gcp-gke.md) - Fully managed with GCE Ingress
- [AWS (EKS)](platforms/aws-eks.md) - AWS Load Balancer Controller
- [Azure (AKS)](platforms/azure-aks.md) - Application Gateway or NGINX
- [Your Own Kubernetes](QUICK_START.md#prerequisites-checklist) - Follow guide below

### Prerequisites Checklist

Before starting, ensure you have:

- [ ] Kubernetes cluster (1.28+) with `kubectl` access
- [ ] Ingress controller installed (NGINX recommended)
- [ ] Domain name with DNS access
- [ ] cert-manager installed (optional, for automatic TLS)

## Step 1: Prepare Your Domain

Choose your domain configuration strategy:

### Option A: Single Domain (Simplest)
Best for: Testing, small deployments, single domain availability

**DNS Records:**
```
kuberde.com          A/CNAME  -> <your-ingress-ip>
*.kuberde.com        A/CNAME  -> <your-ingress-ip>
```

**Environment Variables:**
```bash
export KUBERDE_PUBLIC_URL=https://kuberde.com
export KUBERDE_AGENT_DOMAIN=kuberde.com
export KEYCLOAK_URL=https://kuberde.com/auth
export KEYCLOAK_PUBLIC_URL=https://kuberde.com/auth
```

### Option B: Subdomain-based (Recommended)
Best for: Production deployments, better organization

**DNS Records:**
```
kuberde.com          A/CNAME  -> <your-ingress-ip>
*.kuberde.com        A/CNAME  -> <your-ingress-ip>
sso.kuberde.com      A/CNAME  -> <your-ingress-ip>
```

**Environment Variables:**
```bash
export KUBERDE_PUBLIC_URL=https://kuberde.com
export KUBERDE_AGENT_DOMAIN=kuberde.com
export KEYCLOAK_URL=https://sso.kuberde.com
export KEYCLOAK_PUBLIC_URL=https://sso.kuberde.com
```

## Step 2: Get Your Ingress IP

```bash
# For NGINX Ingress
kubectl get svc -n ingress-nginx ingress-nginx-controller

# For other ingress controllers
kubectl get svc -A | grep ingress
```

Look for the `EXTERNAL-IP` column.

## Step 3: Configure DNS

Point your domain to the Ingress IP from Step 2.

**Verify DNS propagation:**
```bash
nslookup kuberde.com
nslookup user-test.kuberde.com
```

Both should return your Ingress IP.

## Step 4: Install cert-manager (Optional)

For automatic TLS certificates:

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Wait for cert-manager to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=300s
```

## Step 5: Customize Configuration

### 5.1 Update Keycloak Configuration

Choose the Keycloak configuration that matches your domain setup:

**For Single Domain (with path prefix):**
```bash
# Edit deploy/k8s/02-keycloak.yaml
# Add these environment variables:
env:
- name: KC_HTTP_RELATIVE_PATH
  value: "/auth"
- name: KC_HOSTNAME
  value: "kuberde.com"  # Your domain
- name: KC_PROXY
  value: "edge"
```

**For Subdomain:**
```bash
# Edit deploy/k8s/02-keycloak.yaml
env:
- name: KC_HOSTNAME
  value: "sso.kuberde.com"  # Your Keycloak subdomain
- name: KC_PROXY
  value: "edge"
```

### 5.2 Update Server Configuration

```bash
# Edit deploy/k8s/03-server.yaml
# Add environment variables in the server container:
env:
- name: KUBERDE_PUBLIC_URL
  value: "https://kuberde.com"
- name: KUBERDE_AGENT_DOMAIN
  value: "kuberde.com"
- name: KEYCLOAK_URL
  value: "https://kuberde.com/auth"  # or https://sso.kuberde.com
- name: KEYCLOAK_PUBLIC_URL
  value: "https://kuberde.com/auth"  # or https://sso.kuberde.com
- name: DATABASE_URL
  value: "postgres://kuberde:kuberde@postgresql:5432/kuberde?sslmode=disable"
```

### 5.3 Create Ingress Configuration

Choose the ingress configuration from `deploy/k8s/05-ingress-example.yaml`:

**For Single Domain:**
```bash
# Copy the "Option 1" section from 05-ingress-example.yaml
# Update the domain names to match yours
# Save as deploy/k8s/05-ingress.yaml
```

**For Subdomain:**
```bash
# Copy the "Option 2" section from 05-ingress-example.yaml
# Update the domain names to match yours
# Save as deploy/k8s/05-ingress.yaml
```

### 5.4 Update ClusterIssuer (if using cert-manager)

```bash
# Edit the ClusterIssuer in 05-ingress-example.yaml
# Change the email address:
spec:
  acme:
    email: your-email@example.com  # CHANGE THIS
```

## Step 6: Deploy KubeRDE

```bash
# Clone the repository
git clone https://github.com/your-org/kube-rde.git
cd kube-rde

# Deploy all components
make deploy

# Or deploy manually
kubectl create namespace kuberde
kubectl apply -f deploy/k8s/
```

## Step 7: Wait for Deployment

```bash
# Watch deployment status
kubectl get pods -n kuberde -w

# All pods should be Running
# Expected pods:
# - keycloak-xxx
# - kuberde-server-xxx
# - kuberde-operator-xxx
# - kuberde-web-xxx
# - postgresql-xxx
```

This may take 2-5 minutes depending on your cluster.

## Step 8: Verify Installation

```bash
# Check all pods are running
kubectl get pods -n kuberde

# Check ingress
kubectl get ingress -n kuberde

# Check certificates (if using cert-manager)
kubectl get certificate -n kuberde
```

## Step 9: Access KubeRDE

### 9.1 Access Web UI

Open your browser and navigate to:
- Single Domain: `https://kuberde.com`
- Subdomain: `https://kuberde.com`

### 9.2 Login

Default credentials (CHANGE IN PRODUCTION!):
- Username: `admin`
- Password: `admin`

### 9.3 Change Admin Password

1. Go to Keycloak admin console:
   - Single Domain: `https://kuberde.com/auth/admin`
   - Subdomain: `https://sso.kuberde.com/admin`
2. Login with admin/admin
3. Change the admin password
4. Create additional users

## Step 10: Create Your First Workspace

1. Login to KubeRDE Web UI
2. Navigate to "Workspaces"
3. Click "Create Workspace"
4. Fill in:
   - Name: `my-workspace`
   - Storage Size: `10Gi`
   - Storage Class: `standard` (or your cluster's default)
5. Click "Create"

## Step 11: Create a Development Service

1. Open your workspace
2. Click "Create Service"
3. Choose service type:
   - **SSH**: Terminal access
   - **Coder**: VS Code Server
   - **Jupyter**: JupyterLab
   - **File**: File browser
4. Configure resources:
   - CPU: 2 cores
   - Memory: 4 GiB
   - TTL: 8h (auto-shutdown after idle)
5. Click "Create"

## Step 12: Access Your Service

Once the service is running, you'll see an access URL:
- Format: `https://user-{username}-{service-name}.kuberde.com`
- Example: `https://user-admin-dev.kuberde.com`

Click the URL to access your development environment!

## Troubleshooting

### Pods Not Starting

```bash
# Check pod logs
kubectl logs -n kuberde <pod-name>

# Describe pod for events
kubectl describe pod -n kuberde <pod-name>
```

### Certificate Issues

```bash
# Check certificate status
kubectl describe certificate -n kuberde kuberde-tls

# Check cert-manager logs
kubectl logs -n cert-manager -l app=cert-manager
```

### DNS Not Resolving

```bash
# Verify DNS
dig kuberde.com
dig user-test.kuberde.com

# Check Ingress IP
kubectl get ingress -n kuberde
```

### Cannot Access Keycloak

**For Single Domain (path prefix):**
- URL should be: `https://kuberde.com/auth`
- Check if `KC_HTTP_RELATIVE_PATH=/auth` is set in Keycloak deployment
- Check Ingress path rewrite rules

**For Subdomain:**
- URL should be: `https://sso.kuberde.com`
- Check DNS for sso.kuberde.com
- Check Ingress rules for sso.kuberde.com

### Database Connection Errors

```bash
# Check PostgreSQL is running
kubectl get pods -n kuberde -l app=postgresql

# Check PostgreSQL logs
kubectl logs -n kuberde -l app=postgresql

# Test database connection
kubectl exec -it -n kuberde <server-pod> -- env | grep DATABASE_URL
```

## Next Steps

1. **Secure Your Deployment**
   - Change default admin password
   - Configure proper TLS certificates
   - Review security settings

2. **Configure Resource Quotas**
   - Navigate to "Resource Management"
   - Set default CPU/memory limits
   - Configure GPU types (if available)

3. **Create Users**
   - Add users in Keycloak
   - Assign quotas in KubeRDE
   - Set up SSH keys

4. **Monitor Your System**
   - Check "Audit Logs" for activity
   - Review "Admin Workspaces" for resource usage
   - Monitor agent status

## Common Configuration Patterns

### Development Environment
```yaml
CPU: 2 cores
Memory: 4 GiB
TTL: 8h
GPU: None
Service: Coder (VS Code Server)
```

### Data Science Environment
```yaml
CPU: 4 cores
Memory: 16 GiB
TTL: 24h
GPU: 1x nvidia-tesla-t4
Service: Jupyter
```

### CI/CD Build Environment
```yaml
CPU: 8 cores
Memory: 16 GiB
TTL: 1h
GPU: None
Service: SSH
Pinned: No (scales down after use)
```

## Getting Help

- Documentation: [docs/INDEX.md](INDEX.md)
- Troubleshooting: [docs/troubleshooting.md](troubleshooting.md)
- Issues: https://github.com/your-org/kube-rde/issues

## Production Deployment Checklist

Before going to production:

- [ ] Change all default passwords
- [ ] Configure proper TLS certificates
- [ ] Set up PostgreSQL with persistent storage
- [ ] Configure backup and disaster recovery
- [ ] Enable authentication with your IdP
- [ ] Set up monitoring and alerting
- [ ] Review security policies
- [ ] Configure resource quotas
- [ ] Test failover scenarios
- [ ] Document your configuration

Happy coding with KubeRDE! 🚀
