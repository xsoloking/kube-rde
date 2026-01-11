# DNS Setup for KubeRDE Local Development

This guide explains how to set up DNS for local KubeRDE development. KubeRDE requires **wildcard DNS** to support dynamic agent subdomains (e.g., `user-alice-dev.kuberde.local`).

## Why Wildcard DNS?

KubeRDE uses a multi-tenant architecture where each agent gets its own subdomain:

- **Main domain**: `kuberde.local` - Web UI and API
- **Agent domains**: `*.kuberde.local` - Each agent gets a unique subdomain
  - `user-alice-dev.kuberde.local`
  - `user-bob-jupyter.kuberde.local`
  - etc.

Traditional `/etc/hosts` **cannot** handle wildcard patterns (`*.kuberde.local`), so we need a proper DNS solution.

## Quick Start

The quick-start scripts (`quick-start-kind.sh` and `quick-start-minikube.sh`) now include **automatic DNS setup**. Simply run the script and choose your preferred DNS method when prompted.

```bash
# For kind
./scripts/quick-start-kind.sh

# For minikube
./scripts/quick-start-minikube.sh
```

You'll be presented with these options:

1. **dnsmasq** (Recommended) - Full wildcard support
2. **CoreDNS in Kubernetes** - Cluster-based DNS
3. **nip.io** - No setup required, uses external service
4. **Manual /etc/hosts** - Simple but no wildcards
5. **Skip** - Configure DNS yourself

## DNS Solutions Comparison

| Solution | Wildcards | Setup Difficulty | Best For |
|----------|-----------|-----------------|----------|
| **dnsmasq** | ✅ Yes | Medium | macOS/Linux development |
| **CoreDNS** | ✅ Yes | Medium | Kubernetes-native setup |
| **nip.io** | ✅ Yes | Easy (no setup) | Quick testing, internet required |
| **/etc/hosts** | ❌ No | Easy | Single domain only |

## Option 1: dnsmasq (Recommended)

### Automatic Setup

```bash
# Run the DNS setup script
./scripts/setup-dns.sh

# Choose option 1 (dnsmasq)
```

### Manual Setup

<details>
<summary>macOS Manual Setup</summary>

```bash
# Install dnsmasq
brew install dnsmasq

# Configure dnsmasq for kuberde.local
echo "address=/.kuberde.local/127.0.0.1" >> /usr/local/etc/dnsmasq.conf

# Start dnsmasq
sudo brew services start dnsmasq

# Configure macOS resolver
sudo mkdir -p /etc/resolver
echo "nameserver 127.0.0.1" | sudo tee /etc/resolver/kuberde.local

# Test
ping test.kuberde.local  # Should resolve to 127.0.0.1
```

For **minikube**, replace `127.0.0.1` with your minikube IP:

```bash
MINIKUBE_IP=$(minikube ip)
echo "address=/.kuberde.local/${MINIKUBE_IP}" >> /usr/local/etc/dnsmasq.conf
sudo brew services restart dnsmasq
```

</details>

<details>
<summary>Linux Manual Setup</summary>

```bash
# Install dnsmasq
sudo apt-get install dnsmasq  # Debian/Ubuntu
# OR
sudo yum install dnsmasq      # RHEL/CentOS

# Configure dnsmasq
echo "address=/.kuberde.local/127.0.0.1" | sudo tee /etc/dnsmasq.d/kuberde

# For minikube, use minikube IP instead
# MINIKUBE_IP=$(minikube ip)
# echo "address=/.kuberde.local/${MINIKUBE_IP}" | sudo tee /etc/dnsmasq.d/kuberde

# Restart dnsmasq
sudo systemctl restart dnsmasq
sudo systemctl enable dnsmasq

# Configure NetworkManager (if present)
echo -e "[main]\ndns=dnsmasq" | sudo tee /etc/NetworkManager/conf.d/dnsmasq.conf
sudo systemctl restart NetworkManager

# Test
ping test.kuberde.local  # Should resolve to 127.0.0.1 or minikube IP
```

</details>

## Option 2: CoreDNS in Kubernetes

This deploys CoreDNS inside your Kubernetes cluster and configures it to handle `*.kuberde.local`.

### Automatic Setup

```bash
# Run the DNS setup script
./scripts/setup-dns.sh

# Choose option 4 (CoreDNS in Kubernetes)
```

### Manual Setup

```bash
# Deploy CoreDNS
kubectl apply -f deploy/k8s/dev-dns.yaml

# The service exposes DNS on NodePort 30053
# Configure your system DNS to use it

# For kind (localhost)
# Add to /etc/resolv.conf: nameserver 127.0.0.1

# For minikube
# Add to /etc/resolv.conf: nameserver $(minikube ip)

# Test
dig @localhost -p 30053 test.kuberde.local
```

**Note**: You may need to run `kubectl port-forward` to access the DNS service:

```bash
kubectl port-forward -n kuberde svc/kuberde-dns 53:53 &
```

## Option 3: nip.io (Easiest, No Setup)

nip.io is a public DNS service that automatically resolves IPs embedded in domain names.

### For kind (localhost)

```bash
# Use domain: 127.0.0.1.nip.io
export DOMAIN="127.0.0.1.nip.io"

# Run quick-start with nip.io
USE_NIP_IO=true ./scripts/quick-start-kind.sh
```

