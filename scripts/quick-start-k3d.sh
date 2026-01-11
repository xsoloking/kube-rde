#!/usr/bin/env bash

# KubeRDE Quick Start Script for k3d + nip.io
# Zero-configuration local deployment with k3s in Docker

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-kuberde}"
K3S_VERSION="${K3S_VERSION:-v1.33.6+k3s1}"
NAMESPACE="${NAMESPACE:-kuberde}"
WORKERS="${WORKERS:-2}"

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

log_step() {
    echo -e "${CYAN}▶${NC} $1"
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
    kubectl wait --for=condition=ready pod --all -n $namespace --timeout=${timeout}s 2>/dev/null || {
        log_warn "Some pods may still be starting..."
        return 0
    }
}

# Banner
echo "
╔═══════════════════════════════════════════════════════╗
║                                                       ║
║   KubeRDE Quick Start with k3d + nip.io              ║
║   k3s in Docker - Zero Configuration                 ║
║                                                       ║
╚═══════════════════════════════════════════════════════╝
"

# Prerequisites check
log_step "Checking prerequisites..."
check_command docker || exit 1
check_command kubectl || exit 1
check_command k3d || {
    log_error "k3d is not installed. Install it with:"
    echo "  macOS:   brew install k3d"
    echo "  Linux:   curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash"
    echo "  Windows: choco install k3d"
    exit 1
}

# Get host IP (use 127.0.0.1 for k3d as it runs in Docker)
HOST_IP="192.168.97.2"
log_success "Using localhost: ${HOST_IP}"

# Convert IP to nip.io format
NIP_IO_IP="${HOST_IP//./-}"

# Configure domains with nip.io
DOMAIN="${NIP_IO_IP}.nip.io"
KEYCLOAK_DOMAIN="sso.${NIP_IO_IP}.nip.io"
PUBLIC_URL="http://${DOMAIN}"
KEYCLOAK_URL="http://${KEYCLOAK_DOMAIN}"

echo ""
log_info "DNS Configuration (via nip.io):"
echo "  📱 Main Domain:     ${DOMAIN}"
echo "  🔐 Keycloak Domain: ${KEYCLOAK_DOMAIN}"
echo "  🤖 Agent Pattern:   *.${DOMAIN}"
echo "  🌐 Public URL:      ${PUBLIC_URL}"
echo ""

# Check if cluster already exists
if k3d cluster list | grep -q "^${CLUSTER_NAME}"; then
    log_warn "Cluster '${CLUSTER_NAME}' already exists."
    read -p "Do you want to delete and recreate it? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_step "Deleting existing cluster..."
        k3d cluster delete ${CLUSTER_NAME}
    else
        log_info "Using existing cluster..."
        kubectl cluster-info --context k3d-${CLUSTER_NAME}
    fi
fi

# Create k3d cluster if it doesn't exist
if ! k3d cluster list | grep -q "^${CLUSTER_NAME}"; then
    log_step "Creating k3d cluster '${CLUSTER_NAME}'..."

    # Create cluster with port mappings for HTTP/HTTPS
    k3d cluster create ${CLUSTER_NAME} \
        --agents ${WORKERS} \
        --wait

    log_success "k3d cluster created"
fi

# Deploy KubeRDE
log_step "Deploying KubeRDE to namespace '${NAMESPACE}'..."

# Create namespace
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# Check if we should use Helm or direct YAML
# if command -v helm &> /dev/null && [ -d "./charts/kuberde" ]; then
#     log_info "Deploying KubeRDE using Helm..."
#     helm upgrade --install kuberde ./charts/kuberde \
#         --namespace ${NAMESPACE} \
#         --set global.domain=${DOMAIN} \
#         --set global.publicUrl=${PUBLIC_URL} \
#         --set server.env.KUBERDE_PUBLIC_URL=${PUBLIC_URL} \
#         --set server.env.KUBERDE_AGENT_DOMAIN=${DOMAIN} \
#         --set keycloak.domain=${KEYCLOAK_DOMAIN} \
#         --set keycloak.url=${KEYCLOAK_URL} \
#         --set ingress.className=traefik \
#         --set ingress.tls.enabled=false \
#         --wait \
#         --timeout=10m
# else
    log_info "Deploying KubeRDE using Kubernetes manifests..."

    # Create temporary directory for processed manifests
    TEMP_MANIFESTS=$(mktemp -d)
    trap "rm -rf ${TEMP_MANIFESTS}" EXIT

    log_info "Processing manifests with nip.io configuration..."

    # Export variables for envsubst
    export KUBERDE_DOMAIN=${DOMAIN}
    export KUBERDE_PUBLIC_URL=${PUBLIC_URL}
    export KUBERDE_AGENT_DOMAIN=${DOMAIN}
    export KEYCLOAK_DOMAIN=${KEYCLOAK_DOMAIN}
    export KEYCLOAK_URL=${KEYCLOAK_URL}
    export POSTGRES_PASSWORD=kuberde
    export DB_USER=kuberde
    export DB_PASSWORD=kuberde

    # Apply base resources (namespace, CRD, PostgreSQL)
    kubectl apply -f deploy/k8s/00-namespace.yaml
    kubectl apply -f deploy/k8s/01-crd.yaml
    envsubst < deploy/k8s/06-postgresql.yaml > ${TEMP_MANIFESTS}/postgresql.yaml
    kubectl apply -f ${TEMP_MANIFESTS}/postgresql.yaml -n ${NAMESPACE}

    # Process and apply secrets
    for secret_file in deploy/k8s/02-*-secret.yaml; do
        if [[ -f "$secret_file" ]]; then
            envsubst < "$secret_file" > ${TEMP_MANIFESTS}/$(basename "$secret_file")    
            kubectl apply -f ${TEMP_MANIFESTS}/$(basename "$secret_file") -n ${NAMESPACE}
        fi
    done

    # Process k3s-specific configurations
    # Substitute environment variables in YAML files
    log_info "Configuring Keycloak for ${KEYCLOAK_DOMAIN}..."
    envsubst < deploy/k8s/02-keycloak-k3s.yaml > ${TEMP_MANIFESTS}/keycloak.yaml
    kubectl apply -f ${TEMP_MANIFESTS}/keycloak.yaml -n ${NAMESPACE}

    log_info "Configuring Server for ${DOMAIN}..."
    envsubst < deploy/k8s/03-server-k3s.yaml > ${TEMP_MANIFESTS}/server.yaml
    kubectl apply -f ${TEMP_MANIFESTS}/server.yaml -n ${NAMESPACE}

    # Apply other components
    kubectl apply -f deploy/k8s/02-web.yaml -n ${NAMESPACE}
    kubectl apply -f deploy/k8s/04-operator.yaml -n ${NAMESPACE}

    # Process and apply ingress with environment variables
    log_info "Configuring Traefik Ingress for nip.io domains..."
    envsubst < deploy/k8s/05-ingress-k3s.yaml > ${TEMP_MANIFESTS}/ingress.yaml
    kubectl apply -f ${TEMP_MANIFESTS}/ingress.yaml -n ${NAMESPACE}

    log_success "Manifests applied successfully"
