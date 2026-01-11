# Deploying KubeRDE on Microsoft Azure (AKS)

This guide walks you through deploying KubeRDE on Azure Kubernetes Service (AKS).

## Prerequisites

- Azure account with active subscription
- `az` CLI installed and configured
- `kubectl` installed
- Domain name with DNS access (for production deployment)
- Basic knowledge of Azure and Kubernetes

## Architecture Overview

```
Internet
    ↓
Azure Application Gateway or Load Balancer
    ↓
AKS Cluster
    ├── KubeRDE Server (WebSocket + API)
    ├── KubeRDE Operator
    ├── PostgreSQL Database (or Azure Database for PostgreSQL)
    ├── Keycloak (OIDC Provider)
    ├── Web UI
    └── Agent Pods (per workspace)
```

## Step 1: Create Resource Group

```bash
# Set variables
export RESOURCE_GROUP="kuberde-rg"
export LOCATION="eastus"
export CLUSTER_NAME="kuberde-cluster"

# Login to Azure
az login

# Create resource group
az group create \
  --name $RESOURCE_GROUP \
  --location $LOCATION
```

## Step 2: Create AKS Cluster

### Option A: Standard AKS Cluster (Recommended)

```bash
# Create AKS cluster
az aks create \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME \
  --node-count 3 \
  --node-vm-size Standard_D2s_v3 \
  --enable-managed-identity \
  --enable-addons monitoring \
  --network-plugin azure \
  --enable-cluster-autoscaler \
  --min-count 2 \
  --max-count 10 \
  --kubernetes-version 1.28.0 \
  --generate-ssh-keys

# Get credentials
az aks get-credentials \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME

# Verify cluster
kubectl get nodes
```

### Option B: AKS with Azure CNI

```bash
# Create virtual network
az network vnet create \
  --resource-group $RESOURCE_GROUP \
  --name kuberde-vnet \
  --address-prefix 10.0.0.0/8 \
  --subnet-name kuberde-subnet \
  --subnet-prefix 10.240.0.0/16

# Get subnet ID
SUBNET_ID=$(az network vnet subnet show \
  --resource-group $RESOURCE_GROUP \
  --vnet-name kuberde-vnet \
  --name kuberde-subnet \
  --query id -o tsv)

# Create AKS with Azure CNI
az aks create \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME \
  --node-count 3 \
  --node-vm-size Standard_D2s_v3 \
  --network-plugin azure \
  --vnet-subnet-id $SUBNET_ID \
  --service-cidr 10.0.0.0/16 \
  --dns-service-ip 10.0.0.10 \
  --enable-managed-identity \
  --enable-cluster-autoscaler \
  --min-count 2 \
  --max-count 10 \
  --generate-ssh-keys
```

### Option C: AKS with GPU Support (for Data Science Workloads)

```bash
# Create AKS cluster
az aks create \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME \
  --node-count 2 \
  --node-vm-size Standard_D2s_v3 \
  --generate-ssh-keys

# Add GPU node pool
az aks nodepool add \
  --resource-group $RESOURCE_GROUP \
  --cluster-name $CLUSTER_NAME \
  --name gpupool \
  --node-count 1 \
  --node-vm-size Standard_NC6s_v3 \
  --min-count 0 \
  --max-count 5 \
  --enable-cluster-autoscaler

# Install NVIDIA device plugin
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml
```

## Step 3: Install Ingress Controller

You have two main options for Ingress on AKS:

### Option A: Application Gateway Ingress Controller (AGIC)

**Benefits**:
- Fully managed Azure service
- WAF integration
- Auto-scaling
- SSL termination

#### 3.1: Enable AGIC Add-on

```bash
# Create Application Gateway
az network public-ip create \
  --resource-group $RESOURCE_GROUP \
  --name kuberde-agw-pip \
  --allocation-method Static \
  --sku Standard

# Create Application Gateway
az network application-gateway create \
  --name kuberde-appgateway \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION \
  --sku Standard_v2 \
  --public-ip-address kuberde-agw-pip \
  --vnet-name kuberde-vnet \
  --subnet agw-subnet \
  --capacity 2

# Enable AGIC
az aks enable-addons \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME \
  --addons ingress-appgw \
  --appgw-id $(az network application-gateway show \
    --resource-group $RESOURCE_GROUP \
    --name kuberde-appgateway \
    --query id -o tsv)
```

