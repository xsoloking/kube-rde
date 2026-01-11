# Deploying KubeRDE on Amazon Web Services (EKS)

This guide walks you through deploying KubeRDE on Amazon Elastic Kubernetes Service (EKS).

## Prerequisites

- AWS account with appropriate permissions
- `aws` CLI installed and configured
- `eksctl` installed (recommended) or Terraform
- `kubectl` installed
- Domain name with DNS access (for production deployment)
- Basic knowledge of AWS and Kubernetes

## Architecture Overview

```
Internet
    ↓
Application Load Balancer (ALB)
    ↓
EKS Cluster
    ├── KubeRDE Server (WebSocket + API)
    ├── KubeRDE Operator
    ├── PostgreSQL Database (or RDS)
    ├── Keycloak (OIDC Provider)
    ├── Web UI
    └── Agent Pods (per workspace)
```

## Step 1: Create EKS Cluster

### Option A: Using eksctl (Recommended for Quick Start)

```bash
# Set your cluster name and region
export CLUSTER_NAME="kuberde-cluster"
export AWS_REGION="us-west-2"

# Create cluster with eksctl
eksctl create cluster \
  --name $CLUSTER_NAME \
  --region $AWS_REGION \
  --version 1.28 \
  --nodegroup-name standard-workers \
  --node-type t3.medium \
  --nodes 3 \
  --nodes-min 2 \
  --nodes-max 10 \
  --managed \
  --with-oidc

# Verify cluster is ready
kubectl get nodes
```

### Option B: Using AWS Console

1. Navigate to EKS in AWS Console
2. Click "Create cluster"
3. Configure cluster settings:
   - Name: `kuberde-cluster`
   - Kubernetes version: 1.28
   - Cluster IAM role: Create or select existing
4. Configure networking:
   - VPC: Default or custom
   - Subnets: Select at least 2 in different AZs
5. Click "Create"
6. Add node group after cluster is ready

### Option C: Using Terraform

See `terraform/aws/complete/` for full Terraform configuration.

```bash
cd terraform/aws/complete
terraform init
terraform plan
terraform apply
```

### Option D: EKS with GPU Support (for Data Science Workloads)

```bash
# Create cluster with GPU node group
eksctl create cluster \
  --name $CLUSTER_NAME \
  --region $AWS_REGION \
  --version 1.28 \
  --nodegroup-name cpu-workers \
  --node-type t3.medium \
  --nodes 2

# Add GPU node group
eksctl create nodegroup \
  --cluster $CLUSTER_NAME \
  --region $AWS_REGION \
  --name gpu-workers \
  --node-type g4dn.xlarge \
  --nodes 1 \
  --nodes-min 0 \
  --nodes-max 5 \
  --node-ami-family AmazonLinux2 \
  --gpu-enabled

# Install NVIDIA device plugin
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml
```

## Step 2: Configure IAM OIDC Provider

Required for ALB Ingress Controller and other AWS integrations:

```bash
# Create OIDC provider (if not done by eksctl)
eksctl utils associate-iam-oidc-provider \
  --cluster $CLUSTER_NAME \
  --region $AWS_REGION \
  --approve
```

## Step 3: Install AWS Load Balancer Controller

### 3.1: Create IAM Policy

```bash
# Download IAM policy
curl -o iam_policy.json https://raw.githubusercontent.com/kubernetes-sigs/aws-load-balancer-controller/v2.6.0/docs/install/iam_policy.json

# Create IAM policy
aws iam create-policy \
  --policy-name AWSLoadBalancerControllerIAMPolicy \
  --policy-document file://iam_policy.json

# Note the policy ARN for next step
export POLICY_ARN=$(aws iam list-policies --query 'Policies[?PolicyName==`AWSLoadBalancerControllerIAMPolicy`].Arn' --output text)
```

### 3.2: Create IAM Service Account

```bash
# Create service account
eksctl create iamserviceaccount \
  --cluster=$CLUSTER_NAME \
  --namespace=kube-system \
  --name=aws-load-balancer-controller \
  --attach-policy-arn=$POLICY_ARN \
  --override-existing-serviceaccounts \
  --region $AWS_REGION \
  --approve
```

### 3.3: Install ALB Controller

