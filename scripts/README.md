# KubeRDE Installation Scripts

One-click installation scripts for deploying KubeRDE on various platforms.

## 🚀 Recommended: K3D or K3S + nip.io

**Zero-configuration deployment with automatic wildcard DNS!**

We recommend using **k3d** (k3s in Docker) or **k3s** (lightweight Kubernetes) with **nip.io** for the easiest local setup:

- ✅ **No DNS configuration required** - nip.io handles wildcard domains automatically
- ✅ **Multi-subdomain support** - Main domain, Keycloak subdomain, agent wildcards
- ✅ **Fast deployment** - Up and running in under 2 minutes
- ✅ **Easy cleanup** - Single command to delete everything

### Quick Start: K3D (Recommended for Development)

```bash
# Install k3d (if not already installed)
brew install k3d  # macOS
# or: curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

# Deploy KubeRDE
./scripts/quick-start-k3d.sh

# Access at: http://127-0-0-1.nip.io
```

**Perfect for:**
- Local development and testing
- CI/CD pipelines
- Quick experiments
- Multi-cluster setups

### Quick Start: K3S (Recommended for Long-term)

```bash
# Deploy KubeRDE (installs k3s if needed)
./scripts/quick-start-k3s.sh

# Access from any device on network: http://[your-ip].nip.io
```

**Perfect for:**
- Persistent dev environments
- Network-accessible testing
- Edge computing scenarios
- Production-like setup

**📖 See [K3D vs K3S Comparison](../docs/K3D-VS-K3S.md) for detailed comparison.**

---

## Alternative Options

### Universal Installer (Recommended)

The universal installer detects your environment and chooses the best installation method:

```bash
curl -sSL https://raw.githubusercontent.com/xsoloking/kube-rde/main/scripts/install.sh | bash
```

Or clone and run locally:

```bash
git clone https://github.com/xsoloking/kube-rde.git
cd kube-rde
./scripts/install.sh
```

## Platform-Specific Scripts

### Local Development with kind

Install KubeRDE on kind (Kubernetes in Docker):

```bash
./scripts/quick-start-kind.sh
```

**Requirements:**
- Docker
- kind
- kubectl

**What it does:**
1. Creates a kind cluster with 1 control-plane and 2 worker nodes
2. Installs NGINX Ingress Controller
3. **Interactive DNS setup** with multiple options (dnsmasq, CoreDNS, nip.io, /etc/hosts)
4. Deploys KubeRDE with proper domain configuration
5. Opens Web UI in browser

**Configuration:**
```bash
# Customize with environment variables
CLUSTER_NAME=my-cluster \
DOMAIN=my-app.local \
NAMESPACE=kuberde \
./scripts/quick-start-kind.sh

# Use nip.io for zero-config DNS
USE_NIP_IO=true ./scripts/quick-start-kind.sh
```

### Local Development with minikube

Install KubeRDE on minikube:

```bash
./scripts/quick-start-minikube.sh
```

**Requirements:**
- minikube
- kubectl

**What it does:**
1. Starts minikube with recommended resources
2. Enables Ingress and metrics-server addons
3. **Interactive DNS setup** with automatic IP detection
4. Deploys KubeRDE with proper domain configuration
5. Opens Web UI in browser

**Configuration:**
```bash
# Customize resources
PROFILE=kuberde \
CPUS=4 \
MEMORY=8192 \
DRIVER=docker \
./scripts/quick-start-minikube.sh

# Use nip.io for zero-config DNS
USE_NIP_IO=true ./scripts/quick-start-minikube.sh
```

## Script Reference

### quick-start-k3d.sh ⭐ RECOMMENDED

Zero-configuration deployment using k3d (k3s in Docker) + nip.io.

**Usage:**
```bash
./scripts/quick-start-k3d.sh
```

**What it does:**
1. Creates k3d cluster with Traefik ingress
2. Automatically configures nip.io domains (127-0-0-1.nip.io)
3. Deploys KubeRDE with multi-subdomain support
4. No DNS configuration needed!

