# Tutorial 7: Cloud Deployment

**Time:** 30 minutes
**Difficulty:** Intermediate
**Prerequisites:** Basic Kubernetes knowledge, Cloud account (GCP/AWS/Azure)

## What You'll Learn

- How to deploy KubeRDE to production cloud environments
- Best practices for cloud deployment
- How to configure domain names and TLS
- How to set up monitoring and logging
- Production-ready configuration

## Prerequisites

- Cloud account (GCP, AWS, or Azure)
- `kubectl` installed and configured
- Domain name (for production)
- Basic understanding of Kubernetes
- Helm 3 installed

## Choose Your Cloud Platform

This tutorial covers deployment to:
- [Google Kubernetes Engine (GKE)](#deploy-to-gke)
- [Amazon Elastic Kubernetes Service (EKS)](#deploy-to-eks)
- [Azure Kubernetes Service (AKS)](#deploy-to-aks)

Each section is self-contained—choose the platform you'll use.

---

## Deploy to GKE

### Step 1: Create GKE Cluster

**Using gcloud CLI:**

```bash
# Set variables
export PROJECT_ID="your-project-id"
export CLUSTER_NAME="kuberde-prod"
export REGION="us-central1"
export ZONE="${REGION}-a"

# Create cluster
gcloud container clusters create $CLUSTER_NAME \
  --project=$PROJECT_ID \
  --zone=$ZONE \
  --machine-type=n1-standard-2 \
  --num-nodes=3 \
  --enable-autoscaling \
  --min-nodes=3 \
  --max-nodes=10 \
  --enable-autorepair \
  --enable-autoupgrade \
  --disk-size=50GB \
  --disk-type=pd-standard
```

**Get credentials:**
```bash
gcloud container clusters get-credentials $CLUSTER_NAME --zone=$ZONE
```

**Verify:**
```bash
kubectl get nodes
```

### Step 2: Install NGINX Ingress Controller

```bash
# Install NGINX Ingress
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/cloud/deploy.yaml

# Wait for external IP
kubectl get svc -n ingress-nginx ingress-nginx-controller -w
```

Note the EXTERNAL-IP for DNS configuration.

### Step 3: Configure Cloud DNS

```bash
# Get Ingress IP
export INGRESS_IP=$(kubectl get svc -n ingress-nginx ingress-nginx-controller \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

echo "Ingress IP: $INGRESS_IP"

# Create DNS record
gcloud dns managed-zones create kuberde-zone \
  --dns-name="kuberde.example.com." \
  --description="KubeRDE DNS Zone"

# Add A record for main domain
gcloud dns record-sets create kuberde.example.com. \
  --zone=kuberde-zone \
  --type=A \
  --ttl=300 \
  --rrdatas=$INGRESS_IP

# Add wildcard for agents
gcloud dns record-sets create "*.kuberde.example.com." \
  --zone=kuberde-zone \
  --type=A \
  --ttl=300 \
  --rrdatas=$INGRESS_IP
```

### Step 4: Install cert-manager

```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Wait for cert-manager
kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=300s

# Create Let's Encrypt issuer
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

### Step 5: Deploy KubeRDE

```bash
# Add Helm repository (if available)
# helm repo add kuberde https://charts.kuberde.io
# helm repo update

# Or clone repository
git clone https://github.com/xsoloking/kube-rde.git
cd kube-rde

# Install using Helm
helm upgrade --install kuberde ./charts/kuberde \
  --namespace kuberde \
  --create-namespace \
  --set global.domain=kuberde.example.com \
  --set global.publicUrl=https://kuberde.example.com \
  --set ingress.className=nginx \
  --set ingress.tls.enabled=true \
  --set ingress.tls.issuer=letsencrypt-prod \
  --set postgresql.persistence.size=20Gi \
  --set server.replicaCount=2 \
  --wait \
  --timeout=10m
```

### Step 6: Verify Deployment

```bash
# Check pods
kubectl get pods -n kuberde

# Check ingress
kubectl get ingress -n kuberde

# Check certificate
kubectl get certificate -n kuberde

# Wait for certificate to be ready
kubectl wait --for=condition=ready certificate kuberde-tls -n kuberde --timeout=300s
```

Access KubeRDE at: https://kuberde.example.com

---

## Deploy to EKS

### Step 1: Create EKS Cluster

**Using eksctl:**

```bash
# Set variables
export CLUSTER_NAME="kuberde-prod"
export AWS_REGION="us-east-1"

# Create cluster
eksctl create cluster \
  --name $CLUSTER_NAME \
  --region $AWS_REGION \
  --version 1.28 \
  --nodegroup-name standard-workers \
  --node-type t3.medium \
  --nodes 3 \
  --nodes-min 3 \
  --nodes-max 10 \
  --managed
```

**Get credentials:**
```bash
aws eks update-kubeconfig --name $CLUSTER_NAME --region $AWS_REGION
```

### Step 2: Install AWS Load Balancer Controller

```bash
# Install AWS Load Balancer Controller
kubectl apply -k "github.com/aws/eks-charts/stable/aws-load-balancer-controller/crds?ref=master"

helm repo add eks https://aws.github.io/eks-charts
helm repo update

helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=$CLUSTER_NAME \
  --set serviceAccount.create=true \
  --set region=$AWS_REGION
```

Or install NGINX Ingress:
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/aws/deploy.yaml
```

### Step 3: Configure Route53

```bash
# Get Load Balancer DNS name
export LB_DNS=$(kubectl get svc -n ingress-nginx ingress-nginx-controller \
  -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')

echo "Load Balancer: $LB_DNS"

# Create hosted zone
aws route53 create-hosted-zone \
  --name kuberde.example.com \
  --caller-reference $(date +%s)

# Get zone ID
export ZONE_ID=$(aws route53 list-hosted-zones \
  --query "HostedZones[?Name=='kuberde.example.com.'].Id" \
  --output text | cut -d'/' -f3)

# Create alias record
cat > /tmp/route53-change.json <<EOF
{
  "Changes": [{
    "Action": "CREATE",
    "ResourceRecordSet": {
      "Name": "kuberde.example.com",
      "Type": "A",
      "AliasTarget": {
        "HostedZoneId": "Z1234567890ABC",
        "DNSName": "$LB_DNS",
        "EvaluateTargetHealth": false
      }
    }
  }]
}
EOF

aws route53 change-resource-record-sets \
  --hosted-zone-id $ZONE_ID \
  --change-batch file:///tmp/route53-change.json
```

### Step 4: Install cert-manager and Deploy

Follow the same cert-manager installation and KubeRDE deployment steps as GKE above.

---

## Deploy to AKS

### Step 1: Create AKS Cluster

```bash
# Set variables
export RESOURCE_GROUP="kuberde-rg"
export CLUSTER_NAME="kuberde-prod"
export LOCATION="eastus"

# Create resource group
az group create --name $RESOURCE_GROUP --location $LOCATION

# Create cluster
az aks create \
  --resource-group $RESOURCE_GROUP \
  --name $CLUSTER_NAME \
  --node-count 3 \
  --node-vm-size Standard_D2s_v3 \
  --enable-managed-identity \
  --enable-cluster-autoscaler \
  --min-count 3 \
  --max-count 10 \
  --generate-ssh-keys
```

**Get credentials:**
```bash
az aks get-credentials --resource-group $RESOURCE_GROUP --name $CLUSTER_NAME
```

### Step 2: Install NGINX Ingress

```bash
# Install NGINX Ingress
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/cloud/deploy.yaml

# Get external IP
kubectl get svc -n ingress-nginx ingress-nginx-controller -w
```

### Step 3: Configure Azure DNS

```bash
# Get Ingress IP
export INGRESS_IP=$(kubectl get svc -n ingress-nginx ingress-nginx-controller \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# Create DNS zone
az network dns zone create \
  --resource-group $RESOURCE_GROUP \
  --name kuberde.example.com

# Create A record
az network dns record-set a add-record \
  --resource-group $RESOURCE_GROUP \
  --zone-name kuberde.example.com \
  --record-set-name @ \
  --ipv4-address $INGRESS_IP

# Create wildcard record
az network dns record-set a add-record \
  --resource-group $RESOURCE_GROUP \
  --zone-name kuberde.example.com \
  --record-set-name "*" \
  --ipv4-address $INGRESS_IP
```

### Step 4: Install cert-manager and Deploy

Follow the same cert-manager installation and KubeRDE deployment steps as GKE above.

---

## Post-Deployment Configuration

### Configure Keycloak

1. **Access Keycloak admin:**
   ```
   https://kuberde.example.com/auth/admin
   ```

2. **Login:** admin / admin

3. **Change admin password:**
   - Users → admin → Credentials → Reset Password

4. **Configure realm:**
   - Verify realm settings
   - Set up email server (SMTP)
   - Configure password policies

### Create First User

1. **In Keycloak admin:**
   - Users → Add User
   - Username: alice
   - Email: alice@example.com
   - Email verified: ON
   - Save

2. **Set password:**
   - Credentials tab
   - Set password
   - Temporary: OFF

3. **Test login:**
   - Go to https://kuberde.example.com
   - Login as alice

### Configure Resource Quotas

1. **In Web UI (as admin):**
   - Navigate to User Management
   - Select user (alice)
   - Set quotas:
     - Max workspaces: 5
     - Max CPU: 10000m
     - Max memory: 20Gi
     - Max storage: 100Gi

### Set Up Monitoring (Optional)

**Install Prometheus and Grafana:**

```bash
# Add Prometheus Helm repository
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install Prometheus + Grafana
helm install monitoring prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set grafana.adminPassword=admin
```

**Access Grafana:**
```bash
kubectl port-forward -n monitoring svc/monitoring-grafana 3000:80
```

Navigate to http://localhost:3000

## Production Best Practices

### Security

1. **Change all default passwords**
2. **Enable RBAC**
3. **Use network policies**
4. **Regular security updates**
5. **Enable audit logging**

### High Availability

```bash
# Scale server
kubectl scale deployment kuberde-server -n kuberde --replicas=3

# Use external PostgreSQL
helm upgrade kuberde ./charts/kuberde \
  --set postgresql.enabled=false \
  --set externalDatabase.host=postgres.example.com \
  --set externalDatabase.password=secure-password
```

### Backup

```bash
# Backup PostgreSQL
kubectl exec -n kuberde kuberde-postgresql-0 -- \
  pg_dump -U kuberde kuberde > backup-$(date +%Y%m%d).sql

# Backup PVCs
velero backup create kuberde-backup --include-namespaces kuberde
```

### Monitoring

- Set up alerts for pod failures
- Monitor resource usage
- Track agent connection metrics
- Log aggregation (ELK, Loki)

## Verification

Verify production deployment:

1. ✓ **HTTPS working:** https://kuberde.example.com shows green lock
2. ✓ **Certificate valid:** Check cert-manager certificate
3. ✓ **Services running:** All pods in Running state
4. ✓ **Can create workspace:** Test workspace creation
5. ✓ **SSH access works:** Connect via CLI
6. ✓ **Monitoring active:** Grafana shows metrics

## Troubleshooting

### Certificate Not Issuing

```bash
# Check certificate status
kubectl describe certificate kuberde-tls -n kuberde

# Check cert-manager logs
kubectl logs -n cert-manager -l app=cert-manager -f

# Check challenges
kubectl get challenges -n kuberde
```

### Ingress Not Working

```bash
# Check ingress
kubectl describe ingress -n kuberde

# Check ingress controller logs
kubectl logs -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx -f

# Verify DNS
dig kuberde.example.com
```

### Pod Crashes

```bash
# Check pod logs
kubectl logs -n kuberde <pod-name>

# Check events
kubectl get events -n kuberde --sort-by='.lastTimestamp'

# Describe pod
kubectl describe pod -n kuberde <pod-name>
```

## Next Steps

- [Tutorial 8: Domain and TLS Setup](08-domain-tls-setup.md)
- [Tutorial 9: User Management](09-user-management.md)
- [Tutorial 13: Monitoring and Observability](13-monitoring-observability.md)
- [Tutorial 14: Security Hardening](14-security-hardening.md)

## Additional Resources

- [GKE Deployment Guide](../platforms/gcp-gke.md)
- [EKS Deployment Guide](../platforms/aws-eks.md)
- [AKS Deployment Guide](../platforms/azure-aks.md)
- [Production Checklist](../guides/PRODUCTION_CHECKLIST.md)

## Video Tutorial

[![Cloud Deployment Video](../media/videos/thumbnails/tutorial-07-thumbnail.png)](../media/videos/tutorial-04-production-deployment.mp4)

---

**Congratulations!** 🎉 You've deployed KubeRDE to production. Your team can now access secure, scalable remote development environments.
