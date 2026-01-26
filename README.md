<div align="center">
  <h1>KubeRDE</h1>
  <p><strong>Kubernetes-native Remote Development Environments</strong></p>
  <p>Access your development workspace from anywhere with just a browser - no VPN required.</p>

  [![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
  [![Go Report Card](https://goreportcard.com/badge/github.com/xsoloking/kube-rde)](https://goreportcard.com/report/github.com/xsoloking/kube-rde)
  [![Kubernetes](https://img.shields.io/badge/kubernetes-1.28%2B-blue.svg)](https://kubernetes.io/)
  [![Go Version](https://img.shields.io/badge/go-1.24%2B-blue.svg)](https://golang.org/)
</div>

---

## 🚀 What is KubeRDE?

KubeRDE is a lightweight, production-ready platform for managing remote development environments on Kubernetes. It securely exposes development workspaces behind NAT/firewalls using WebSocket tunneling, enabling teams to access powerful development environments from anywhere.

**Perfect for:**
- 🏢 **Remote Teams** - Provide isolated development environments with centralized management
- 💻 **Organizations with Network Restrictions** - Access internal resources without VPN complexity
- 🔬 **Data Science Teams** - Share GPU resources efficiently with automatic idle shutdown
- 🎓 **Educational Institutions** - Offer pre-configured coding environments to students
- 🌐 **Distributed Development** - Standardize development environments across teams

## ✨ Key Features

- **🔐 Secure by Default**
  - OIDC/OAuth2 authentication with Keycloak
  - **Team-based Multi-tenancy**: Shared control plane with isolated team namespaces
  - JWT-based authorization for all connections
  - Audit logging for compliance

- **🌐 Access Anywhere**
  - Web-based access through browser - no client installation
  - WebSocket + Yamux multiplexing for efficient connections
  - Works behind NAT and corporate firewalls
  - Custom subdomain routing per workspace

- **🎯 Resource Control**
  - Fine-grained CPU, memory, and GPU quotas per user
  - Declarative resource limits via Kubernetes CRDs
  - Real-time resource usage tracking
  - Cost allocation and reporting

- **⚡ Auto-scaling Intelligence**
  - Automatic sleep/wake based on activity (TTL-based)
  - Scale-to-zero for cost optimization
  - Operator-managed lifecycle automation
  - Configurable idle timeouts per service

- **🛠️ Multiple IDEs**
  - VS Code Server (Coder)
  - JupyterLab for data science
  - SSH terminal access
  - Web-based file browser

- **📊 Full Observability**
  - Comprehensive audit logs for all operations
  - Resource usage metrics and dashboards
  - User activity tracking
  - Integration with Prometheus/Grafana

- **🎨 Self-Service UI**
  - React-based management console
  - Workspace and service management
  - User quota configuration
  - Template-based workspace creation
  - Agent status monitoring

## 🎛️ Advanced Features

### GPU Resource Management

KubeRDE provides flexible GPU resource management for AI/ML workloads:

- **Custom GPU Resource Types**: Configure GPU resources with custom Kubernetes resource names (e.g., `nvidia.com/gpu`, `amd.com/gpu`, `intel.com/gpu`)
- **Node Scheduling Labels**: Set node selector labels (e.g., `nvidia.com/gpu.product=NVIDIA-A100-80GB`) to schedule workloads on specific GPU types
- **Multi-GPU Support**: Define different GPU types for heterogeneous clusters (A100, H100, RTX 4090, etc.)
- **One-Click Quota Sync**: Users can click **SYNC** on their profile page to automatically inherit all resource types from the system's resource management configuration

**How it works:**
1. Administrators configure GPU resource types in **Resource Management** with resource name and node labels
2. Users click **SYNC** on their personal page to get all available resource types
3. User quotas can be customized after sync to set specific limits per GPU type
4. When creating services (agents), users can specify GPU resources to power AI/ML workloads

### GPU-Enabled Services for AI Development

Services (agents) in KubeRDE fully support GPU resource allocation for AI research and development:

- **Per-Service GPU Allocation**: Each service can request specific GPU resources based on user quota
- **AI/ML Workloads**: Run training jobs, inference servers, or GPU-accelerated Jupyter notebooks
- **Flexible Resource Requests**: Specify CPU, memory, storage, and GPU resources independently per service
- **Automatic Scheduling**: Kubernetes schedules GPU workloads to appropriate nodes based on configured labels

### Custom Agent Templates

Create reusable workspace templates with pre-configured settings for your team:

- **Template Import/Export**: Share templates across environments with JSON export/import
- **Pre-configured Services**: Define Docker images, ports, environment variables, and volume mounts
- **Security Context**: Configure UID/GID and other security settings per template
- **Multiple Agent Types**: Support for SSH, web-based IDEs, Jupyter notebooks, and custom services

**Example Template (Claude Code UI):**
```json
{
    "exported_at": "2026-01-03T00:01:37Z",
    "template": {
        "name": "claude-code-ui",
        "agent_type": "web",
        "description": "Claude Code UI",
        "docker_image": "soloking/claude-code-ui",
        "default_local_target": "localhost:3001",
        "default_external_port": 3001,
        "env_vars": {
            "PORT": 3001,
            "NODE_ENV": "production"
        },
        "security_context": {},
        "volume_mounts": [
            {
                "name": "workspace",
                "readOnly": false,
                "mountPath": "/home/claudeui"
            }
        ]
    },
    "version": "1.0"
}
```

**Template fields:**
- `agent_type`: Service type (`ssh`, `web`, `jupyter`, `coder`, `file-browser`)
- `docker_image`: Container image to use
- `default_local_target`: Internal service address
- `default_external_port`: Exposed port for external access
- `env_vars`: Environment variables passed to the container
- `volume_mounts`: Persistent storage configuration
- `security_context`: Container security settings (UID, GID, capabilities)

## 🎬 See It In Action

<!-- ![KubeRDE Demo](docs/images/demo.gif) -->

_Demo screenshots and videos coming soon!_

<!-- Uncomment when ready:
[📺 Watch 5-minute overview](https://youtube.com/...)
-->

## 🏃 Quick Start

### Try It Locally (5 Minutes)

The fastest way to try KubeRDE without any cloud setup:

```bash
# Using k3d (Kubernetes in Docker)
k3d create cluster kuberde

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

See [Local Kubernetes Guide](docs/platforms/local-k8s.md) for detailed instructions with kind, minikube, k3d, or Docker Desktop.

### Deploy to Production

Choose your platform for detailed deployment instructions:

<table>
  <tr>
    <td align="center"><strong>🔵 Google Cloud</strong></td>
    <td align="center"><strong>🟠 AWS</strong></td>
    <td align="center"><strong>🔷 Azure</strong></td>
    <td align="center"><strong>🐳 Your Cluster</strong></td>
  </tr>
  <tr>
    <td align="center"><a href="docs/platforms/gcp-gke.md">GKE Guide</a></td>
    <td align="center"><a href="docs/platforms/aws-eks.md">EKS Guide</a></td>
    <td align="center"><a href="docs/platforms/azure-aks.md">AKS Guide</a></td>
    <td align="center"><a href="docs/QUICK_START.md">Generic Guide</a></td>
  </tr>
</table>

**One-line Terraform deployment:**
```bash
# For GCP
cd terraform/gcp/complete && terraform init && terraform apply

# For AWS
cd terraform/aws/complete && terraform init && terraform apply

# For Azure
cd terraform/azure/complete && terraform init && terraform apply
```

## 🏗️ Architecture

```
┌─────────────────┐
│   Browser       │  ──HTTPS──▶  Ingress (TLS)
└─────────────────┘                    │
                                       ▼
                         ┌─────────────────────────┐
                         │  KubeRDE Server         │
                         │  - REST API             │
                         │  - WebSocket Relay      │
                         │  - OIDC Auth           │
                         └─────────────────────────┘
                                       │
                         ┌─────────────┴─────────────┐
                         ▼                           ▼
              ┌──────────────────┐       ┌──────────────────┐
              │  PostgreSQL      │       │  Keycloak        │
              │  (State)         │       │  (Auth)          │
              └──────────────────┘       └──────────────────┘
                                       │
                         ┌─────────────┴─────────────┐
                         ▼                           ▼
              ┌──────────────────┐       ┌──────────────────┐
              │  Operator        │       │  Agent Pods      │
              │  (Lifecycle)     │       │  - User Code     │
              │                  │       │  - VS Code/SSH   │
              └──────────────────┘       │  - Jupyter       │
                                         └──────────────────┘
```

**Key Components:**

- **Server**: Public relay with REST API, handles authentication and routes traffic
- **Agent**: Kubernetes pod that bridges traffic to internal services (SSH, Jupyter, VS Code)
- **Operator**: Custom Kubernetes operator managing agent lifecycle, PVCs, and TTL-based scaling
- **CLI**: User utility for OIDC authentication and SSH tunneling
- **Web UI**: React-based management console

[View detailed architecture →](docs/ARCHITECTURE.md)

## 📚 Documentation

### Getting Started
- 📖 [Quick Start Guide](docs/QUICK_START.md) - Get running in 15 minutes
- 🎓 [Concepts and Architecture](docs/ARCHITECTURE.md) - Understand how it works
- 🔧 [Configuration Reference](docs/CONFIGURATION.md) - All configuration options

### Platform Guides
- [Google Cloud (GKE)](docs/platforms/gcp-gke.md) - GCE Ingress, Cloud DNS, managed certificates
- [AWS (EKS)](docs/platforms/aws-eks.md) - ALB Controller, Route53, ACM certificates
- [Azure (AKS)](docs/platforms/azure-aks.md) - Application Gateway, Azure DNS
- [Local Kubernetes](docs/platforms/local-k8s.md) - kind, minikube, k3d, Docker Desktop

### Operations
- 🛠️ [Operations Runbook](docs/guides/OPERATORS_RUNBOOK.md) - Running in production
- 📊 [Monitoring and Alerts](docs/guides/OPERATORS_RUNBOOK.md#monitoring) - Observability setup
- 🔒 [Security Best Practices](SECURITY.md) - Hardening your deployment
- 💾 [Backup and Recovery](docs/guides/OPERATORS_RUNBOOK.md#backup) - Disaster recovery

### Development
- 💻 [Developer Guide](CLAUDE.md) - Build from source, contribute code
- 🧪 [Testing Guide](CONTRIBUTING.md#testing-guidelines) - Run tests, write tests
- 🏗️ [Building and Deploying](CLAUDE.md#building--running) - Local development workflow

### Reference
- 📝 [API Documentation](docs/API.md) - REST API reference (coming soon)
- ⚙️ [CRD Reference](deploy/k8s/01-crd.yaml) - RDEAgent custom resource
- 🔑 [Environment Variables](docs/CONFIGURATION.md#environment-variables) - All env vars

## 🤝 Contributing

We welcome contributions! KubeRDE is an open-source project and we'd love your help.

**Ways to contribute:**
- 🐛 [Report bugs](https://github.com/xsoloking/kube-rde/issues/new?template=bug_report.md)
- 💡 [Suggest features](https://github.com/xsoloking/kube-rde/issues/new?template=feature_request.md)
- 📝 Improve documentation
- 🔧 Submit pull requests
- ⭐ Star the project

**Getting Started:**
1. Read the [Contributing Guide](CONTRIBUTING.md)
2. Check out [good first issues](https://github.com/xsoloking/kube-rde/labels/good%20first%20issue)
3. Join our community discussions

Please read our [Code of Conduct](CODE_OF_CONDUCT.md) before contributing.

## 💬 Community

- 💬 **Discussions**: [GitHub Discussions](https://github.com/xsoloking/kube-rde/discussions) - Ask questions, share ideas
- 🐛 **Issues**: [Issue Tracker](https://github.com/xsoloking/kube-rde/issues) - Report bugs, request features
- 📧 **Email**: [maintainers@kuberde.io](mailto:maintainers@kuberde.io) - Contact maintainers
<!--
- 🗨️ **Discord**: [Join our Discord](https://discord.gg/...) - Real-time chat
- 🐦 **Twitter**: [@KubeRDE](https://twitter.com/KubeRDE) - Follow for updates
-->

## 🛣️ Roadmap

See our [GitHub Projects](https://github.com/xsoloking/kube-rde/projects) for upcoming features and current progress.

**Upcoming features:**
- Helm chart for easy installation
- Terraform modules for all major cloud providers
- Enhanced Web UI with resource usage graphs
- Multi-cluster support
- Plugin system for custom integrations

## 📄 License

KubeRDE is licensed under the [MIT License](LICENSE).

## 🙏 Acknowledgments

KubeRDE is built with amazing open source projects:

- [Kubernetes](https://kubernetes.io/) - Container orchestration
- [Keycloak](https://www.keycloak.org/) - Identity and access management
- [PostgreSQL](https://www.postgresql.org/) - Reliable database
- [React](https://react.dev/) - UI framework
- [Go](https://golang.org/) - Backend language
- [Yamux](https://github.com/hashicorp/yamux) - Connection multiplexing

Special thanks to all our [contributors](https://github.com/xsoloking/kube-rde/graphs/contributors)!

## 🔗 Related Projects

- [frp](https://github.com/fatedier/frp) - Fast reverse proxy (inspiration)
- [code-server](https://github.com/coder/code-server) - VS Code in the browser
- [JupyterHub](https://jupyter.org/hub) - Multi-user Jupyter environments
- [Telepresence](https://www.telepresence.io/) - Local Kubernetes development

---

<div align="center">
  <p>Made with ❤️ by the KubeRDE community</p>
  <p>
    <a href="https://github.com/xsoloking/kube-rde/stargazers">⭐ Star us on GitHub!</a>
  </p>
</div>