### Option B: NGINX Ingress Controller (More Flexible)

**Benefits**:
- Open source and widely used
- More configuration options
- Better WebSocket support
- Kubernetes standard

```bash
# Add NGINX Helm repository
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

# Install NGINX Ingress Controller
helm install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace \
  --set controller.service.annotations."service\.beta\.kubernetes\.io/azure-load-balancer-health-probe-request-path"=/healthz

# Wait for external IP
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s

# Get external IP
kubectl get service -n ingress-nginx ingress-nginx-controller
```

## Step 4: Configure Azure DNS

### Option A: Create DNS Zone in Azure

```bash
# Create DNS zone
az network dns zone create \
  --resource-group $RESOURCE_GROUP \
  --name kuberde.example.com

# Get name servers
az network dns zone show \
  --resource-group $RESOURCE_GROUP \
  --name kuberde.example.com \
  --query nameServers

# Update your domain registrar with these name servers
```

### Option B: Use External DNS Provider

If using external DNS, you'll configure it after getting the Load Balancer IP.

## Step 5: Deploy KubeRDE Core Components

```bash
# Clone the repository
git clone https://github.com/xsoloking/kube-rde.git
cd kube-rde

# Set your domain
export KUBERDE_DOMAIN="kuberde.example.com"

# Deploy all components
make deploy

# Or deploy step by step
make deploy-namespace
make deploy-crd
make deploy-postgresql
make deploy-keycloak
make deploy-server
make deploy-operator
make deploy-web
```

## Step 6: Configure TLS Certificates

### Option A: Using cert-manager with Let's Encrypt

```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Wait for cert-manager to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/instance=cert-manager \
  -n cert-manager \
  --timeout=120s

# Create ClusterIssuer for Let's Encrypt
cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
EOF
```

### Option B: Using Azure Key Vault for Certificates

```bash
# Create Key Vault
az keyvault create \
  --name kuberde-kv \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION

# Import certificate
az keyvault certificate import \
  --vault-name kuberde-kv \
  --name kuberde-cert \
  --file /path/to/certificate.pfx

# Install CSI Secret Store Driver
helm repo add csi-secrets-store-provider-azure https://azure.github.io/secrets-store-csi-driver-provider-azure/charts
helm install csi csi-secrets-store-provider-azure/csi-secrets-store-provider-azure
```

## Step 7: Configure Ingress

### Option A: Using NGINX Ingress with cert-manager

```bash
# Get Load Balancer IP
export LB_IP=$(kubectl get service -n ingress-nginx ingress-nginx-controller \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

echo "Load Balancer IP: $LB_IP"

# Create Ingress
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kuberde-ingress
  namespace: kuberde
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    nginx.ingress.kubernetes.io/websocket-services: "kuberde-server"
spec:
  tls:
  - hosts:
    - kuberde.example.com
    - "*.kuberde.example.com"
    secretName: kuberde-tls
  rules:
  - host: kuberde.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: kuberde-web
            port:
              number: 80
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: kuberde-server
            port:
              number: 8080
      - path: /ws
        pathType: Prefix
        backend:
          service:
            name: kuberde-server
            port:
              number: 8080
  - host: "*.kuberde.example.com"
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: kuberde-server
            port:
              number: 8080
EOF
```

### Option B: Using Application Gateway Ingress Controller

