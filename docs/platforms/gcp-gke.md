# Deploying KubeRDE on Google Cloud Platform (GKE)

This guide walks you through deploying KubeRDE on Google Kubernetes Engine (GKE).

## Prerequisites

- Google Cloud account with billing enabled
- `gcloud` CLI installed and configured
- `kubectl` installed
- Domain name with DNS access (for production deployment)
- Basic knowledge of GCP and Kubernetes

## Architecture Overview

```
Internet
    ↓
Cloud Load Balancer (managed by GCE Ingress Controller)
    ↓
GKE Cluster
    ├── KubeRDE Server (WebSocket + API)
    ├── KubeRDE Operator
    ├── PostgreSQL Database
    ├── Keycloak (OIDC Provider)
    ├── Web UI
    └── Agent Pods (per workspace)
```

## Step 1: Create GKE Cluster

### Option A: Standard GKE Cluster (Recommended for Production)

```bash
# Set your project ID
export PROJECT_ID="your-gcp-project-id"
export CLUSTER_NAME="kuberde-cluster"
export REGION="us-central1"
export ZONE="us-central1-a"

# Set project
gcloud config set project $PROJECT_ID

# Create GKE cluster with autoscaling
gcloud container clusters create $CLUSTER_NAME \
  --zone $ZONE \
  --num-nodes 3 \
  --machine-type n1-standard-2 \
  --enable-autoscaling \
  --min-nodes 3 \
  --max-nodes 10 \
  --enable-autorepair \
  --enable-autoupgrade \
  --enable-ip-alias \
  --network "default" \
  --subnetwork "default" \
  --addons HorizontalPodAutoscaling,HttpLoadBalancing \
  --workload-pool=$PROJECT_ID.svc.id.goog \
  --enable-stackdriver-kubernetes

# Get cluster credentials
gcloud container clusters get-credentials $CLUSTER_NAME --zone $ZONE
```

### Option B: Autopilot GKE Cluster (Managed)

```bash
# Create Autopilot cluster (fully managed)
gcloud container clusters create-auto $CLUSTER_NAME \
  --region $REGION \
  --project $PROJECT_ID

# Get cluster credentials
gcloud container clusters get-credentials $CLUSTER_NAME --region $REGION
```

### Option C: GKE with GPU Support (for Data Science Workloads)

```bash
# Create node pool with GPUs
gcloud container clusters create $CLUSTER_NAME \
  --zone $ZONE \
  --num-nodes 2 \
  --machine-type n1-standard-2

# Add GPU node pool
gcloud container node-pools create gpu-pool \
  --cluster $CLUSTER_NAME \
  --zone $ZONE \
  --machine-type n1-standard-4 \
  --accelerator type=nvidia-tesla-t4,count=1 \
  --num-nodes 1 \
  --min-nodes 0 \
  --max-nodes 5 \
  --enable-autoscaling

# Install NVIDIA GPU drivers
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/container-engine-accelerators/master/nvidia-driver-installer/cos/daemonset-preloaded.yaml
```

## Step 2: Configure DNS

### Option A: Cloud DNS (Recommended)

```bash
# Create DNS zone
gcloud dns managed-zones create kuberde-zone \
  --dns-name="kuberde.example.com." \
  --description="KubeRDE DNS Zone"

# Note the name servers
gcloud dns managed-zones describe kuberde-zone

# Update your domain registrar with these name servers
```

### Option B: External DNS Provider

If using external DNS (Cloudflare, Route53, etc.), you'll configure it after getting the Load Balancer IP in Step 4.

## Step 3: Install Ingress Controller

You have two options for Ingress on GKE:

### Option A: GCE Ingress Controller (Default, Recommended)

GKE comes with GCE Ingress Controller pre-installed. It creates Google Cloud Load Balancers automatically.

**Benefits**:
- Fully managed by Google
- Automatic SSL certificate provisioning with Google-managed certificates
- Integrated with Cloud Armor for DDoS protection
- No additional setup required

**Limitations**:
- Only supports L7 (HTTP/HTTPS) load balancing
- Slower updates compared to NGINX
- Less flexible configuration options

**No installation needed** - proceed to Step 4.

### Option B: NGINX Ingress Controller (More Flexible)

**Benefits**:
- More configuration options
- Faster updates
- Better WebSocket support
- Community standard