### For minikube

```bash
# Get minikube IP
MINIKUBE_IP=$(minikube ip)

# Use domain: <IP>.nip.io (e.g., 192-168-49-2.nip.io)
export DOMAIN="${MINIKUBE_IP//./-}.nip.io"

# Run quick-start with nip.io
USE_NIP_IO=true ./scripts/quick-start-minikube.sh
```

**Advantages**:
- No local DNS setup required
- Works with wildcards: `*.127.0.0.1.nip.io` automatically resolves
- Great for quick testing

**Disadvantages**:
- Requires internet connectivity
- Slower than local DNS
- Not suitable for production

## Option 4: Manual /etc/hosts (Not Recommended)

**Warning**: This does **NOT** support wildcards. You must manually add each agent subdomain.

```bash
# For kind
echo "127.0.0.1 kuberde.local" | sudo tee -a /etc/hosts

# For minikube
MINIKUBE_IP=$(minikube ip)
echo "${MINIKUBE_IP} kuberde.local" | sudo tee -a /etc/hosts

# Add each agent subdomain manually (tedious!)
echo "127.0.0.1 user-alice-dev.kuberde.local" | sudo tee -a /etc/hosts
echo "127.0.0.1 user-bob-jupyter.kuberde.local" | sudo tee -a /etc/hosts
# ... and so on for every agent
```

This method is **not recommended** for development due to the manual work required for each agent.

## Testing DNS Resolution

After setting up DNS, verify it works:

```bash
# Test main domain
ping kuberde.local

# Test wildcard subdomains
ping test.kuberde.local
ping user-alice-dev.kuberde.local
ping anything.kuberde.local

# All should resolve to the correct IP (127.0.0.1 or minikube IP)
```

Use `dig` for more detailed testing:

```bash
dig kuberde.local
dig test.kuberde.local
```

## Troubleshooting

### DNS Not Resolving

**macOS**:
```bash
# Check dnsmasq status
brew services list | grep dnsmasq

# Check resolver
cat /etc/resolver/kuberde.local

# Restart DNS cache
sudo dscacheutil -flushcache
sudo killall -HUP mDNSResponder
```

**Linux**:
```bash
# Check dnsmasq status
sudo systemctl status dnsmasq

# Check DNS resolution
dig @localhost kuberde.local

# Restart dnsmasq
sudo systemctl restart dnsmasq
```

### Minikube IP Changed

If you restart minikube, the IP may change. Update your DNS:

```bash
# Get new IP
NEW_IP=$(minikube ip)

# Update dnsmasq (macOS)
sudo sed -i.bak "s|address=/.kuberde.local/.*|address=/.kuberde.local/${NEW_IP}|" /usr/local/etc/dnsmasq.conf
sudo brew services restart dnsmasq

# Update dnsmasq (Linux)
sudo sed -i.bak "s|address=/.kuberde.local/.*|address=/.kuberde.local/${NEW_IP}|" /etc/dnsmasq.d/kuberde
sudo systemctl restart dnsmasq
```

Or use **nip.io** to avoid this issue entirely.

### Agent Subdomains Not Working

This usually means wildcard DNS is not set up correctly.

**Verify wildcards work**:
```bash
# These should ALL resolve to the same IP
ping kuberde.local
ping test.kuberde.local
ping anything-random.kuberde.local
```

If only `kuberde.local` works, you're likely using `/etc/hosts` which doesn't support wildcards. Switch to dnsmasq or nip.io.

## Best Practices

1. **For local development**: Use **dnsmasq** for full control and performance
2. **For quick testing**: Use **nip.io** for zero-configuration setup
3. **For CI/CD**: Use **nip.io** or CoreDNS depending on your setup
4. **Avoid /etc/hosts**: It doesn't support wildcards and requires manual updates

## Environment Variables

Once DNS is configured, set these environment variables for your deployment:

```bash
# For kind with dnsmasq or /etc/hosts
export DOMAIN="kuberde.local"
export KUBERDE_PUBLIC_URL="http://kuberde.local"
export KUBERDE_AGENT_DOMAIN="kuberde.local"

# For kind with nip.io
export DOMAIN="127.0.0.1.nip.io"
export KUBERDE_PUBLIC_URL="http://127.0.0.1.nip.io"
export KUBERDE_AGENT_DOMAIN="127.0.0.1.nip.io"

# For minikube with dnsmasq or /etc/hosts
MINIKUBE_IP=$(minikube ip)
export DOMAIN="kuberde.local"
export KUBERDE_PUBLIC_URL="http://kuberde.local"
export KUBERDE_AGENT_DOMAIN="kuberde.local"

# For minikube with nip.io
MINIKUBE_IP=$(minikube ip)
export DOMAIN="${MINIKUBE_IP//./-}.nip.io"
export KUBERDE_PUBLIC_URL="http://${DOMAIN}"
export KUBERDE_AGENT_DOMAIN="${DOMAIN}"
```

The quick-start scripts handle these automatically based on your DNS choice.

## See Also

- [KubeRDE Quick Start Guide](../README.md)
- [Architecture Documentation](../CLAUDE.md)
- [Development Setup](./DEVELOPMENT.md)