# fi

log_info "Waiting for KubeRDE pods to be ready..."
sleep 10
wait_for_pods ${NAMESPACE}

log_success "KubeRDE deployed successfully"

# Summary
echo ""
echo "╔═══════════════════════════════════════════════════════╗
║                                                       ║
║   ✓ KubeRDE Installation Complete!                   ║
║                                                       ║
╚═══════════════════════════════════════════════════════╝"
echo ""
log_success "Cluster: k3d-${CLUSTER_NAME}"
log_success "Namespace: ${NAMESPACE}"
log_success "Workers: ${WORKERS}"
echo ""
log_info "Access KubeRDE at:"
echo "  📱 Web UI:        ${PUBLIC_URL}"
echo "  🔐 Keycloak:      ${KEYCLOAK_URL}/auth/admin"
echo ""
log_info "Agent URLs (examples):"
echo "  🤖 user-alice-dev:   http://user-alice-dev.${DOMAIN}"
echo "  🤖 user-bob-jupyter: http://user-bob-jupyter.${DOMAIN}"
echo ""
log_info "Default credentials:"
echo "  👤 Username: admin"
echo "  🔑 Password: admin"
echo ""
log_warn "IMPORTANT: Change the default password immediately!"
echo ""
log_info "Useful commands:"
echo "  📊 Check status:     kubectl get pods -n ${NAMESPACE}"
echo "  📝 View server logs: kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=kuberde-server -f"
echo "  🔄 Restart server:   kubectl rollout restart deployment/kuberde-server -n ${NAMESPACE}"
echo "  📋 Cluster info:     k3d cluster list"
echo "  ⏸️  Stop cluster:     k3d cluster stop ${CLUSTER_NAME}"
echo "  ▶️  Start cluster:    k3d cluster start ${CLUSTER_NAME}"
echo "  🗑️  Delete cluster:   k3d cluster delete ${CLUSTER_NAME}"
echo ""
log_info "DNS Details (nip.io):"
echo "  ✓ No local DNS configuration needed!"
echo "  ✓ Wildcard domains automatically work"
echo "  ✓ All traffic routes through localhost"
echo "  ✓ Main:     ${DOMAIN} → 127.0.0.1"
echo "  ✓ Keycloak: ${KEYCLOAK_DOMAIN} → 127.0.0.1"
echo "  ✓ Agents:   *.${DOMAIN} → 127.0.0.1"
echo ""
log_info "Why k3d?"
echo "  🐳 Runs k3s in Docker (easy to manage)"
echo "  🚀 Fast cluster creation (~30 seconds)"
echo "  🧹 Clean removal (just delete containers)"
echo "  🔄 Multi-cluster support"
echo "  💾 Persistent (survives system restart)"
echo ""
log_info "Documentation: https://github.com/xsoloking/kube-rde/tree/main/docs"
echo ""

# Test DNS resolution
log_step "Testing nip.io DNS resolution..."
if ping -c 1 -W 2 ${DOMAIN} &> /dev/null; then
    log_success "DNS resolution working: ${DOMAIN} → 127.0.0.1"
else
    log_warn "DNS resolution test failed - check internet connection"
    log_info "nip.io requires internet connectivity"
fi

# Open browser (optional)
read -p "Open Web UI in browser? (Y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    sleep 2
    if command -v open &> /dev/null; then
        open ${PUBLIC_URL}
    elif command -v xdg-open &> /dev/null; then
        xdg-open ${PUBLIC_URL}
    else
        log_info "Please open ${PUBLIC_URL} in your browser"
    fi
fi

log_success "Happy coding with KubeRDE! 🚀"
