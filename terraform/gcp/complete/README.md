# KubeRDE Complete Deployment on GCP

This Terraform configuration deploys a complete KubeRDE environment on Google Cloud Platform, including:

- GKE cluster with autoscaling
- Cloud DNS configuration
- NGINX Ingress Controller
- cert-manager for TLS certificates
- KubeRDE components (Server, Operator, Web UI, PostgreSQL, Keycloak)

## Prerequisites

1. GCP account with billing enabled
2. `gcloud` CLI installed and configured
3. Terraform >= 1.5 installed
4. A domain name for KubeRDE

## Quick Start

```bash
# Clone the repository
git clone https://github.com/xsoloking/kube-rde.git
cd kube-rde/terraform/gcp/complete

# Copy and edit the example variables
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values

# Initialize Terraform
terraform init

# Review the plan
terraform plan

# Apply the configuration
terraform apply
```

## Configuration

Create a `terraform.tfvars` file with your configuration:

```hcl
project_id      = "your-gcp-project-id"
region          = "us-central1"
cluster_name    = "kuberde-cluster"
domain          = "kuberde.example.com"
email           = "your-email@example.com"

# Optional: Enable GPU support
enable_gpu_pool = true
gpu_type        = "nvidia-tesla-t4"

# Optional: Customize node configuration
machine_type    = "n1-standard-4"
min_node_count  = 2
max_node_count  = 10
```

## Variables

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|----------|
| project_id | GCP project ID | string | - | yes |
| region | GCP region | string | us-central1 | no |
| cluster_name | GKE cluster name | string | kuberde-cluster | no |
| domain | Domain name for KubeRDE | string | - | yes |
| email | Email for Let's Encrypt certificates | string | - | yes |
| enable_gpu_pool | Enable GPU node pool | bool | false | no |

See [variables.tf](variables.tf) for all available variables.

## Outputs

| Name | Description |
|------|-------------|
| cluster_name | GKE cluster name |
| cluster_endpoint | GKE cluster endpoint |
| kuberde_url | KubeRDE Web UI URL |
| keycloak_url | Keycloak admin console URL |

## Post-Deployment

After successful deployment:

1. **Configure kubectl:**
   ```bash
   gcloud container clusters get-credentials kuberde-cluster --region=us-central1
   ```

2. **Access KubeRDE:**
   - Web UI: https://kuberde.example.com
   - Keycloak: https://kuberde.example.com/auth/admin
   - Default credentials: admin / admin (CHANGE IMMEDIATELY)

3. **Verify deployment:**
   ```bash
   kubectl get pods -n kuberde
   kubectl get ingress -n kuberde
   ```

4. **DNS propagation:**
   - Wait 5-15 minutes for DNS to propagate
   - Verify: `nslookup kuberde.example.com`

## Cost Estimation

Typical monthly costs (us-central1):

- GKE Cluster: ~$70/month (cluster management fee)
- 3 x n1-standard-2 nodes: ~$150/month
- Load Balancer: ~$18/month
- Persistent Storage (100GB): ~$17/month
- **Total: ~$255/month**

GPU nodes (if enabled):
- 1 x n1-standard-4 + T4 GPU: ~$350/month additional

## Cleanup

To destroy all resources:

```bash
terraform destroy
```

**Warning:** This will delete all data including databases. Back up any important data first!

## Architecture

This deployment creates:

```
Internet
    │
    ▼
Cloud Load Balancer (managed by NGINX Ingress)
    │
    ▼
GKE Cluster
    ├── kuberde namespace
    │   ├── Server (Deployment + Service)
    │   ├── Operator (Deployment)
    │   ├── Web UI (Deployment + Service)
    │   ├── PostgreSQL (StatefulSet + PVC)
    │   └── Keycloak (Deployment + Service)
    ├── ingress-nginx namespace
    │   └── NGINX Ingress Controller
    └── cert-manager namespace
        └── cert-manager
```

## Troubleshooting

### Pods not starting
```bash
kubectl describe pod <pod-name> -n kuberde
kubectl logs <pod-name> -n kuberde
```

### Certificate not issued
```bash
kubectl get certificate -n kuberde
kubectl describe certificate kuberde-tls -n kuberde
kubectl logs -n cert-manager deployment/cert-manager
```

### Cannot access KubeRDE
1. Check DNS: `nslookup kuberde.example.com`
2. Check Ingress: `kubectl get ingress -n kuberde`
3. Check LoadBalancer IP: `kubectl get svc -n ingress-nginx`

## Security Best Practices

After deployment:

1. **Change default passwords** in Keycloak
2. **Enable binary authorization:** Set `enable_binary_authorization = true`
3. **Restrict master access:** Configure `master_authorized_networks`
4. **Enable Cloud Armor:** Add WAF rules to the load balancer
5. **Review IAM permissions:** Follow principle of least privilege

## Advanced Configuration

### Using an existing VPC
```hcl
network    = "projects/your-project/global/networks/your-vpc"
subnetwork = "projects/your-project/regions/us-central1/subnetworks/your-subnet"
```

### Enabling Workload Identity
```hcl
# Already enabled by default in this module
```

### Custom node pools
See [../gke-cluster/README.md](../gke-cluster/README.md) for advanced node pool configuration.

## Support

- Documentation: [docs/platforms/gcp-gke.md](../../../docs/platforms/gcp-gke.md)
- Issues: https://github.com/xsoloking/kube-rde/issues
- Community: https://github.com/xsoloking/kube-rde/discussions