```bash
# Add EKS Helm repository
helm repo add eks https://aws.github.io/eks-charts
helm repo update

# Install AWS Load Balancer Controller
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=$CLUSTER_NAME \
  --set serviceAccount.create=false \
  --set serviceAccount.name=aws-load-balancer-controller

# Verify installation
kubectl get deployment -n kube-system aws-load-balancer-controller
```

### Alternative: Using NGINX Ingress Controller

If you prefer NGINX over ALB:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.0/deploy/static/provider/aws/deploy.yaml
```

## Step 4: Configure Route53 DNS

### Option A: Create New Hosted Zone

```bash
# Create hosted zone
aws route53 create-hosted-zone \
  --name kuberde.example.com \
  --caller-reference $(date +%s)

# Note the name servers and update your domain registrar
aws route53 get-hosted-zone \
  --id /hostedzone/YOUR_ZONE_ID \
  --query 'DelegationSet.NameServers'
```

### Option B: Use Existing Hosted Zone

```bash
# List hosted zones
aws route53 list-hosted-zones

# Note your hosted zone ID
export HOSTED_ZONE_ID="/hostedzone/Z1234567890ABC"
```

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

## Step 6: Request ACM Certificate

### Option A: Using AWS Certificate Manager (Recommended)

```bash
# Request certificate
aws acm request-certificate \
  --domain-name kuberde.example.com \
  --subject-alternative-names "*.kuberde.example.com" \
  --validation-method DNS \
  --region $AWS_REGION

# Note the certificate ARN
export CERT_ARN=$(aws acm list-certificates \
  --query 'CertificateSummaryList[?DomainName==`kuberde.example.com`].CertificateArn' \
  --output text \
  --region $AWS_REGION)

# Get validation records
aws acm describe-certificate \
  --certificate-arn $CERT_ARN \
  --region $AWS_REGION \
  --query 'Certificate.DomainValidationOptions[*].[ResourceRecord.Name, ResourceRecord.Value]' \
  --output table
```

### Option B: Add DNS Validation Records to Route53

```bash
# Get validation details
VALIDATION_NAME=$(aws acm describe-certificate \
  --certificate-arn $CERT_ARN \
  --region $AWS_REGION \
  --query 'Certificate.DomainValidationOptions[0].ResourceRecord.Name' \
  --output text)

VALIDATION_VALUE=$(aws acm describe-certificate \
  --certificate-arn $CERT_ARN \
  --region $AWS_REGION \
  --query 'Certificate.DomainValidationOptions[0].ResourceRecord.Value' \
  --output text)

# Create validation record
aws route53 change-resource-record-sets \
  --hosted-zone-id $HOSTED_ZONE_ID \
  --change-batch '{
    "Changes": [{
      "Action": "CREATE",
      "ResourceRecordSet": {
        "Name": "'"$VALIDATION_NAME"'",
        "Type": "CNAME",
        "TTL": 300,
        "ResourceRecords": [{"Value": "'"$VALIDATION_VALUE"'"}]
      }
    }]
  }'

# Wait for certificate validation (can take 5-30 minutes)
aws acm wait certificate-validated \
  --certificate-arn $CERT_ARN \
  --region $AWS_REGION
```

### Option C: Use cert-manager with Let's Encrypt

```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Create ClusterIssuer
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
          class: alb
EOF
```

## Step 7: Configure Ingress

### Option A: Using ALB Ingress Controller with ACM

```bash
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kuberde-ingress
  namespace: kuberde
  annotations:
    kubernetes.io/ingress.class: alb
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/certificate-arn: $CERT_ARN
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTP": 80}, {"HTTPS": 443}]'
    alb.ingress.kubernetes.io/ssl-redirect: '443'
    alb.ingress.kubernetes.io/healthcheck-path: /health
    alb.ingress.kubernetes.io/backend-protocol: HTTP
    alb.ingress.kubernetes.io/success-codes: '200,301,302'
    # WebSocket support
    alb.ingress.kubernetes.io/target-group-attributes: |
      stickiness.enabled=true,
      stickiness.lb_cookie.duration_seconds=3600,
      deregistration_delay.timeout_seconds=30
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

### Option B: Using NGINX Ingress with cert-manager

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

## Step 8: Update DNS Records