**Options:**
| Variable | Description | Default |
|----------|-------------|---------|
| `CLUSTER_NAME` | k3d cluster name | `kuberde` |
| `NAMESPACE` | Kubernetes namespace | `kuberde` |
| `WORKERS` | Number of worker nodes | `2` |
| `K3S_VERSION` | k3s version | `v1.28.5-k3s1` |

**Example:**
```bash
# Create cluster with 3 workers
WORKERS=3 ./scripts/quick-start-k3d.sh

# Custom cluster name
CLUSTER_NAME=dev ./scripts/quick-start-k3d.sh
```

**Cleanup:**
```bash
# Stop cluster (preserves data)
k3d cluster stop kuberde

# Delete cluster
k3d cluster delete kuberde
```

**Why k3d?**
- 🐳 Runs in Docker (no sudo needed)
- ⚡ Fast creation (~30 seconds)
- 🔄 Multi-cluster support
- 🧹 Easy cleanup
- 💻 Works offline (127.0.0.1)

---

### quick-start-k3s.sh ⭐ RECOMMENDED

Production-grade deployment using k3s + nip.io with network access.

**Usage:**
```bash
./scripts/quick-start-k3s.sh
```

**What it does:**
1. Installs k3s if not present (requires sudo)
2. Detects host IP automatically
3. Configures nip.io domains ([host-ip].nip.io)
4. Deploys KubeRDE with multi-subdomain support
5. Accessible from any device on your network

**Options:**
| Variable | Description | Default |
|----------|-------------|---------|
| `NAMESPACE` | Kubernetes namespace | `kuberde` |
| `K3S_VERSION` | k3s version | `v1.28.5+k3s1` |
| `INSTALL_K3S` | Install k3s if needed | `true` |

**Example:**
```bash
# Use specific k3s version
K3S_VERSION=v1.27.0+k3s1 ./scripts/quick-start-k3s.sh

# Skip k3s installation (use existing)
INSTALL_K3S=false ./scripts/quick-start-k3s.sh
```

**Cleanup:**
```bash
# Uninstall k3s completely
sudo /usr/local/bin/k3s-uninstall.sh
```

**Why k3s?**
- 🌐 Network accessible (test from phone/tablet)
- ⚡ Production-grade lightweight k8s
- 🔒 Persistent across reboots
- 📦 Single binary (~70MB)
- 🎯 Perfect for edge/IoT

**📖 Compare with k3d:** [K3D vs K3S Guide](../docs/K3D-VS-K3S.md)

---

### setup-dns.sh

Interactive DNS configuration tool for wildcard domain support.

**Usage:**
```bash
./scripts/setup-dns.sh
```

**DNS Options:**
1. **dnsmasq** (Recommended) - Full wildcard support, best for development
2. **CoreDNS in Kubernetes** - Cluster-based DNS solution
3. **nip.io** - Zero-config external DNS service
4. **/etc/hosts** - Simple but no wildcard support
5. **Skip** - Manual configuration

**Environment Variables:**
| Variable | Description | Default |
|----------|-------------|---------|
| `DOMAIN` | Domain to configure | `kuberde.local` |
| `IP_ADDRESS` | IP to resolve to | `127.0.0.1` |

**Examples:**
```bash
# For kind (localhost)
IP_ADDRESS=127.0.0.1 DOMAIN=kuberde.local ./scripts/setup-dns.sh

# For minikube
IP_ADDRESS=$(minikube ip) DOMAIN=kuberde.local ./scripts/setup-dns.sh
```

**Why DNS Setup?**

KubeRDE requires **wildcard DNS** for agent subdomains:
- `kuberde.local` → Main service
- `user-alice-dev.kuberde.local` → Alice's agent
- `user-bob-jupyter.kuberde.local` → Bob's agent

Traditional `/etc/hosts` cannot handle `*.kuberde.local` patterns.

See [DNS Setup Guide](../docs/DNS-SETUP.md) for detailed information.

### install.sh

Universal installer that detects your environment and deploys accordingly.

**Usage:**
```bash
./scripts/install.sh
```

