# KubeRDE Terraform Modules

This directory contains Terraform modules for deploying KubeRDE on various cloud platforms.

## Available Platforms

### Google Cloud Platform (GCP)
- **Status**: ✅ Complete
- **Location**: [`gcp/`](gcp/)
- **Guide**: [GCP Deployment Guide](../docs/platforms/gcp-gke.md)

```bash
cd gcp/complete
terraform init
terraform apply
```

### Amazon Web Services (AWS)
- **Status**: 🚧 In Development
- **Location**: [`aws/`](aws/)
- **Guide**: [AWS Deployment Guide](../docs/platforms/aws-eks.md)

Coming soon! Follow the manual deployment guide for now.

### Microsoft Azure
- **Status**: 🚧 In Development
- **Location**: [`azure/`](azure/)
- **Guide**: [Azure Deployment Guide](../docs/platforms/azure-aks.md)

Coming soon! Follow the manual deployment guide for now.

## Module Structure

Each cloud provider has the following structure:

```
<provider>/
├── <cluster>/           # Kubernetes cluster module
├── kuberde-deploy/      # KubeRDE deployment module
├── complete/            # End-to-end deployment
└── modules/             # Shared modules
    ├── dns/             # DNS configuration
    ├── cert-manager/    # Certificate management
    └── ingress/         # Ingress setup
```

## Quick Start

### 1. Choose Your Platform

Navigate to your cloud provider's directory:
- GCP: `cd gcp/complete`
- AWS: `cd aws/complete` (coming soon)
- Azure: `cd azure/complete` (coming soon)

### 2. Configure Variables

Copy the example variables file:
```bash
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` with your configuration:
```hcl
# Required variables
project_id = "your-project-id"
domain     = "kuberde.example.com"
email      = "your-email@example.com"

# Optional: Enable GPU support
enable_gpu_pool = true
```

### 3. Deploy

```bash
# Initialize Terraform
terraform init

# Preview changes
terraform plan

# Apply configuration
terraform apply
```

### 4. Access KubeRDE

After deployment completes:

```bash
# Get cluster credentials
<output will show kubectl config command>

# Access KubeRDE
# Web UI: https://kuberde.example.com
# Keycloak: https://kuberde.example.com/auth/admin
# Default credentials: admin / admin (CHANGE IMMEDIATELY!)
```

## Common Variables

All platforms support these common variables:

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `cluster_name` | Kubernetes cluster name | kuberde-cluster | No |
| `domain` | Domain name for KubeRDE | - | Yes |
| `email` | Email for certificates | - | Yes |
| `min_node_count` | Minimum nodes | 2 | No |
| `max_node_count` | Maximum nodes | 10 | No |
| `enable_gpu_pool` | Enable GPU nodes | false | No |

See platform-specific documentation for additional variables.

## Cost Estimation

### GCP (us-central1)
- Cluster management: ~$70/month
- 3 x n1-standard-2 nodes: ~$150/month
- Load Balancer: ~$18/month
- Storage (100GB): ~$17/month
- **Total: ~$255/month**

### AWS (us-east-1)
- EKS cluster: ~$73/month
- 3 x t3.medium nodes: ~$100/month
- ALB: ~$23/month
- Storage (100GB EBS): ~$10/month
- **Total: ~$206/month**

### Azure (East US)
- AKS cluster: Free
- 3 x Standard_D2s_v3 nodes: ~$140/month
- Load Balancer: ~$20/month
- Storage (100GB): ~$10/month
- **Total: ~$170/month**

GPU costs additional ~$300-500/month per node.

## Troubleshooting

### Terraform State Issues

If you encounter state lock issues:
```bash
# For GCS backend
terraform force-unlock <LOCK_ID>
```

### Provider Authentication

**GCP:**
```bash
gcloud auth application-default login
```

**AWS:**
```bash
aws configure
```

**Azure:**
```bash
az login
```

### Resource Quotas

Check your cloud provider quotas before deployment:
- **GCP**: https://console.cloud.google.com/iam-admin/quotas
- **AWS**: https://console.aws.amazon.com/servicequotas
- **Azure**: https://portal.azure.com/#blade/Microsoft_Azure_Capacity/QuotaMenuBlade

## Advanced Usage

### Using Remote State

**GCS (GCP):**
```hcl
terraform {
  backend "gcs" {
    bucket = "your-terraform-state-bucket"
    prefix = "kuberde"
  }
}
```

**S3 (AWS):**
```hcl
terraform {
  backend "s3" {
    bucket = "your-terraform-state-bucket"
    key    = "kuberde/terraform.tfstate"
    region = "us-east-1"
  }
}
```

**Azure Storage:**
```hcl
terraform {
  backend "azurerm" {
    storage_account_name = "yourstorageaccount"
    container_name       = "tfstate"
    key                  = "kuberde.terraform.tfstate"
  }
}
```

### Importing Existing Resources

If you have existing infrastructure:

```bash
# Import existing cluster
terraform import module.cluster.<resource_type>.<resource_name> <resource_id>
```

### Customizing Deployments

Each module can be used independently. For example, to only deploy KubeRDE to an existing cluster:

```hcl
module "kuberde" {
  source = "./kuberde-deploy"

  cluster_name = "existing-cluster"
  domain       = "kuberde.example.com"
  # ... other variables
}
```

## Security Best Practices

1. **Use remote state** with encryption
2. **Enable state locking** to prevent concurrent modifications
3. **Store secrets in** secret managers (not in tfvars files)
4. **Use workspaces** for different environments
5. **Enable audit logging** for Terraform operations
6. **Review plan output** before applying changes

### Example: Using Secret Manager for Sensitive Values

**GCP:**
```hcl
data "google_secret_manager_secret_version" "db_password" {
  secret = "kuberde-db-password"
}

locals {
  db_password = data.google_secret_manager_secret_version.db_password.secret_data
}
```

## Cleanup

To destroy all resources:

```bash
terraform destroy
```

**⚠️ Warning**: This will delete:
- The entire Kubernetes cluster
- All workloads and data
- Load balancers and networking
- DNS records
- Everything created by Terraform

**Always backup** your data before destroying resources!

## Contributing

We welcome contributions to improve these Terraform modules!

Areas for contribution:
- AWS and Azure complete implementations
- Additional modules (monitoring, backup, etc.)
- Cost optimization configurations
- Multi-region deployments
- Disaster recovery modules

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

## Support

- **Documentation**: See platform-specific guides in [`docs/platforms/`](../docs/platforms/)
- **Issues**: [GitHub Issues](https://github.com/xsoloking/kube-rde/issues)
- **Discussions**: [GitHub Discussions](https://github.com/xsoloking/kube-rde/discussions)

## License

These Terraform modules are part of KubeRDE and are licensed under the [MIT License](../LICENSE).