```bash
# Get ALB DNS name
export ALB_DNS=$(kubectl get ingress kuberde-ingress -n kuberde -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')

echo "ALB DNS: $ALB_DNS"

# Create Route53 records
# Main domain
aws route53 change-resource-record-sets \
  --hosted-zone-id $HOSTED_ZONE_ID \
  --change-batch '{
    "Changes": [{
      "Action": "CREATE",
      "ResourceRecordSet": {
        "Name": "kuberde.example.com",
        "Type": "A",
        "AliasTarget": {
          "HostedZoneId": "Z1234567890ABC",
          "DNSName": "'"$ALB_DNS"'",
          "EvaluateTargetHealth": false
        }
      }
    }]
  }'

# Wildcard for agents
aws route53 change-resource-record-sets \
  --hosted-zone-id $HOSTED_ZONE_ID \
  --change-batch '{
    "Changes": [{
      "Action": "CREATE",
      "ResourceRecordSet": {
        "Name": "*.kuberde.example.com",
        "Type": "A",
        "AliasTarget": {
          "HostedZoneId": "Z1234567890ABC",
          "DNSName": "'"$ALB_DNS"'",
          "EvaluateTargetHealth": false
        }
      }
    }]
  }'
```

**Note**: Replace `Z1234567890ABC` with your ALB's hosted zone ID. Find it [here](https://docs.aws.amazon.com/general/latest/gr/elb.html).

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

# Describe ingress to see ALB details
kubectl describe ingress kuberde-ingress -n kuberde

# Check logs
kubectl logs -n kuberde deployment/kuberde-server -f
kubectl logs -n kuberde deployment/kuberde-operator -f
```

## Step 11: Access KubeRDE

1. Wait for DNS propagation (usually 5-15 minutes)
2. Navigate to `https://kuberde.example.com`
3. You should see the KubeRDE login page
4. Log in with your Keycloak credentials

## Using Amazon RDS for PostgreSQL (Optional)

For production deployments, consider using RDS instead of in-cluster PostgreSQL:

### Create RDS Instance

```bash
# Create DB subnet group
aws rds create-db-subnet-group \
  --db-subnet-group-name kuberde-db-subnet \
  --db-subnet-group-description "KubeRDE DB subnet group" \
  --subnet-ids subnet-12345 subnet-67890

# Create security group
aws ec2 create-security-group \
  --group-name kuberde-rds-sg \
  --description "KubeRDE RDS security group" \
  --vpc-id vpc-12345

# Allow EKS nodes to access RDS
aws ec2 authorize-security-group-ingress \
  --group-id sg-12345 \
  --protocol tcp \
  --port 5432 \
  --source-group eks-node-sg

# Create RDS instance
aws rds create-db-instance \
  --db-instance-identifier kuberde-db \
  --db-instance-class db.t3.medium \
  --engine postgres \
  --engine-version 14.9 \
  --master-username kuberde \
  --master-user-password 'YourSecurePassword' \
  --allocated-storage 20 \
  --db-subnet-group-name kuberde-db-subnet \
  --vpc-security-group-ids sg-12345 \
  --backup-retention-period 7 \
  --preferred-backup-window "03:00-04:00" \
  --storage-encrypted \
  --no-publicly-accessible

# Wait for instance to be available
aws rds wait db-instance-available \
  --db-instance-identifier kuberde-db

# Get endpoint
aws rds describe-db-instances \
  --db-instance-identifier kuberde-db \
  --query 'DBInstances[0].Endpoint.Address' \
  --output text
```

### Update KubeRDE Configuration

```bash
# Update server deployment to use RDS
kubectl create secret generic kuberde-db-secret \
  -n kuberde \
  --from-literal=DATABASE_URL='postgres://kuberde:YourSecurePassword@kuberde-db.region.rds.amazonaws.com:5432/kuberde?sslmode=require'

kubectl set env deployment/kuberde-server \
  -n kuberde \
  --from=secret/kuberde-db-secret
```

## Cost Optimization

### Use Spot Instances for Agent Workloads

```bash
# Create spot instance node group
eksctl create nodegroup \
  --cluster $CLUSTER_NAME \
  --region $AWS_REGION \
  --name spot-agents \
  --node-type t3.medium \
  --nodes 0 \
  --nodes-min 0 \
  --nodes-max 10 \
  --spot
```

### Enable Cluster Autoscaler

