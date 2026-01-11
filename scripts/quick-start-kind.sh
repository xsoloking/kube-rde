#!/usr/bin/env bash

# KubeRDE Quick Start Script for kind (Kubernetes in Docker)
# This script sets up a complete KubeRDE environment on your local machine using kind

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-kuberde}"
DOMAIN="${DOMAIN:-kuberde.local}"
NAMESPACE="${NAMESPACE:-kuberde}"

# Functions
log_info() {
    echo -e "${BLUE}ℹ ${NC}$1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
}

check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "$1 is not installed. Please install it first."
        return 1
    fi
    log_success "$1 is installed"
    return 0
}

wait_for_pods() {
    local namespace=$1
    local timeout=300
    log_info "Waiting for pods in namespace $namespace to be ready..."
    kubectl wait --for=condition=ready pod --all -n $namespace --timeout=${timeout}s
}

# Banner
echo "
╔═══════════════════════════════════════════════════════╗
║                                                       ║
║   KubeRDE Quick Start for kind                       ║
║   Kubernetes Remote Development Environment          ║
║                                                       ║
╚═══════════════════════════════════════════════════════╝
"

# Prerequisites check
log_info "Checking prerequisites..."
check_command docker || exit 1
check_command kubectl || exit 1
check_command kind || {
    log_error "kind is not installed. Install it with:"
    echo "  macOS: brew install kind"
    echo "  Linux: curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind"
    exit 1
}

# Check if cluster already exists
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    log_warn "Cluster '${CLUSTER_NAME}' already exists."
    read -p "Do you want to delete and recreate it? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Deleting existing cluster..."
        kind delete cluster --name ${CLUSTER_NAME}
    else
        log_info "Using existing cluster..."
        kubectl cluster-info --context kind-${CLUSTER_NAME}
    fi
fi

# Create kind cluster if it doesn't exist
if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    log_info "Creating kind cluster '${CLUSTER_NAME}'..."

    cat <<EOF | kind create cluster --name ${CLUSTER_NAME} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
  - containerPort: 53
    hostPort: 53
    protocol: UDP
  - containerPort: 53
    hostPort: 53
    protocol: TCP
- role: worker
- role: worker
EOF

    log_success "Kind cluster created"
fi

# Configure kubectl context
kubectl cluster-info --context kind-${CLUSTER_NAME}

# Install NGINX Ingress Controller
log_info "Installing NGINX Ingress Controller..."
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

log_info "Waiting for NGINX Ingress to be ready..."
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s

log_success "NGINX Ingress Controller installed"

# Deploy KubeRDE
log_info "Deploying KubeRDE to namespace '${NAMESPACE}'..."

# Create namespace
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# Prepare manifests
export DOMAIN=${DOMAIN}
export TARGET_IP="127.0.0.1"

# Helper function to apply manifest with env substitution
apply_manifest() {
    if command -v envsubst &> /dev/null; then
        envsubst < $1 | kubectl apply -f -
    else
        sed -e "s/\${DOMAIN}/${DOMAIN}/g" -e "s/\${TARGET_IP}/${TARGET_IP}/g" $1 | kubectl apply -f -
    fi
}

# Deploy core components
log_info "Applying manifests..."
kubectl apply -f deploy/k8s/00-namespace.yaml
kubectl apply -f deploy/k8s/01-crd.yaml
kubectl apply -f deploy/k8s/02-keycloak-admin-secret.yaml
kubectl apply -f deploy/k8s/02-keycloak-realm-secret.yaml
kubectl apply -f deploy/k8s/02-keycloak.yaml
kubectl apply -f deploy/k8s/02-web.yaml
kubectl apply -f deploy/k8s/03-agent.yaml
kubectl apply -f deploy/k8s/03-server.yaml
kubectl apply -f deploy/k8s/04-operator.yaml
kubectl apply -f deploy/k8s/06-postgresql.yaml

# Apply Dev Ingress
log_info "Applying Dev Ingress..."
apply_manifest deploy/k8s/05-ingress-dev.yaml

# Deploy Local DNS
log_info "Deploying Local DNS Service..."
apply_manifest deploy/k8s/dev-dns.yaml

log_info "Waiting for KubeRDE pods to be ready..."
sleep 10
wait_for_pods ${NAMESPACE}

log_success "KubeRDE deployed successfully"

# Configure DNS Resolver (macOS/Linux)
log_info "DNS Configuration:"
if [ "$(uname)" == "Darwin" ]; then
    log_info "For macOS, to support *.${DOMAIN}, create a resolver:"
    echo "  sudo mkdir -p /etc/resolver"
    echo "  echo 'nameserver 127.0.0.1' | sudo tee /etc/resolver/${DOMAIN}"
elif [ -d "/etc/systemd/resolved.conf.d" ]; then
    log_info "For systemd-resolved, configure a link-specific DNS:"
    echo "  # Verify how to add 127.0.0.1 as DNS for ${DOMAIN}"
else
    log_info "You may need to configure your DNS manually to point *.${DOMAIN} to 127.0.0.1"
fi

# Configure /etc/hosts (Still useful for main domain if DNS resolver isn't set)
log_info "Configuring /etc/hosts..."
if ! grep -q "${DOMAIN}" /etc/hosts; then
    log_warn "Adding '127.0.0.1 ${DOMAIN}' to /etc/hosts requires sudo access"
    echo "127.0.0.1 ${DOMAIN}" | sudo tee -a /etc/hosts > /dev/null
    log_success "/etc/hosts configured"
else
    log_success "/etc/hosts already configured"
fi

# Summary
echo ""
echo "╔═══════════════════════════════════════════════════════╗
║                                                       ║
║   ✓ KubeRDE Installation Complete!                   ║
║                                                       ║
╚═══════════════════════════════════════════════════════╝"
echo ""
log_success "Cluster: kind-${CLUSTER_NAME}"
log_success "Namespace: ${NAMESPACE}"
echo ""
log_info "Access KubeRDE at:"
echo "  📱 Web UI:     http://${DOMAIN}"
echo "  🔐 Keycloak:   http://${DOMAIN}/auth/admin"
echo ""
log_info "Default credentials:"
echo "  👤 Username: admin"
echo "  🔑 Password: admin"
echo ""
log_warn "IMPORTANT: Change the default password immediately!"
echo ""
log_info "Useful commands:"
echo "  📊 Check status:  kubectl get pods -n ${NAMESPACE}"
echo "  📝 View logs:     kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=kuberde-server -f"
echo "  🗑️  Delete:        kind delete cluster --name ${CLUSTER_NAME}"
echo ""
log_info "Documentation: https://github.com/xsoloking/kube-rde/tree/main/docs"
echo ""

# Open browser (optional)
read -p "Open Web UI in browser? (Y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    sleep 2
    if command -v open &> /dev/null; then
        open http://${DOMAIN}
    elif command -v xdg-open &> /dev/null; then
        xdg-open http://${DOMAIN}
    else
        log_info "Please open http://${DOMAIN} in your browser"
    fi
fi

log_success "Happy coding with KubeRDE! 🚀"
