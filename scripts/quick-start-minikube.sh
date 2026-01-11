#!/usr/bin/env bash

# KubeRDE Quick Start Script for minikube
# This script sets up a complete KubeRDE environment on your local machine using minikube

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROFILE="${PROFILE:-kuberde}"
DOMAIN="${DOMAIN:-kuberde.local}"
NAMESPACE="${NAMESPACE:-kuberde}"
CPUS="${CPUS:-4}"
MEMORY="${MEMORY:-8192}"
DRIVER="${DRIVER:-docker}"

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

# Banner
echo "
╔═══════════════════════════════════════════════════════╗
║                                                       ║
║   KubeRDE Quick Start for minikube                   ║
║   Kubernetes Remote Development Environment          ║
║                                                       ║
╚═══════════════════════════════════════════════════════╝
"

# Prerequisites check
log_info "Checking prerequisites..."
check_command kubectl || exit 1
check_command minikube || {
    log_error "minikube is not installed. Install it with:"
    echo "  macOS: brew install minikube"
    echo "  Linux: curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64 && sudo install minikube-linux-amd64 /usr/local/bin/minikube"
    exit 1
}

# Check if profile exists
if minikube profile list 2>/dev/null | grep -q "${PROFILE}"; then
    log_warn "Minikube profile '${PROFILE}' already exists."
    read -p "Do you want to delete and recreate it? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Deleting existing profile..."
        minikube delete --profile ${PROFILE}
    else
        log_info "Using existing profile..."
        minikube start --profile ${PROFILE}
    fi
fi

# Start minikube if not running
if ! minikube status --profile ${PROFILE} 2>/dev/null | grep -q "host: Running"; then
    log_info "Starting minikube profile '${PROFILE}'..."
    log_info "  CPUs: ${CPUS}"
    log_info "  Memory: ${MEMORY}MB"
    log_info "  Driver: ${DRIVER}"

    minikube start \
        --profile ${PROFILE} \
        --cpus=${CPUS} \
        --memory=${MEMORY} \
        --disk-size=40g \
        --driver=${DRIVER} \
        --kubernetes-version=stable

    log_success "Minikube started"
fi

# Configure kubectl context
kubectl config use-context ${PROFILE}

# Enable addons
log_info "Enabling minikube addons..."
minikube addons enable ingress --profile ${PROFILE}
minikube addons enable metrics-server --profile ${PROFILE}
log_success "Addons enabled"

# Wait for Ingress to be ready
log_info "Waiting for Ingress controller..."
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s 2>/dev/null || log_warn "Ingress controller might still be starting..."

# Deploy KubeRDE
log_info "Deploying KubeRDE to namespace '${NAMESPACE}'..."

# Create namespace
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# Get minikube IP
MINIKUBE_IP=$(minikube ip --profile ${PROFILE})
log_info "Minikube IP: ${MINIKUBE_IP}"

export DOMAIN=${DOMAIN}
export TARGET_IP=${MINIKUBE_IP}

# Helper function to apply manifest with env substitution
apply_manifest() {
    if command -v envsubst &> /dev/null; then
        envsubst < $1 | kubectl apply -f -
    else
        sed -e "s/\${DOMAIN}/${DOMAIN}/g" -e "s/\${TARGET_IP}/${TARGET_IP}/g" $1 | kubectl apply -f -
    fi
}

# Deploy components
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
log_info "Deploying Local DNS Service (NodePort)..."
# Note: Minikube doesn't map host ports automatically like kind extraPortMappings.
# We deploy it anyway, but the user might need 'minikube service' or tunnel to access it.
apply_manifest deploy/k8s/dev-dns.yaml

log_info "Waiting for KubeRDE pods to be ready..."
sleep 10
kubectl wait --for=condition=ready pod --all -n ${NAMESPACE} --timeout=300s

log_success "KubeRDE deployed successfully"

# Configure /etc/hosts
log_info "Configuring /etc/hosts..."
if ! grep -q "${DOMAIN}" /etc/hosts; then
    log_warn "Adding '${MINIKUBE_IP} ${DOMAIN}' to /etc/hosts requires sudo access"
    # Remove old entries first
    sudo sed -i.bak "/${DOMAIN}/d" /etc/hosts 2>/dev/null || true
    # Add new entry
    echo "${MINIKUBE_IP} ${DOMAIN}" | sudo tee -a /etc/hosts > /dev/null
    log_success "/etc/hosts configured"
else
    # Update IP if changed
    if ! grep "${MINIKUBE_IP}.*${DOMAIN}" /etc/hosts; then
        log_warn "Updating ${DOMAIN} IP in /etc/hosts"
        sudo sed -i.bak "s/.*${DOMAIN}/${MINIKUBE_IP} ${DOMAIN}/" /etc/hosts
    fi
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
log_success "Profile: ${PROFILE}"
log_success "Namespace: ${NAMESPACE}"
log_success "IP: ${MINIKUBE_IP}"
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
echo "  📊 Check status:      kubectl get pods -n ${NAMESPACE}"
echo "  📝 View logs:         kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=kuberde-server -f"
echo "  🖥️  Minikube dashboard: minikube dashboard --profile ${PROFILE}"
echo "  ⏸️  Stop minikube:     minikube stop --profile ${PROFILE}"
echo "  🗑️  Delete:            minikube delete --profile ${PROFILE}"
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
