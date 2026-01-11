#!/usr/bin/env bash

# KubeRDE Quick Start Script for k3s + nip.io
# Zero-configuration local deployment with automatic DNS via nip.io

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
K3S_VERSION="${K3S_VERSION:-v1.28.5+k3s1}"
NAMESPACE="${NAMESPACE:-kuberde}"
INSTALL_K3S="${INSTALL_K3S:-true}"

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

get_host_ip() {
    # Try to get the primary network interface IP (not localhost)
    local ip=""

    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        ip=$(ifconfig | grep "inet " | grep -v 127.0.0.1 | head -1 | awk '{print $2}')
    else
        # Linux
        ip=$(hostname -I | awk '{print $1}')
    fi

    # Fallback to localhost if no network IP found
    if [[ -z "$ip" ]]; then
        ip="127.0.0.1"
        log_warn "Could not detect network IP, using 127.0.0.1"
    fi

    echo "$ip"
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
║   KubeRDE Quick Start with k3s + nip.io              ║
║   Zero-Configuration Local Deployment                ║
║                                                       ║
╚═══════════════════════════════════════════════════════╝
"

# Prerequisites check
log_step "Checking prerequisites..."
check_command kubectl || exit 1

# Get host IP
HOST_IP=$(get_host_ip)
log_success "Detected host IP: ${HOST_IP}"

# Convert IP to nip.io format (dots to dashes)
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

# Install k3s if needed
if [[ "$INSTALL_K3S" == "true" ]]; then
    if command -v k3s &> /dev/null; then
        log_warn "k3s is already installed"
        read -p "Reinstall k3s? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_step "Uninstalling existing k3s..."
            if [[ -f /usr/local/bin/k3s-uninstall.sh ]]; then
                sudo /usr/local/bin/k3s-uninstall.sh
            fi
        else
            log_info "Using existing k3s installation"
            INSTALL_K3S="false"
        fi
    fi

    if [[ "$INSTALL_K3S" == "true" ]]; then
        log_step "Installing k3s ${K3S_VERSION}..."

        # Install k3s with Traefik disabled (we'll configure it ourselves)
        curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="${K3S_VERSION}" sh -s - \
            --write-kubeconfig-mode 644 \
            --disable traefik \
            --disable servicelb

        # Wait for k3s to be ready
        log_info "Waiting for k3s to be ready..."
        sleep 10

        # Set up kubeconfig
        export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
        mkdir -p ~/.kube
        sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
        sudo chown $(id -u):$(id -g) ~/.kube/config

        log_success "k3s installed successfully"
    fi
else
    log_info "Skipping k3s installation"
fi

# Configure kubectl
if [[ -f /etc/rancher/k3s/k3s.yaml ]]; then
    export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
    if [[ ! -f ~/.kube/config ]]; then
        mkdir -p ~/.kube
        sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
        sudo chown $(id -u):$(id -g) ~/.kube/config
    fi
fi

# Verify cluster
log_step "Verifying cluster connectivity..."
kubectl cluster-info
log_success "Cluster is accessible"

# Install Traefik Ingress Controller
log_step "Installing Traefik Ingress Controller..."

# Create Traefik namespace
kubectl create namespace traefik --dry-run=client -o yaml | kubectl apply -f -

# Install Traefik via Helm (if available) or manifests
if command -v helm &> /dev/null; then
    log_info "Using Helm to install Traefik..."
    helm repo add traefik https://traefik.github.io/charts 2>/dev/null || true
    helm repo update

    helm upgrade --install traefik traefik/traefik \
        --namespace traefik \
        --set ports.web.hostPort=80 \
        --set ports.websecure.hostPort=443 \
        --set service.type=NodePort \
        --wait \
        --timeout=5m
else
    log_info "Installing Traefik using manifests..."
    kubectl apply -f https://raw.githubusercontent.com/traefik/traefik/v2.10/docs/content/reference/dynamic-configuration/kubernetes-crd-definition-v1.yml
    kubectl apply -f https://raw.githubusercontent.com/traefik/traefik/v2.10/docs/content/reference/dynamic-configuration/kubernetes-crd-rbac.yml
fi

log_info "Waiting for Traefik to be ready..."
kubectl wait --namespace traefik \
    --for=condition=ready pod \
    --selector=app.kubernetes.io/name=traefik \
    --timeout=90s 2>/dev/null || log_warn "Traefik may still be starting..."

log_success "Traefik Ingress Controller installed"

# Deploy KubeRDE
log_step "Deploying KubeRDE to namespace '${NAMESPACE}'..."

# Create namespace
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# Check if we should use Helm or direct YAML
if command -v helm &> /dev/null && [ -d "./charts/kuberde" ]; then
    log_info "Deploying KubeRDE using Helm..."
    helm upgrade --install kuberde ./charts/kuberde \
        --namespace ${NAMESPACE} \
        --set global.domain=${DOMAIN} \
        --set global.publicUrl=${PUBLIC_URL} \
        --set server.env.KUBERDE_PUBLIC_URL=${PUBLIC_URL} \
        --set server.env.KUBERDE_AGENT_DOMAIN=${DOMAIN} \
        --set keycloak.domain=${KEYCLOAK_DOMAIN} \
        --set keycloak.url=${KEYCLOAK_URL} \
        --set ingress.className=traefik \
        --set ingress.tls.enabled=false \
        --wait \
        --timeout=10m
else
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

    # Apply base resources (namespace, CRD, PostgreSQL)
    kubectl apply -f deploy/k8s/00-namespace.yaml
    kubectl apply -f deploy/k8s/01-crd.yaml
    kubectl apply -f deploy/k8s/06-postgresql.yaml -n ${NAMESPACE}

    # Process and apply secrets
    for secret_file in deploy/k8s/02-*-secret.yaml; do
        if [[ -f "$secret_file" ]]; then
            kubectl apply -f "$secret_file" -n ${NAMESPACE}
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
fi

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
log_success "Cluster: k3s"
log_success "Namespace: ${NAMESPACE}"
log_success "Host IP: ${HOST_IP}"
echo ""
log_info "Access KubeRDE at:"
echo "  📱 Web UI:        ${PUBLIC_URL}"
echo "  🔐 Keycloak:      ${KEYCLOAK_URL}/auth/admin"
echo ""
log_info "Agent URLs (examples):"
echo "  🤖 user-alice-dev:  http://user-alice-dev.${DOMAIN}"
echo "  🤖 user-bob-jupyter: http://user-bob-jupyter.${DOMAIN}"
echo ""
log_info "Default credentials:"
echo "  👤 Username: admin"
echo "  🔑 Password: admin"
echo ""
log_warn "IMPORTANT: Change the default password immediately!"
echo ""
log_info "Useful commands:"
echo "  📊 Check status:       kubectl get pods -n ${NAMESPACE}"
echo "  📝 View server logs:   kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=kuberde-server -f"
echo "  🔄 Restart server:     kubectl rollout restart deployment/kuberde-server -n ${NAMESPACE}"
echo "  ⏸️  Stop k3s:          sudo systemctl stop k3s"
echo "  🗑️  Uninstall k3s:     sudo /usr/local/bin/k3s-uninstall.sh"
echo ""
log_info "DNS Details (nip.io):"
echo "  ✓ No local DNS configuration needed!"
echo "  ✓ Wildcard domains automatically work"
echo "  ✓ Works from any device on your network"
echo "  ✓ Main:     ${DOMAIN} → ${HOST_IP}"
echo "  ✓ Keycloak: ${KEYCLOAK_DOMAIN} → ${HOST_IP}"
echo "  ✓ Agents:   *.${DOMAIN} → ${HOST_IP}"
echo ""
log_info "Why k3s?"
echo "  ⚡ Production-grade lightweight Kubernetes"
echo "  💻 Installed directly on your system"
echo "  🔒 Persistent across reboots"
echo "  🌐 Accessible from network (not just localhost)"
echo "  📦 Single binary (~70MB)"
echo "  🎯 Perfect for edge, IoT, CI/CD"
echo ""
log_warn "Note: Requires sudo for installation/uninstallation"
echo ""
log_info "Documentation: https://github.com/xsoloking/kube-rde/tree/main/docs"
echo ""

# Test DNS resolution
log_step "Testing nip.io DNS resolution..."
if ping -c 1 -W 2 ${DOMAIN} &> /dev/null; then
    log_success "DNS resolution working: ${DOMAIN} → ${HOST_IP}"
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