```bash
# Get Application Gateway Public IP
export AGW_IP=$(az network public-ip show \
  --resource-group $RESOURCE_GROUP \
  --name kuberde-agw-pip \
  --query ipAddress -o tsv)

echo "Application Gateway IP: $AGW_IP"

# Create Ingress
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kuberde-ingress
  namespace: kuberde
  annotations:
    kubernetes.io/ingress.class: azure/application-gateway
    appgw.ingress.kubernetes.io/ssl-redirect: "true"
    appgw.ingress.kubernetes.io/backend-protocol: "http"
    appgw.ingress.kubernetes.io/connection-draining: "true"
    appgw.ingress.kubernetes.io/connection-draining-timeout: "30"
spec:
  tls:
  - hosts:
    - kuberde.example.com
    - "*.kuberde.example.com"
    secretName: kuberde-tls
  rules:
  - host: kuberde.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: kuberde-web
            port:
              number: 80
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: kuberde-server
            port:
              number: 8080
      - path: /ws
        pathType: Prefix
        backend:
          service:
            name: kuberde-server
            port:
              number: 8080
  - host: "*.kuberde.example.com"
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: kuberde-server
            port:
              number: 8080
EOF
```

## Step 8: Update DNS Records

```bash
# For NGINX Ingress
# Create A record for main domain
az network dns record-set a add-record \
  --resource-group $RESOURCE_GROUP \
  --zone-name kuberde.example.com \
  --record-set-name @ \
  --ipv4-address $LB_IP

# Create wildcard A record for agents
az network dns record-set a add-record \
  --resource-group $RESOURCE_GROUP \
  --zone-name kuberde.example.com \
  --record-set-name "*" \
  --ipv4-address $LB_IP

# For Application Gateway
# Use $AGW_IP instead of $LB_IP
```

## Step 9: Configure KubeRDE Server

```bash
kubectl set env deployment/kuberde-server \
  -n kuberde \
  KUBERDE_PUBLIC_URL=https://kuberde.example.com \
  KUBERDE_AGENT_DOMAIN=kuberde.example.com
```

## Step 10: Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n kuberde

# Check ingress status
kubectl get ingress -n kuberde

# Check certificate status
kubectl get certificate -n kuberde

# Check logs
kubectl logs -n kuberde deployment/kuberde-server -f
kubectl logs -n kuberde deployment/kuberde-operator -f
```

## Step 11: Access KubeRDE

1. Wait for DNS propagation (usually 5-15 minutes)
2. Navigate to `https://kuberde.example.com`
3. You should see the KubeRDE login page
4. Log in with your Keycloak credentials

## Using Azure Database for PostgreSQL (Optional)

For production deployments, consider using Azure Database for PostgreSQL:

```bash
# Create PostgreSQL server
az postgres flexible-server create \
  --resource-group $RESOURCE_GROUP \
  --name kuberde-db \
  --location $LOCATION \
  --admin-user kuberde \
  --admin-password 'YourSecurePassword123!' \
  --sku-name Standard_D2s_v3 \
  --tier GeneralPurpose \
  --version 14 \
  --storage-size 32 \
  --backup-retention 7 \
  --high-availability Enabled

# Create database
az postgres flexible-server db create \
  --resource-group $RESOURCE_GROUP \
  --server-name kuberde-db \
  --database-name kuberde

# Get connection string
az postgres flexible-server show-connection-string \
  --server-name kuberde-db \
  --admin-user kuberde \
  --admin-password 'YourSecurePassword123!' \
  --database-name kuberde

# Configure firewall to allow AKS
az postgres flexible-server firewall-rule create \
  --resource-group $RESOURCE_GROUP \
  --name kuberde-db \
  --rule-name AllowAKS \
  --start-ip-address 10.240.0.0 \
  --end-ip-address 10.240.255.255

# Update KubeRDE server
kubectl create secret generic kuberde-db-secret \
  -n kuberde \
  --from-literal=DATABASE_URL='postgres://kuberde:YourSecurePassword123!@kuberde-db.postgres.database.azure.com:5432/kuberde?sslmode=require'

kubectl set env deployment/kuberde-server \
  -n kuberde \
  --from=secret/kuberde-db-secret
```

## Cost Optimization

### Use Azure Spot VMs for Agent Workloads

```bash
# Create spot node pool
az aks nodepool add \
  --resource-group $RESOURCE_GROUP \
  --cluster-name $CLUSTER_NAME \
  --name spotpool \
  --node-count 0 \
  --min-count 0 \
  --max-count 10 \
  --enable-cluster-autoscaler \
  --priority Spot \
  --eviction-policy Delete \
  --spot-max-price -1 \
  --node-vm-size Standard_D2s_v3
```

