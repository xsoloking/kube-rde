# K3D vs K3S: Choosing Your Local Deployment

KubeRDE now supports two lightweight Kubernetes deployment options, both using **nip.io** for zero-configuration DNS with multi-subdomain support.

## Quick Comparison

| Feature | K3D | K3S |
|---------|-----|-----|
| **What is it?** | k3s running in Docker containers | Lightweight k8s installed on your system |
| **Installation** | No sudo required | Requires sudo |
| **Startup Time** | ~30 seconds | ~60 seconds (first install) |
| **Cleanup** | `k3d cluster delete` (instant) | `sudo k3s-uninstall.sh` |
| **Persistence** | Survives system restart | Survives system restart |
| **Multi-cluster** | ✅ Easy (multiple Docker containers) | ❌ One per system |
| **Network Access** | 127.0.0.1 only (via port mapping) | Full network access (host IP) |
| **Resource Usage** | Lower (shared Docker daemon) | Slightly higher (native) |
| **Best For** | Development, testing, CI/CD | Long-term dev, edge, production-like |

## DNS Architecture with nip.io

Both solutions use the same DNS pattern with **nip.io** for automatic wildcard domain resolution:

### K3D (127.0.0.1.nip.io)

```
Main Domain:     127-0-0-1.nip.io           → 127.0.0.1
Keycloak:        sso.127-0-0-1.nip.io       → 127.0.0.1
Agent Example:   user-alice-dev.127-0-0-1.nip.io → 127.0.0.1
```

✅ **Advantages:**
- Works offline (localhost always resolves)
- No network exposure
- Same on all machines

❌ **Limitations:**
- Only accessible from localhost
- Cannot access from other devices

### K3S ([host-ip].nip.io)

```
Main Domain:     192-168-1-100.nip.io           → 192.168.1.100
Keycloak:        sso.192-168-1-100.nip.io       → 192.168.1.100
Agent Example:   user-alice-dev.192-168-1-100.nip.io → 192.168.1.100
```

✅ **Advantages:**
- Accessible from any device on network
- Real-world network testing
- Mobile device testing

❌ **Limitations:**
- Requires internet for nip.io DNS
- IP may change (DHCP)
- Network firewall may block

## When to Use Which?

### Use K3D if you want to:

✅ **Quick Development & Testing**
- Rapid cluster creation/destruction
- Testing different configurations
- CI/CD pipelines
- Learning Kubernetes

✅ **Isolation**
- Run multiple KubeRDE instances
- Avoid sudo requirements
- Keep system clean

✅ **Portability**
- Same setup on any Docker-enabled machine
- Consistent localhost experience

**Example Use Cases:**
```bash
# Run multiple instances for different projects
k3d cluster create kuberde-project-a
k3d cluster create kuberde-project-b

# Quick test, then delete
k3d cluster create test-cluster
# ... test ...
k3d cluster delete test-cluster
```

### Use K3S if you want to:

✅ **Production-like Environment**
- Long-term development setup
- Edge computing scenarios
- IoT device deployment
- Always-on local environment

✅ **Network Access**
- Test from mobile devices
- Multi-device development
- Team demos on local network

✅ **System Integration**
- systemd integration
- Native performance
- Production deployment practice

**Example Use Cases:**
```bash
# Persistent dev environment that stays running
sudo systemctl start k3s

# Access from phone/tablet on same network
# http://192-168-1-100.nip.io

# Edge device deployment (Raspberry Pi, etc.)
```

## Multi-Subdomain Setup

Both solutions implement the **same multi-subdomain architecture**:

### Subdomain Structure

1. **Main Domain** - Web UI and API
   ```
   [host-ip].nip.io
   ```

2. **Keycloak Subdomain** - Authentication
   ```
   sso.[host-ip].nip.io
   ```

3. **Agent Wildcard** - Dynamic agent routing
   ```
   *.[host-ip].nip.io
   ```

### How It Works

```
┌─────────────────────────────────────────┐
│  Browser: user-alice-dev.127-0-0-1.nip.io │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  nip.io DNS Resolution                  │
│  *.127-0-0-1.nip.io → 127.0.0.1        │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  Traefik Ingress (port 80/443)         │
│  - Examines Host header                 │
│  - Routes to appropriate service        │
└──────────────┬──────────────────────────┘
               │
     ┌─────────┴──────────┬──────────────┐
     ▼                    ▼              ▼
┌─────────┐      ┌─────────────┐   ┌──────────┐
│ Main    │      │ Keycloak    │   │ Server   │
│ Domain  │      │ (sso.*)     │   │ (agents) │
└─────────┘      └─────────────┘   └──────────┘
```

### Configuration Examples