```bash
# Install cluster autoscaler
kubectl apply -f https://raw.githubusercontent.com/kubernetes/autoscaler/master/cluster-autoscaler/cloudprovider/aws/examples/cluster-autoscaler-autodiscover.yaml

# Configure for your cluster
kubectl -n kube-system \
  annotate deployment.apps/cluster-autoscaler \
  cluster-autoscaler.kubernetes.io/safe-to-evict="false"

kubectl -n kube-system \
  set image deployment.apps/cluster-autoscaler \
  cluster-autoscaler=registry.k8s.io/autoscaling/cluster-autoscaler:v1.27.0
```

### Use EBS gp3 Volumes

```bash
# Create gp3 storage class
cat <<EOF | kubectl apply -f -
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: gp3
provisioner: ebs.csi.aws.com
parameters:
  type: gp3
  iops: "3000"
  throughput: "125"
volumeBindingMode: WaitForFirstConsumer
EOF
```

## Monitoring and Observability

### Enable Container Insights

```bash
# Install Container Insights
aws eks create-addon \
  --cluster-name $CLUSTER_NAME \
  --addon-name amazon-cloudwatch-observability \
  --region $AWS_REGION
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

### EBS Snapshots

```bash
# Create snapshot lifecycle policy
aws dlm create-lifecycle-policy \
  --description "KubeRDE EBS backup policy" \
  --state ENABLED \
  --execution-role-arn arn:aws:iam::123456789012:role/AWSDataLifecycleManagerDefaultRole \
  --policy-details file://lifecycle-policy.json
```

### Velero for Cluster Backups

```bash
# Install Velero
velero install \
  --provider aws \
  --plugins velero/velero-plugin-for-aws:v1.8.0 \
  --bucket kuberde-backups \
  --backup-location-config region=$AWS_REGION \
  --snapshot-location-config region=$AWS_REGION \
  --secret-file ./credentials-velero
```

## Security Hardening

### Enable EKS Secrets Encryption

```bash
# Create KMS key
aws kms create-key \
  --description "KubeRDE EKS encryption key"

# Enable secrets encryption
aws eks associate-encryption-config \
  --cluster-name $CLUSTER_NAME \
  --encryption-config '[{"resources":["secrets"],"provider":{"keyArn":"arn:aws:kms:region:account:key/key-id"}}]'
```

### Use AWS Secrets Manager

```bash
# Install AWS Secrets CSI Driver
kubectl apply -f https://raw.githubusercontent.com/aws/secrets-store-csi-driver-provider-aws/main/deployment/aws-provider-installer.yaml
```

### Enable VPC Flow Logs

```bash
aws ec2 create-flow-logs \
  --resource-type VPC \
  --resource-ids vpc-12345 \
  --traffic-type ALL \
  --log-destination-type cloud-watch-logs \
  --log-group-name /aws/vpc/kuberde
```

## Troubleshooting

### ALB Not Creating

```bash
# Check controller logs
kubectl logs -n kube-system deployment/aws-load-balancer-controller

# Common issues:
# - Missing IAM permissions
# - OIDC provider not configured
# - Subnet tags missing
```

### ACM Certificate Not Validating

```bash
# Check certificate status
aws acm describe-certificate \
  --certificate-arn $CERT_ARN \
  --region $AWS_REGION

# Verify DNS validation records
aws route53 list-resource-record-sets \
  --hosted-zone-id $HOSTED_ZONE_ID
```

### Pods Not Getting IP Addresses

```bash
# Check VPC CNI plugin
kubectl get daemonset -n kube-system aws-node

# Increase IP addresses available
kubectl set env daemonset aws-node \
  -n kube-system \
  WARM_IP_TARGET=5
```

### WebSocket Connection Issues

Ensure ALB sticky sessions are enabled (configured in ingress annotations above).

## Next Steps

- [Configure user quotas](../CONFIGURATION.md#user-quotas)
- [Set up monitoring](../guides/OPERATORS_RUNBOOK.md#monitoring)
- [Review security best practices](../SECURITY.md)
- [Enable audit logging](../CONFIGURATION.md#audit-logging)

## Terraform Automation

For fully automated deployment, see:

```bash
cd terraform/aws/complete
terraform init
terraform apply
```

## Additional Resources

- [EKS Documentation](https://docs.aws.amazon.com/eks/)
- [AWS Load Balancer Controller](https://kubernetes-sigs.github.io/aws-load-balancer-controller/)
- [Route53 Documentation](https://docs.aws.amazon.com/route53/)
- [ACM Documentation](https://docs.aws.amazon.com/acm/)
- [EKS Best Practices Guide](https://aws.github.io/aws-eks-best-practices/)
