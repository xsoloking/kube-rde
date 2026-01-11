# KubeRDE Terraform for Azure

## Status: 🚧 In Development

This Terraform configuration for Azure AKS is currently being developed.

## Current Options

### Option 1: Manual Deployment (Recommended)

Follow our comprehensive manual deployment guide:
- [Azure AKS Deployment Guide](../../docs/platforms/azure-aks.md)

This guide provides step-by-step instructions for:
- Creating an AKS cluster
- Installing Application Gateway Ingress Controller or NGINX
- Configuring Azure DNS
- Setting up certificates with cert-manager
- Deploying KubeRDE

### Option 2: Contribute Terraform Modules

We welcome contributions! Help us build the Azure Terraform modules.

**What we need:**
- AKS cluster module
- Virtual network and subnet configuration
- Application Gateway Ingress Controller setup
- Azure DNS configuration
- Certificate management
- KubeRDE deployment module
- Complete end-to-end example

See our [Contributing Guide](../../CONTRIBUTING.md) for how to get started.

## Planned Structure

```
azure/
├── aks-cluster/           # AKS cluster module
├── kuberde-deploy/        # KubeRDE deployment module
├── complete/              # End-to-end deployment
└── modules/
    ├── dns/               # Azure DNS configuration
    ├── cert-manager/      # Certificate management
    ├── ingress/           # Ingress controller setup
    └── database/          # Azure Database for PostgreSQL (optional)
```

## Community Modules

Until official modules are ready, consider these community resources:
- [terraform-azurerm-aks](https://registry.terraform.io/modules/Azure/aks/azurerm/latest)
- [terraform-azurerm-vnet](https://registry.terraform.io/modules/Azure/vnet/azurerm/latest)

## Quick Deploy with az CLI

While waiting for Terraform modules, you can quickly deploy with Azure CLI:

```bash
# Create resource group
az group create --name kuberde-rg --location eastus

# Create AKS cluster
az aks create \
  --resource-group kuberde-rg \
  --name kuberde-cluster \
  --node-count 3 \
  --node-vm-size Standard_D2s_v3 \
  --enable-managed-identity \
  --enable-cluster-autoscaler \
  --min-count 2 \
  --max-count 10 \
  --generate-ssh-keys

# Get credentials
az aks get-credentials --resource-group kuberde-rg --name kuberde-cluster

# Deploy KubeRDE
kubectl apply -f ../../deploy/k8s/
```

See the [full guide](../../docs/platforms/azure-aks.md) for complete instructions.

## Stay Updated

Watch this repository for updates on Azure Terraform module availability:
- [GitHub Repository](https://github.com/xsoloking/kube-rde)
- [Roadmap](https://github.com/xsoloking/kube-rde/projects)
