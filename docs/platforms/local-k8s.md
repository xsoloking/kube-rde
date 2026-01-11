# Deploying KubeRDE on Local Kubernetes

This guide walks you through deploying KubeRDE on local Kubernetes clusters for development, testing, and learning purposes.

## Local Kubernetes Options

- **kind** (Kubernetes in Docker) - Recommended for CI/CD and testing
- **minikube** - Good for local development with driver options
- **k3s/k3d** - Lightweight, fast, minimal resource usage
- **Docker Desktop** - Easy setup on Mac/Windows with GUI
- **MicroK8s** - Ubuntu's lightweight Kubernetes

## Prerequisites

- Docker installed
- 8GB+ RAM recommended
- 20GB+ free disk space
- `kubectl` installed

## Option 1: kind (Recommended)

### Install kind

```bash
# macOS
brew install k3d

# Linux
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
```

### Create Cluster

```bash
# Create cluster
k3d create cluster kuberde

# Verify cluster
kubectl cluster-info --context k3d-kuberde
kubectl get nodes -o wide
```

### Deploy KubeRDE

```bash
# Clone repository
git clone https://github.com/xsoloking/kube-rde.git
cd kube-rde

# Deploy with helm
helm install kuberde charts/kuberde -f values-http.yaml --namespace kuberde --create-namespace

# check k3d cluster load balancer ip
kubectl get svc -A -o wide | grep traefik

# Deploy KubeRDE ( assume the ip address is 192.168.107.2)
helm install kuberde charts/kuberde -f values-http.yaml --namespace kuberde --create-namespace \
  --set global.domain=192.168.107.2.nip.io \
  --set global.keycloakDomain=sso.192.168.107.2.nip.io \
  --set global.agentDomain=*.192.168.107.2.nip.io \
  --set global.protocol=http 

# Access at http://192.168.107.2.nip.io
# Login: admin / password
```

### Clean Up

```bash
# Delete cluster
k3d delete cluster kuberde
```

## Troubleshooting Tips

```bash
# Check all resources
kubectl get all -n kuberde

# Check events
kubectl get events -n kuberde --sort-by='.lastTimestamp'

# Describe problematic pods
kubectl describe pod <pod-name> -n kuberde

# View logs
kubectl logs -n kuberde deployment/kuberde-server --tail=100

# Check resource usage
kubectl top nodes
kubectl top pods -n kuberde

# Debug networking
kubectl run debug --image=nicolaka/netshoot -it --rm
```

## Additional Resources

- [k3d Documentation](https://k3d.io/)
- [KubeRDE Development Guide](../CLAUDE.md)
