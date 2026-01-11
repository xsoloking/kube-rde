#!/usr/bin/env bash

# KubeRDE Universal Installer
# Detects your environment and installs KubeRDE accordingly

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}ℹ ${NC}$1"; }
log_success() { echo -e "${GREEN}✓${NC} $1"; }
log_warn() { echo -e "${YELLOW}⚠${NC} $1"; }
log_error() { echo -e "${RED}✗${NC} $1"; }

# Banner
cat << "EOF"
╔═══════════════════════════════════════════════════════╗
║                                                       ║
║   ██╗  ██╗██╗   ██╗██████╗ ███████╗██████╗ ██████╗ ███████╗
║   ██║ ██╔╝██║   ██║██╔══██╗██╔════╝██╔══██╗██╔══██╗██╔════╝
║   █████╔╝ ██║   ██║██████╔╝█████╗  ██████╔╝██║  ██║█████╗
║   ██╔═██╗ ██║   ██║██╔══██╗██╔══╝  ██╔══██╗██║  ██║██╔══╝
║   ██║  ██╗╚██████╔╝██████╔╝███████╗██║  ██║██████╔╝███████╗
║   ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚══════╝╚═╝  ╚═╝╚═════╝ ╚══════╝
║                                                       ║
║   Kubernetes Remote Development Environment          ║
║                                                       ║
╚═══════════════════════════════════════════════════════╝
EOF

echo ""
log_info "Welcome to KubeRDE Installer!"
echo ""

# Detect environment
detect_environment() {
    log_info "Detecting your environment..."

    # Check if running in cloud
    if curl -s -m 2 http://169.254.169.254/latest/meta-data/ > /dev/null 2>&1; then
        echo "aws"
        return
    elif curl -s -m 2 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/ > /dev/null 2>&1; then
        echo "gcp"
        return
    elif curl -s -m 2 -H "Metadata: true" http://169.254.169.254/metadata/instance > /dev/null 2>&1; then
        echo "azure"
        return
    fi

    # Check for local Kubernetes
    if command -v kubectl &> /dev/null; then
        local context=$(kubectl config current-context 2>/dev/null || echo "")

        if [[ $context == kind-* ]]; then
            echo "kind"
            return
        elif [[ $context == minikube* ]]; then
            echo "minikube"
            return
        elif [[ $context == *gke* ]]; then
            echo "gke"
            return
        elif [[ $context == *eks* ]]; then
            echo "eks"
            return
        elif [[ $context == *aks* ]]; then
            echo "aks"
            return
        fi
    fi

    # Default to local
    echo "local"
}

# Check prerequisites
check_prereqs() {
    local missing=0

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        missing=1
    fi

    if [[ $ENV == "local" ]]; then
        if ! command -v kind &> /dev/null && ! command -v minikube &> /dev/null; then
            log_error "Neither kind nor minikube is installed"
            log_info "Install one of them:"
            echo "  kind:     https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
            echo "  minikube: https://minikube.sigs.k8s.io/docs/start/"
            missing=1
        fi
    fi

    return $missing
}

# Install based on environment
install_local() {
    log_info "Installing KubeRDE locally..."

    if command -v kind &> /dev/null; then
        log_info "Using kind for local installation"
        if [ -f "./scripts/quick-start-kind.sh" ]; then
            bash ./scripts/quick-start-kind.sh
        else
            log_error "quick-start-kind.sh not found"
            log_info "Please run this script from the kube-rde repository root"
            exit 1
        fi
    elif command -v minikube &> /dev/null; then
        log_info "Using minikube for local installation"
        if [ -f "./scripts/quick-start-minikube.sh" ]; then
            bash ./scripts/quick-start-minikube.sh
        else
            log_error "quick-start-minikube.sh not found"
            log_info "Please run this script from the kube-rde repository root"
            exit 1
        fi
    else
        log_error "No local Kubernetes solution found"
        exit 1
    fi
}

install_cloud() {
    local platform=$1
    log_info "Detected cloud platform: $platform"
    log_info "For cloud deployments, please follow our detailed guides:"

    case $platform in
        gke|gcp)
            echo "  📘 GCP/GKE Guide: docs/platforms/gcp-gke.md"
            echo "  🚀 Quick: cd terraform/gcp/complete && terraform apply"
            ;;
        eks|aws)
            echo "  📘 AWS/EKS Guide: docs/platforms/aws-eks.md"
            echo "  🚀 Quick: cd terraform/aws/complete && terraform apply"
            ;;
        aks|azure)
            echo "  📘 Azure/AKS Guide: docs/platforms/azure-aks.md"
            echo "  🚀 Quick: cd terraform/azure/complete && terraform apply"
            ;;
    esac

    echo ""
    read -p "Do you want to proceed with manual installation? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        install_generic
    else
        log_info "Please follow the platform-specific guide for best results"
        exit 0
    fi
}

install_generic() {
    log_info "Installing KubeRDE to current Kubernetes context..."

    # Check if cluster is accessible
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        log_info "Please ensure kubectl is configured correctly"
        exit 1
    fi

    # Get domain
    read -p "Enter your domain name (e.g., kuberde.example.com): " DOMAIN
    if [ -z "$DOMAIN" ]; then
        DOMAIN="kuberde.local"
        log_warn "Using default domain: $DOMAIN"
    fi

    # Create namespace
    NAMESPACE="kuberde"
    kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    # Install using Helm if available
    if command -v helm &> /dev/null && [ -d "./charts/kuberde" ]; then
        log_info "Installing KubeRDE using Helm..."

        helm upgrade --install kuberde ./charts/kuberde \
            --namespace $NAMESPACE \
            --set global.domain=$DOMAIN \
            --wait \
            --timeout=10m

        log_success "KubeRDE installed successfully!"
    else
        # Fallback to kubectl apply
        log_info "Installing KubeRDE using kubectl..."

        if [ ! -d "./deploy/k8s" ]; then
            log_error "Kubernetes manifests not found"
            log_info "Please run this script from the kube-rde repository root"
            exit 1
        fi

        kubectl apply -f ./deploy/k8s/ -n $NAMESPACE

        log_info "Waiting for pods to be ready..."
        kubectl wait --for=condition=ready pod --all -n $NAMESPACE --timeout=300s

        log_success "KubeRDE installed successfully!"
    fi

    # Show access info
    echo ""
    log_info "To access KubeRDE:"
    echo "  1. Get your Ingress IP:"
    echo "     kubectl get svc -n ingress-nginx"
    echo "  2. Configure DNS:"
    echo "     $DOMAIN -> <Ingress-IP>"
    echo "  3. Access: http://$DOMAIN"
    echo ""
}

# Main installation flow
main() {
    ENV=$(detect_environment)
    log_success "Environment detected: $ENV"
    echo ""

    # Check prerequisites
    if ! check_prereqs; then
        log_error "Prerequisites check failed"
        exit 1
    fi

    # Install based on environment
    case $ENV in
        kind|minikube|local)
            install_local
            ;;
        gke|gcp|eks|aws|aks|azure)
            install_cloud $ENV
            ;;
        *)
            log_warn "Unknown environment, proceeding with generic installation"
            install_generic
            ;;
    esac
}

# Run main function
main