**Detects:**
- Cloud platform (GCP, AWS, Azure)
- Local Kubernetes (kind, minikube)
- Existing clusters (GKE, EKS, AKS)

**Behavior:**
- **Local**: Runs kind or minikube quick-start
- **Cloud**: Shows platform-specific guide
- **Generic**: Prompts for domain and deploys

### quick-start-kind.sh

Automated setup for kind clusters.

**Options:**
| Variable | Description | Default |
|----------|-------------|---------|
| `CLUSTER_NAME` | Kind cluster name | `kuberde` |
| `DOMAIN` | Domain for access | `kuberde.local` |
| `NAMESPACE` | Kubernetes namespace | `kuberde` |
| `USE_NIP_IO` | Use nip.io for DNS (no setup) | `false` |

**Example:**
```bash
DOMAIN=dev.local ./scripts/quick-start-kind.sh
```

**Cleanup:**
```bash
kind delete cluster --name kuberde
```

### quick-start-minikube.sh

Automated setup for minikube.

**Options:**
| Variable | Description | Default |
|----------|-------------|---------|
| `PROFILE` | Minikube profile | `kuberde` |
| `DOMAIN` | Domain for access | `kuberde.local` |
| `NAMESPACE` | Kubernetes namespace | `kuberde` |
| `CPUS` | CPU cores | `4` |
| `MEMORY` | Memory in MB | `8192` |
| `DRIVER` | Minikube driver | `docker` |
| `USE_NIP_IO` | Use nip.io for DNS (no setup) | `false` |

**Example:**
```bash
CPUS=8 MEMORY=16384 ./scripts/quick-start-minikube.sh
```

**Useful Commands:**
```bash
# Stop (preserves state)
minikube stop --profile kuberde

# Restart
minikube start --profile kuberde

# Dashboard
minikube dashboard --profile kuberde

# Cleanup
minikube delete --profile kuberde
```

## Deployment Methods

Each script supports two deployment methods (auto-detected):

### 1. Helm Chart (Preferred)

If Helm is installed and charts are available:
```bash
helm upgrade --install kuberde ./charts/kuberde \
  --namespace kuberde \
  --set global.domain=kuberde.local
```

### 2. Kubernetes Manifests (Fallback)

If Helm is not available:
```bash
kubectl apply -f deploy/k8s/ -n kuberde
```

## Health Checks

### test-health-checks.sh

Test health check endpoints for all KubeRDE services.

**Usage:**
```bash
./scripts/test-health-checks.sh
```

**What it tests:**
- kuberde-server: `/healthz`, `/livez`, `/readyz`
- kuberde-operator: `/healthz`, `/livez`, `/readyz`
- kuberde-web: `/healthz`, `/readyz`
- keycloak: `/health/live`, `/health/ready`

**Output:**
```
========================================
KubeRDE Health Check Test
========================================
Namespace: kuberde

----------------------------------------
Testing kuberde-server health checks
----------------------------------------
Pod: kuberde-server-5d7b8c9f4d-x8k2p
Testing kuberde-server /healthz... ✓ OK (HTTP 200)
Testing kuberde-server /livez... ✓ OK (HTTP 200)
Testing kuberde-server /readyz... ✓ OK (HTTP 200)

...
```

**Options:**
| Variable | Description | Default |
|----------|-------------|---------|
| `NAMESPACE` | Kubernetes namespace | `kuberde` |

**Example:**
```bash
# Test in custom namespace
NAMESPACE=kuberde-dev ./scripts/test-health-checks.sh
```

**Troubleshooting:**
```bash
# If health checks fail, check pod logs
kubectl logs -n kuberde -l app=kuberde-server

# Check pod events
kubectl describe pod -n kuberde <pod-name>

# Verify probes are configured
kubectl get deployment kuberde-server -n kuberde -o yaml | grep -A 10 "livenessProbe"
```

## Post-Installation

After running any script:

1. **Access KubeRDE:**
   - Web UI: http://kuberde.local (or your domain)
   - Keycloak: http://kuberde.local/auth/admin