**Installation**:

```bash
# Install NGINX Ingress Controller
kubectl create namespace ingress-nginx

kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/cloud/deploy.yaml

# Wait for external IP
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s

# Get the external IP
kubectl get service -n ingress-nginx ingress-nginx-controller
```

## Step 4: Deploy KubeRDE Core Components

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

## Step 5: Configure Ingress

### Option A: Using GCE Ingress Controller

#### 5.1: Reserve Static IP

```bash
# Reserve a global static IP
gcloud compute addresses create kuberde-ip --global

# Get the IP address
gcloud compute addresses describe kuberde-ip --global --format="get(address)"
```

#### 5.2: Update DNS

```bash
# Using Cloud DNS
gcloud dns record-sets create kuberde.example.com. \
  --zone=kuberde-zone \
  --type=A \
  --ttl=300 \
  --rrdatas=<STATIC_IP>

# Wildcard for agents
gcloud dns record-sets create "*.kuberde.example.com." \
  --zone=kuberde-zone \
  --type=A \
  --ttl=300 \
  --rrdatas=<STATIC_IP>
```

#### 5.3: Create Managed Certificate

```bash
# Create managed certificate resource
cat <<EOF | kubectl apply -f -
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: kuberde-cert
  namespace: kuberde
spec:
  domains:
    - kuberde.example.com
    - "*.kuberde.example.com"
EOF
```

#### 5.4: Create Ingress

```bash
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kuberde-ingress
  namespace: kuberde
  annotations:
    kubernetes.io/ingress.global-static-ip-name: "kuberde-ip"
    networking.gke.io/managed-certificates: "kuberde-cert"
    kubernetes.io/ingress.class: "gce"
spec:
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
      - path: /api/*
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
      - path: /*
        pathType: Prefix
        backend:
          service:
            name: kuberde-server
            port:
              number: 8080
EOF
```

**Note**: GCE managed certificates can take 15-60 minutes to provision.

### Option B: Using NGINX Ingress Controller

#### 5.1: Get Load Balancer IP

```bash
# Get NGINX external IP
export INGRESS_IP=$(kubectl get service -n ingress-nginx ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
echo $INGRESS_IP
```

#### 5.2: Update DNS

```bash
# Using Cloud DNS
gcloud dns record-sets create kuberde.example.com. \
  --zone=kuberde-zone \
  --type=A \
  --ttl=300 \
  --rrdatas=$INGRESS_IP

# Wildcard for agents
gcloud dns record-sets create "*.kuberde.example.com." \
  --zone=kuberde-zone \
  --type=A \
  --ttl=300 \
  --rrdatas=$INGRESS_IP
```

#### 5.3: Install cert-manager

```bash
# Install cert-manager for TLS certificates
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Wait for cert-manager to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=120s
```

#### 5.4: Create ClusterIssuer

```bash
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

#### 5.5: Create Ingress

```bash
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

## Step 6: Configure KubeRDE Server

Update the server configuration with your domain:

```bash
kubectl set env deployment/kuberde-server \
  -n kuberde \
  KUBERDE_PUBLIC_URL=https://kuberde.example.com \
  KUBERDE_AGENT_DOMAIN=kuberde.example.com
```

## Step 7: Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n kuberde

# Check ingress status
kubectl get ingress -n kuberde

# Check certificate status (if using cert-manager)
kubectl get certificate -n kuberde

# Check logs
kubectl logs -n kuberde deployment/kuberde-server -f
kubectl logs -n kuberde deployment/kuberde-operator -f
```

## Step 8: Access KubeRDE

1. Wait for DNS propagation (can take up to 48 hours, usually 5-15 minutes)
2. Navigate to `https://kuberde.example.com`
3. You should see the KubeRDE login page
4. Log in with your Keycloak credentials

## Step 9: Create Your First Workspace

```bash
# Using CLI
kuberde-cli login --server https://kuberde.example.com
kuberde-cli workspace create my-dev-env

# Or use the Web UI at https://kuberde.example.com
```

## Cost Optimization

### Use Autopilot for Variable Workloads

Autopilot automatically scales nodes based on demand, potentially reducing costs.

### Enable Cluster Autoscaler

```bash
gcloud container clusters update $CLUSTER_NAME \
  --enable-autoscaling \
  --min-nodes 2 \
  --max-nodes 10 \
  --zone $ZONE
```

