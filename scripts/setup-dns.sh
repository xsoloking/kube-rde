#!/usr/bin/env bash

# DNS Setup Script for KubeRDE Local Development
# Supports multiple DNS solutions for wildcard domain resolution

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

DOMAIN="${DOMAIN:-kuberde.local}"
IP_ADDRESS="${IP_ADDRESS:-127.0.0.1}"

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

# Banner
echo "
╔═══════════════════════════════════════════════════════╗
║                                                       ║
║   KubeRDE DNS Setup for Local Development           ║
║                                                       ║
╚═══════════════════════════════════════════════════════╝
"

log_info "Domain: ${DOMAIN}"
log_info "IP Address: ${IP_ADDRESS}"
echo ""

# Detect OS
if [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
else
    log_error "Unsupported OS: $OSTYPE"
    exit 1
fi

log_info "Detected OS: $OS"
echo ""

# DNS Solution Menu
echo "Select DNS solution:"
echo "  1) dnsmasq (Recommended - supports wildcards)"
echo "  2) /etc/hosts (Simple - no wildcards)"
echo "  3) nip.io/sslip.io (External service - requires internet)"
echo "  4) CoreDNS in Kubernetes (Advanced - for kind/minikube)"
echo ""
read -p "Enter choice [1-4]: " choice

case $choice in
    1)
        log_info "Setting up dnsmasq..."

        if [[ "$OS" == "macos" ]]; then
            # macOS setup
            if ! command -v dnsmasq &> /dev/null; then
                log_info "Installing dnsmasq via Homebrew..."
                brew install dnsmasq
            fi

            # Configure dnsmasq
            DNSMASQ_CONF="/usr/local/etc/dnsmasq.conf"
            if [[ -f "$DNSMASQ_CONF" ]]; then
                sudo cp "$DNSMASQ_CONF" "$DNSMASQ_CONF.backup"
            fi

            log_info "Configuring dnsmasq..."
            echo "address=/.${DOMAIN}/${IP_ADDRESS}" | sudo tee -a "$DNSMASQ_CONF" > /dev/null

            # Start dnsmasq
            sudo brew services start dnsmasq

            # Configure macOS resolver
            sudo mkdir -p /etc/resolver
            echo "nameserver 127.0.0.1" | sudo tee "/etc/resolver/${DOMAIN}" > /dev/null

            log_success "dnsmasq configured and started"
            log_info "Testing DNS resolution..."
            sleep 2
            if ping -c 1 test.${DOMAIN} &> /dev/null; then
                log_success "DNS resolution working!"
            else
                log_warn "DNS resolution test failed - you may need to restart your network"
            fi

        elif [[ "$OS" == "linux" ]]; then
            # Linux setup
            if ! command -v dnsmasq &> /dev/null; then
                log_info "Installing dnsmasq..."
                if command -v apt-get &> /dev/null; then
                    sudo apt-get update && sudo apt-get install -y dnsmasq
                elif command -v yum &> /dev/null; then
                    sudo yum install -y dnsmasq
                else
                    log_error "Could not install dnsmasq - please install manually"
                    exit 1
                fi
            fi

            # Configure dnsmasq
            DNSMASQ_CONF="/etc/dnsmasq.d/kuberde"
            log_info "Configuring dnsmasq..."
            echo "address=/.${DOMAIN}/${IP_ADDRESS}" | sudo tee "$DNSMASQ_CONF" > /dev/null

            # Restart dnsmasq
            sudo systemctl restart dnsmasq
            sudo systemctl enable dnsmasq

            # Configure NetworkManager (if present)
            if command -v nmcli &> /dev/null; then
                log_info "Configuring NetworkManager..."
                echo -e "[main]\ndns=dnsmasq" | sudo tee /etc/NetworkManager/conf.d/dnsmasq.conf > /dev/null
                sudo systemctl restart NetworkManager
            fi

            log_success "dnsmasq configured and started"
        fi

        log_success "Wildcard DNS setup complete!"
        log_info "All *.${DOMAIN} domains will resolve to ${IP_ADDRESS}"
        ;;

    2)
        log_info "Setting up /etc/hosts..."
        log_warn "Note: This does NOT support wildcard domains"
        log_warn "You'll need to add each agent subdomain manually"

        if ! grep -q "${DOMAIN}" /etc/hosts; then
            echo "${IP_ADDRESS} ${DOMAIN} www.${DOMAIN}" | sudo tee -a /etc/hosts > /dev/null
            log_success "/etc/hosts updated"
        else
            log_warn "/etc/hosts already contains ${DOMAIN}"
        fi

        log_info "To add agent subdomains, run:"
        echo "  echo '${IP_ADDRESS} user-alice-dev.${DOMAIN}' | sudo tee -a /etc/hosts"
        ;;

    3)
        log_info "Using nip.io/sslip.io external DNS service..."
        log_warn "This requires internet connectivity"

        if [[ "$IP_ADDRESS" == "127.0.0.1" ]]; then
            NEW_DOMAIN="127.0.0.1.nip.io"
        else
            NEW_DOMAIN="${IP_ADDRESS//./-}.nip.io"
        fi

        log_success "Use domain: ${NEW_DOMAIN}"
        log_info "Update your deployment with:"
        echo "  export DOMAIN=${NEW_DOMAIN}"
        echo "  export KUBERDE_PUBLIC_URL=http://${NEW_DOMAIN}"
        echo "  export KUBERDE_AGENT_DOMAIN=${NEW_DOMAIN}"

        log_info "Testing DNS resolution..."
        if ping -c 1 test.${NEW_DOMAIN} &> /dev/null; then
            log_success "DNS resolution working!"
        else
            log_warn "DNS resolution test failed - check internet connection"
        fi
        ;;

    4)
        log_info "CoreDNS in Kubernetes setup..."
        log_info "This deploys CoreDNS inside your cluster"

        if ! command -v kubectl &> /dev/null; then
            log_error "kubectl not found - please install it first"
            exit 1
        fi

        # Update CoreDNS config with correct IP
        TEMP_FILE=$(mktemp)
        cat > "$TEMP_FILE" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: kuberde-dns
  namespace: kuberde
data:
  Corefile: |
    .:53 {
        errors
        health
        ready
        template IN A ${DOMAIN} {
            match ".*\.${DOMAIN}\."
            answer "{{ .Name }} 60 IN A ${IP_ADDRESS}"
            fallthrough
        }
        forward . /etc/resolv.conf
        cache 30
        loop
        reload
    }
EOF

        log_info "Applying CoreDNS configuration..."
        kubectl apply -f "$TEMP_FILE"
        kubectl apply -f "$(dirname "$0")/../deploy/k8s/dev-dns.yaml"
        rm "$TEMP_FILE"

        log_success "CoreDNS deployed to Kubernetes"
        log_info "Configure your system to use it:"
        echo "  macOS: System Preferences > Network > Advanced > DNS"
        echo "  Linux: Edit /etc/resolv.conf"
        echo "  Add nameserver: 127.0.0.1"

        log_warn "You may need to configure kubectl port-forward:"
        echo "  kubectl port-forward -n kuberde svc/kuberde-dns 53:53"
        ;;

    *)
        log_error "Invalid choice"
        exit 1
        ;;
esac

echo ""
log_success "DNS setup complete!"
echo ""
log_info "Verify DNS resolution:"
echo "  dig @localhost ${DOMAIN}"
echo "  dig @localhost test.${DOMAIN}"
echo "  ping test.${DOMAIN}"
echo ""