2. **Login:**
   - Username: `admin`
   - Password: `admin`

3. **⚠️ Change Password:**
   - Go to Keycloak admin console
   - Navigate to Users > admin > Credentials
   - Set a strong password

4. **Create Workspace:**
   - Login to Web UI
   - Click "Create Workspace"
   - Configure and launch services

## Troubleshooting

### Scripts Won't Execute

```bash
# Make scripts executable
chmod +x scripts/*.sh
```

### Port Already in Use (kind)

```bash
# Check what's using port 80/443
sudo lsof -i :80
sudo lsof -i :443

# Stop conflicting services or use different ports
```

### /etc/hosts Permission Denied

Scripts need sudo to modify `/etc/hosts`. You'll be prompted for your password.

### Pods Not Starting

```bash
# Check pod status
kubectl get pods -n kuberde

# View logs
kubectl logs -n kuberde -l app.kubernetes.io/name=kuberde-server

# Describe pod for events
kubectl describe pod -n kuberde <pod-name>
```

### Domain Not Resolving

```bash
# Test DNS resolution
ping kuberde.local
ping test.kuberde.local  # Should work if wildcards configured

# If using dnsmasq (macOS)
brew services list | grep dnsmasq
cat /etc/resolver/kuberde.local

# If using dnsmasq (Linux)
sudo systemctl status dnsmasq
cat /etc/dnsmasq.d/kuberde

# If using /etc/hosts (not recommended)
cat /etc/hosts | grep kuberde

# For minikube, check IP hasn't changed
minikube ip --profile kuberde

# Re-run DNS setup if needed
./scripts/setup-dns.sh

# Or use nip.io to avoid DNS issues
USE_NIP_IO=true ./scripts/quick-start-kind.sh
```

### Wildcard Domains Not Working

If `kuberde.local` works but `test.kuberde.local` doesn't:

```bash
# /etc/hosts doesn't support wildcards - use dnsmasq or nip.io
./scripts/setup-dns.sh  # Choose option 1 (dnsmasq) or 3 (nip.io)
```

### Ingress Not Working

```bash
# Check Ingress controller
kubectl get pods -n ingress-nginx

# Check Ingress resource
kubectl get ingress -n kuberde
kubectl describe ingress -n kuberde
```

## Advanced Usage

### Custom Configuration

Create a custom values file for Helm:

```yaml
# my-values.yaml
global:
  domain: "my-domain.local"

server:
  replicaCount: 2
  resources:
    limits:
      cpu: 2000m
      memory: 2Gi

postgresql:
  persistence:
    size: 20Gi
```

Then use it:

```bash
# Modify script to use custom values
helm upgrade --install kuberde ./charts/kuberde \
  --namespace kuberde \
  -f my-values.yaml
```

### CI/CD Integration

Use these scripts in CI/CD pipelines:

```yaml
# Example GitHub Actions
- name: Install KubeRDE
  run: |
    kind create cluster
    ./scripts/quick-start-kind.sh
    kubectl wait --for=condition=ready pod --all -n kuberde --timeout=300s
```

### Multiple Instances

Run multiple KubeRDE instances:

```bash
# Instance 1
CLUSTER_NAME=kuberde-dev \
DOMAIN=dev.local \
NAMESPACE=kuberde-dev \
./scripts/quick-start-kind.sh

# Instance 2
CLUSTER_NAME=kuberde-staging \
DOMAIN=staging.local \
NAMESPACE=kuberde-staging \
./scripts/quick-start-kind.sh
```

## Contributing

To add a new installation script:

1. Follow existing script patterns
2. Include error handling
3. Add colorful output
4. Support environment variables
5. Document in this README
6. Test on clean environment

## Support

- **Documentation**: [docs/](../docs/)
- **Platform Guides**: [docs/platforms/](../docs/platforms/)
- **Issues**: [GitHub Issues](https://github.com/xsoloking/kube-rde/issues)
- **Discussions**: [GitHub Discussions](https://github.com/xsoloking/kube-rde/discussions)
