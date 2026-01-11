# Local Development Quick Start

Get started with KubeRDE development in minutes using Docker Compose.

## Table of Contents

- [Quick Start](#quick-start)
- [Development Workflows](#development-workflows)
- [Environment Details](#environment-details)
- [Common Tasks](#common-tasks)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Prerequisites

- kind
- kubectl
- Go 1.24+ (for direct development)
- Node.js 18+ (for Web UI development)
- make (optional, for convenience)

### Workflow 1: Kubernetes Development

Deploy to local Kubernetes cluster:

```bash
# Create kind cluster
kind create cluster --name kuberde-dev

# Deploy using Helm with dev values
helm upgrade --install kuberde ./charts/kuberde \
  --namespace kuberde-dev \
  --create-namespace \
  -f ./charts/kuberde/values-dev.yaml

# Port-forward for local access
kubectl port-forward -n kuberde-dev svc/kuberde-server 8080:8080 &
kubectl port-forward -n kuberde-dev svc/kuberde-web 5173:80 &

# View logs
kubectl logs -n kuberde-dev -l app.kubernetes.io/name=kuberde-server -f
```

**Pros:**
- Test in real Kubernetes
- Test CRDs and Operator
- Production-like environment

**Cons:**
- Slower iteration
- More complex setup
- Need to rebuild images


## Next Steps

- [Full Development Guide](DEVELOPMENT.md)
- [Testing Guide](TESTING.md)
- [Contributing Guidelines](../../CONTRIBUTING.md)
- [Architecture Overview](../ARCHITECTURE.md)