### Use Preemptible/Spot Instances for Agent Pools

```bash
# Create preemptible node pool for agents
gcloud container node-pools create agent-pool \
  --cluster $CLUSTER_NAME \
  --zone $ZONE \
  --machine-type n1-standard-4 \
  --preemptible \
  --num-nodes 0 \
  --min-nodes 0 \
  --max-nodes 10 \
  --enable-autoscaling
```

### Set Resource Quotas

Configure KubeRDE user quotas to prevent resource waste.

## Monitoring and Observability

### Enable GKE Monitoring

GKE automatically sends metrics to Cloud Monitoring.

```bash
# View in Cloud Console
gcloud console dashboard --project $PROJECT_ID
```

### Install Prometheus and Grafana (Optional)

```bash
# Add Prometheus Helm repo
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install Prometheus
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace
```

## Backup and Disaster Recovery

### Database Backups

```bash
# Enable automated backups for CloudSQL (if using CloudSQL instead of in-cluster PostgreSQL)
gcloud sql instances patch kuberde-db \
  --backup-start-time=03:00 \
  --enable-bin-log
```

### Cluster Backups

```bash
# Use Velero for cluster backups
kubectl apply -f https://github.com/vmware-tanzu/velero/releases/download/v1.12.0/velero-v1.12.0-linux-amd64.tar.gz

# Configure GCS bucket for backups
gsutil mb gs://kuberde-backups-$PROJECT_ID

# Install Velero
velero install \
  --provider gcp \
  --plugins velero/velero-plugin-for-gcp:v1.8.0 \
  --bucket kuberde-backups-$PROJECT_ID \
  --secret-file ./credentials-velero
```

## Security Hardening

### Enable Workload Identity

```bash
# Enable Workload Identity
gcloud container clusters update $CLUSTER_NAME \
  --workload-pool=$PROJECT_ID.svc.id.goog \
  --zone $ZONE
```

### Enable Binary Authorization

```bash
# Enable Binary Authorization
gcloud container clusters update $CLUSTER_NAME \
  --enable-binauthz \
  --zone $ZONE
```

### Configure Cloud Armor (with GCE Ingress)

```bash
# Create Cloud Armor security policy
gcloud compute security-policies create kuberde-policy \
  --description "KubeRDE security policy"

# Add DDoS protection rules
gcloud compute security-policies rules create 1000 \
  --security-policy kuberde-policy \
  --expression "origin.region_code == 'CN'" \
  --action "deny-403"
```

## Troubleshooting

### Certificate Not Provisioning (GCE)

```bash
# Check certificate status
kubectl describe managedcertificate kuberde-cert -n kuberde

# Common issues:
# - DNS not properly configured
# - Waiting for propagation (can take up to 60 minutes)
# - Invalid domain format
```

### Load Balancer Not Getting IP

```bash
# Check ingress status
kubectl describe ingress kuberde-ingress -n kuberde

# Check service
kubectl get svc -n kuberde
```

### WebSocket Connection Issues

```bash
# For GCE Ingress, ensure backend config is set
kubectl annotate service kuberde-server \
  -n kuberde \
  cloud.google.com/backend-config='{"default": "kuberde-backendconfig"}'
```

### High Costs

- Check node pool sizes and autoscaling settings
- Review resource requests and limits
- Consider using preemptible instances
- Enable cluster autoscaler

## Next Steps

- [Configure user quotas](../CONFIGURATION.md#user-quotas)
- [Set up monitoring](../guides/OPERATORS_RUNBOOK.md#monitoring)
- [Review security best practices](../SECURITY.md)
- [Enable audit logging](../CONFIGURATION.md#audit-logging)

## Terraform Automation

For automated deployment, see the Terraform modules in `terraform/gcp/`:

```bash
cd terraform/gcp/complete
terraform init
terraform apply
```

## Additional Resources

- [GKE Documentation](https://cloud.google.com/kubernetes-engine/docs)
- [GCE Ingress Controller](https://cloud.google.com/kubernetes-engine/docs/concepts/ingress)
- [Google Cloud Load Balancing](https://cloud.google.com/load-balancing/docs)
- [Cloud DNS Documentation](https://cloud.google.com/dns/docs)