### Enable Cluster Autoscaler

Already enabled in cluster creation, but you can update:

```bash
az aks update \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME \
  --enable-cluster-autoscaler \
  --min-count 2 \
  --max-count 10
```

### Use Azure Premium SSD v2 (Cost-effective)

```bash
# Create storage class
cat <<EOF | kubectl apply -f -
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: managed-csi-premium
provisioner: disk.csi.azure.com
parameters:
  skuName: Premium_LRS
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
EOF
```

## Monitoring and Observability

### Enable Azure Monitor for Containers

```bash
# Enable monitoring
az aks enable-addons \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME \
  --addons monitoring

# Create Log Analytics workspace (if not exists)
az monitor log-analytics workspace create \
  --resource-group $RESOURCE_GROUP \
  --workspace-name kuberde-logs
```

### Install Prometheus and Grafana

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace
```

## Backup and Disaster Recovery

### AKS Backup

```bash
# Enable backup
az aks update \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME \
  --enable-azure-rbac

# Install Velero
velero install \
  --provider azure \
  --plugins velero/velero-plugin-for-microsoft-azure:v1.8.0 \
  --bucket kuberde-backups \
  --secret-file ./credentials-velero \
  --backup-location-config \
    resourceGroup=$RESOURCE_GROUP,storageAccount=kuberdebackups
```

### Database Backups

Azure Database for PostgreSQL has automated backups enabled by default.

## Security Hardening

### Enable Azure Policy for AKS

```bash
# Enable Azure Policy add-on
az aks enable-addons \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME \
  --addons azure-policy
```

### Enable Defender for Containers

```bash
# Enable Microsoft Defender for Containers
az security pricing create \
  --name Containers \
  --tier Standard
```

### Use Azure Key Vault for Secrets

```bash
# Enable Azure Key Vault Provider for Secrets Store CSI Driver
az aks enable-addons \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME \
  --addons azure-keyvault-secrets-provider
```

### Enable Network Policies

```bash
# Update cluster to use Azure Network Policy
az aks update \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME \
  --network-policy azure
```

## Troubleshooting

### Application Gateway Not Creating

```bash
# Check AGIC logs
kubectl logs -n kube-system -l app=ingress-appgw

# Common issues:
# - Subnet overlap
# - Missing permissions
# - Invalid configuration
```

### Certificate Not Provisioning

```bash
# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager

# Check certificate status
kubectl describe certificate kuberde-tls -n kuberde

# Common issues:
# - DNS not properly configured
# - HTTP challenge failing
# - Rate limiting from Let's Encrypt
```

### Pods Not Getting IP Addresses

```bash
# Check Azure CNI configuration
kubectl get nodes -o wide

# Increase IP addresses in subnet if needed
az network vnet subnet update \
  --resource-group $RESOURCE_GROUP \
  --vnet-name kuberde-vnet \
  --name kuberde-subnet \
  --address-prefix 10.240.0.0/12
```

### High Costs

- Review node pool configurations
- Use spot instances for non-critical workloads
- Enable cluster autoscaler
- Review storage class (use Standard instead of Premium where possible)

## Next Steps

- [Configure user quotas](../CONFIGURATION.md#user-quotas)
- [Set up monitoring](../guides/OPERATORS_RUNBOOK.md#monitoring)
- [Review security best practices](../SECURITY.md)
- [Enable audit logging](../CONFIGURATION.md#audit-logging)

## Terraform Automation

For automated deployment, see:

```bash
cd terraform/azure/complete
terraform init
terraform apply
```

## Additional Resources

- [AKS Documentation](https://docs.microsoft.com/en-us/azure/aks/)
- [Application Gateway Ingress Controller](https://docs.microsoft.com/en-us/azure/application-gateway/ingress-controller-overview)
- [Azure DNS Documentation](https://docs.microsoft.com/en-us/azure/dns/)
- [Azure Database for PostgreSQL](https://docs.microsoft.com/en-us/azure/postgresql/)
- [AKS Best Practices](https://docs.microsoft.com/en-us/azure/aks/best-practices)
