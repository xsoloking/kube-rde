# KubeRDE Terraform for AWS

## Status: 🚧 In Development

This Terraform configuration for AWS EKS is currently being developed.

## Current Options

### Option 1: Manual Deployment (Recommended)

Follow our comprehensive manual deployment guide:
- [AWS EKS Deployment Guide](../../docs/platforms/aws-eks.md)

This guide provides step-by-step instructions for:
- Creating an EKS cluster with eksctl
- Installing AWS Load Balancer Controller
- Configuring Route53 DNS
- Setting up ACM certificates
- Deploying KubeRDE

### Option 2: Contribute Terraform Modules

We welcome contributions! Help us build the AWS Terraform modules.

**What we need:**
- EKS cluster module
- VPC and networking configuration
- ALB Ingress Controller setup
- Route53 DNS configuration
- ACM certificate management
- KubeRDE deployment module
- Complete end-to-end example

See our [Contributing Guide](../../CONTRIBUTING.md) for how to get started.

## Planned Structure

```
aws/
├── eks-cluster/           # EKS cluster module
├── kuberde-deploy/        # KubeRDE deployment module
├── complete/              # End-to-end deployment
└── modules/
    ├── dns/               # Route53 configuration
    ├── cert-manager/      # ACM/cert-manager setup
    ├── ingress/           # ALB controller setup
    └── rds/               # RDS PostgreSQL (optional)
```

## Community Modules

Until official modules are ready, consider these community resources:
- [terraform-aws-eks](https://registry.terraform.io/modules/terraform-aws-modules/eks/aws/latest)
- [terraform-aws-vpc](https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws/latest)

## Quick Deploy with eksctl

While waiting for Terraform modules, you can quickly deploy with eksctl:

```bash
eksctl create cluster \
  --name kuberde-cluster \
  --region us-west-2 \
  --nodegroup-name standard-workers \
  --node-type t3.medium \
  --nodes 3 \
  --nodes-min 2 \
  --nodes-max 10 \
  --managed \
  --with-oidc

# Then deploy KubeRDE
kubectl apply -f ../../deploy/k8s/
```

See the [full guide](../../docs/platforms/aws-eks.md) for complete instructions.

## Stay Updated

Watch this repository for updates on AWS Terraform module availability:
- [GitHub Repository](https://github.com/xsoloking/kube-rde)
- [Roadmap](https://github.com/xsoloking/kube-rde/projects)
