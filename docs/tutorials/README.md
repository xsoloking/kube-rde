# KubeRDE Tutorials

Step-by-step tutorials for learning KubeRDE from basics to advanced usage.

## Tutorial Series

### Getting Started
1. [**Quick Start Guide**](01-quick-start.md) - Get KubeRDE running in 5 minutes
2. [**Your First Workspace**](02-first-workspace.md) - Create and access your first development environment
3. [**SSH Access**](03-ssh-access.md) - Connect to workspaces via SSH

### Workspace Management
4. [**Managing Services**](04-managing-services.md) - Add Jupyter, VS Code, and other services
5. [**Resource Configuration**](05-resource-configuration.md) - Configure CPU, memory, and storage
6. [**Workspace Templates**](06-workspace-templates.md) - Create and use custom templates

### Production Deployment
7. [**Cloud Deployment**](07-cloud-deployment.md) - Deploy KubeRDE to GCP/AWS/Azure
8. [**Domain and TLS Setup**](08-domain-tls-setup.md) - Configure custom domains and SSL certificates
9. [**User Management**](09-user-management.md) - Manage users, roles, and quotas

### Advanced Topics
10. [**Operator and CRDs**](10-operator-crds.md) - Use Kubernetes Operator for declarative management
11. [**Custom Agent Images**](11-custom-agent-images.md) - Build custom development environments
12. [**High Availability**](12-high-availability.md) - Setup HA production deployment
13. [**Monitoring and Observability**](13-monitoring-observability.md) - Monitor and troubleshoot KubeRDE
14. [**Security Hardening**](14-security-hardening.md) - Production security best practices

### Integration Guides
15. [**GitLab CI/CD Integration**](15-gitlab-integration.md) - Use KubeRDE in GitLab pipelines
16. [**GitHub Actions Integration**](16-github-actions-integration.md) - Integrate with GitHub workflows
17. [**VS Code Remote Development**](17-vscode-remote.md) - Configure VS Code Remote SSH
18. [**JupyterHub Integration**](18-jupyterhub-integration.md) - Multi-user Jupyter environments

## Tutorial Format

Each tutorial follows this structure:

```markdown
# Tutorial Title

**Time:** X minutes
**Difficulty:** Beginner/Intermediate/Advanced
**Prerequisites:** List of prerequisites

## What You'll Learn
- Learning objective 1
- Learning objective 2
- Learning objective 3

## Prerequisites
- Requirement 1
- Requirement 2

## Steps

### Step 1: Description
Detailed instructions...

### Step 2: Description
Detailed instructions...

## Verification
How to verify the tutorial worked...

## Troubleshooting
Common issues and solutions...

## Next Steps
- Link to related tutorial
- Link to relevant documentation

## Additional Resources
- External links
- Related documentation
```

## Learning Paths

### Path 1: Local Development
Perfect for getting started locally:
1. Quick Start Guide
2. Your First Workspace
3. SSH Access
4. Managing Services

### Path 2: Team Deployment
For teams deploying to cloud:
1. Quick Start Guide
2. Cloud Deployment
3. Domain and TLS Setup
4. User Management
5. Monitoring and Observability

### Path 3: Platform Engineering
For platform teams and operators:
1. Cloud Deployment
2. Operator and CRDs
3. Custom Agent Images
4. High Availability
5. Security Hardening
6. Monitoring and Observability

### Path 4: Data Science Teams
For data science and ML teams:
1. Quick Start Guide
2. Your First Workspace
3. Managing Services (focus on Jupyter)
4. Resource Configuration (GPU)
5. JupyterHub Integration

## Video Tutorials

Video versions of these tutorials are available:
- [Tutorial Playlist on YouTube](https://www.youtube.com/playlist?list=PLACEHOLDER)
- Individual video links are provided in each tutorial

## Sample Projects

Companion code for tutorials:
- [sample-projects/](./sample-projects/) - Example configurations and code

## Contributing Tutorials

Want to contribute a tutorial? See [CONTRIBUTING.md](../../CONTRIBUTING.md).

Tutorial contributions should:
- Follow the standard tutorial format
- Include working code examples
- Be tested on a clean environment
- Include screenshots or terminal output
- Provide troubleshooting tips

## Feedback

Found an issue in a tutorial?
- [Report an issue](https://github.com/xsoloking/kube-rde/issues/new?template=documentation.yml)
- Suggest improvements via pull request

## Quick Reference

| Tutorial | Time | Difficulty | Topics |
|----------|------|------------|--------|
| [Quick Start](01-quick-start.md) | 5 min | Beginner | Installation, First run |
| [First Workspace](02-first-workspace.md) | 10 min | Beginner | Workspace creation, Services |
| [SSH Access](03-ssh-access.md) | 10 min | Beginner | CLI setup, SSH keys, Connection |
| [Managing Services](04-managing-services.md) | 15 min | Beginner | Jupyter, VS Code, File browser |
| [Resource Config](05-resource-configuration.md) | 10 min | Intermediate | CPU, Memory, Storage, GPU |
| [Templates](06-workspace-templates.md) | 15 min | Intermediate | Custom templates, Import/Export |
| [Cloud Deployment](07-cloud-deployment.md) | 30 min | Intermediate | GKE, EKS, AKS deployment |
| [Domain/TLS](08-domain-tls-setup.md) | 20 min | Intermediate | DNS, Certificates, Ingress |
| [User Management](09-user-management.md) | 15 min | Intermediate | Users, Quotas, RBAC |
| [Operator/CRDs](10-operator-crds.md) | 20 min | Advanced | RDEAgent CRD, Declarative |
| [Custom Images](11-custom-agent-images.md) | 30 min | Advanced | Docker, Custom environments |
| [High Availability](12-high-availability.md) | 45 min | Advanced | HA setup, Redundancy |
| [Monitoring](13-monitoring-observability.md) | 30 min | Advanced | Prometheus, Grafana, Logs |
| [Security](14-security-hardening.md) | 30 min | Advanced | Security best practices |
| [GitLab CI/CD](15-gitlab-integration.md) | 20 min | Intermediate | GitLab pipelines |
| [GitHub Actions](16-github-actions-integration.md) | 20 min | Intermediate | GitHub workflows |
| [VS Code Remote](17-vscode-remote.md) | 15 min | Beginner | VS Code SSH setup |
| [JupyterHub](18-jupyterhub-integration.md) | 30 min | Advanced | Multi-user Jupyter |

## Additional Learning Resources

- [Documentation](../README.md) - Complete documentation
- [Architecture Guide](../ARCHITECTURE.md) - System architecture
- [API Reference](../API.md) - REST API documentation
- [FAQ](../FAQ.md) - Frequently asked questions
- [Examples](../../examples/) - Example configurations