**K3D:**
```bash
./scripts/quick-start-k3d.sh

# Automatically configured:
DOMAIN=127-0-0-1.nip.io
KEYCLOAK_DOMAIN=sso.127-0-0-1.nip.io
PUBLIC_URL=http://127-0-0-1.nip.io
```

**K3S:**
```bash
./scripts/quick-start-k3s.sh

# Automatically detects host IP (e.g., 192.168.1.100):
DOMAIN=192-168-1-100.nip.io
KEYCLOAK_DOMAIN=sso.192-168-1-100.nip.io
PUBLIC_URL=http://192-168-1-100.nip.io
```

## Installation Commands

### K3D

```bash
# Install k3d (if not already installed)
# macOS
brew install k3d

# Linux
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

# Run quick start
./scripts/quick-start-k3d.sh

# Access
open http://127-0-0-1.nip.io
```

### K3S

```bash
# Run quick start (installs k3s if needed)
./scripts/quick-start-k3s.sh

# Access (from any device on network)
# Replace IP with your actual host IP
open http://192-168-1-100.nip.io
```

## Management Commands

### K3D

```bash
# List clusters
k3d cluster list

# Stop cluster (keeps data)
k3d cluster stop kuberde

# Start cluster
k3d cluster start kuberde

# Delete cluster (removes everything)
k3d cluster delete kuberde

# Create with custom workers
WORKERS=3 ./scripts/quick-start-k3d.sh
```

### K3S

```bash
# Check status
sudo systemctl status k3s

# Stop service
sudo systemctl stop k3s

# Start service
sudo systemctl start k3s

# Uninstall completely
sudo /usr/local/bin/k3s-uninstall.sh

# Custom version
K3S_VERSION=v1.27.0+k3s1 ./scripts/quick-start-k3s.sh
```

## Troubleshooting

### K3D Issues

**Port already in use:**
```bash
# Check what's using port 80
sudo lsof -i :80

# Or use different ports
k3d cluster create kuberde --port "8080:80@loadbalancer"
# Access: http://127-0-0-1.nip.io:8080
```

**Docker not running:**
```bash
# Start Docker Desktop (macOS/Windows)
# Or Docker daemon (Linux)
sudo systemctl start docker
```

### K3S Issues

**IP address changed:**
```bash
# Get new IP
hostname -I

# Re-run quick-start (it will detect new IP)
./scripts/quick-start-k3s.sh
```

**Permission denied:**
```bash
# k3s requires sudo for install/uninstall
sudo /usr/local/bin/k3s-uninstall.sh
```

### Common Issues (Both)

**nip.io not resolving:**
```bash
# Test internet connection
ping nip.io

# Test specific subdomain
ping 127-0-0-1.nip.io
dig 127-0-0-1.nip.io

# Fallback: Use localhost (no Keycloak subdomain)
# This won't work for multi-subdomain setup
```

**Pods not starting:**
```bash
# Check pod status
kubectl get pods -n kuberde

# View logs
kubectl logs -n kuberde deployment/kuberde-server

# Describe pod for events
kubectl describe pod -n kuberde <pod-name>
```

## Migration Between K3D and K3S

You can easily move your workloads between k3d and k3s:

### Export from K3D

```bash
# Export KubeRDE configuration
kubectl get all -n kuberde -o yaml > kuberde-backup.yaml

# Export data (if using PVCs)
kubectl cp kuberde/<pod-name>:/data ./kuberde-data
```

### Import to K3S

```bash
# Switch to k3s context
kubectl config use-context default

# Apply configuration
kubectl apply -f kuberde-backup.yaml

# Restore data
kubectl cp ./kuberde-data kuberde/<pod-name>:/data
```

## Recommended Workflow

### For Daily Development

```bash
# Use k3d for quick iterations
k3d cluster create kuberde
./scripts/quick-start-k3d.sh

# Develop and test
# ...

# When done for the day
k3d cluster stop kuberde  # Preserves state

# Next day
k3d cluster start kuberde  # Resume where you left off
```

### For Long-term Setup

```bash
# Install k3s once
./scripts/quick-start-k3s.sh

# Always available
# Just use kubectl as needed

# Test from other devices
# Phone: http://192-168-1-100.nip.io
# Tablet: http://192-168-1-100.nip.io
```

## Conclusion

- **K3D**: Best for development, testing, and rapid iteration
- **K3S**: Best for long-term environments and network access
- **Both**: Use nip.io for zero-configuration multi-subdomain DNS

Choose based on your workflow, not technical limitations - both are excellent choices!

## See Also

- [DNS Setup Guide](./DNS-SETUP.md)
- [Quick Start Scripts](../scripts/README.md)
- [Architecture Documentation](../CLAUDE.md)
