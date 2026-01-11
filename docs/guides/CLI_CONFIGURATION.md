# CLI Configuration and Update Guide

Complete guide for configuring and updating the KubeRDE CLI.

## Table of Contents

- [Installation](#installation)
- [Initial Configuration](#initial-configuration)
- [Authentication](#authentication)
- [Configuration Management](#configuration-management)
- [Updating the CLI](#updating-the-cli)
- [Troubleshooting](#troubleshooting)
- [Advanced Usage](#advanced-usage)

## Installation

### macOS

```bash
# Using Homebrew (recommended)
brew install kuberde-cli

# Or download binary
curl -LO https://github.com/xsoloking/kube-rde/releases/latest/download/kuberde-cli-darwin-amd64
chmod +x kuberde-cli-darwin-amd64
sudo mv kuberde-cli-darwin-amd64 /usr/local/bin/kuberde-cli
```

### Linux

```bash
# Download and install
curl -LO https://github.com/xsoloking/kube-rde/releases/latest/download/kuberde-cli-linux-amd64
chmod +x kuberde-cli-linux-amd64
sudo mv kuberde-cli-linux-amd64 /usr/local/bin/kuberde-cli

# Verify installation
kuberde-cli version
```

### Windows

```powershell
# Download from GitHub releases
# https://github.com/xsoloking/kube-rde/releases

# Or using Scoop
scoop bucket add kuberde https://github.com/xsoloking/scoop-kuberde
scoop install kuberde-cli
```

### From Source

```bash
# Clone repository
git clone https://github.com/xsoloking/kube-rde.git
cd kube-rde

# Build CLI
make build-cli

# Install
sudo cp kuberde-cli /usr/local/bin/
```

## Initial Configuration

### Quick Setup

```bash
# Configure server URL
kuberde-cli config set server https://kuberde.example.com

# Login and save credentials
kuberde-cli login
```

### Manual Configuration

The CLI stores configuration in `~/.frp/config.json`:

```json
{
  "server": "https://kuberde.example.com",
  "token": "<your-jwt-token>",
  "lastUpdated": "2025-01-05T10:00:00Z"
}
```

**Create manually:**
```bash
mkdir -p ~/.frp
cat > ~/.frp/config.json << EOF
{
  "server": "https://kuberde.example.com"
}
EOF
```

## Authentication

### Login Process

The CLI uses OIDC authentication flow:

```bash
# Start login
kuberde-cli login

# This will:
# 1. Open your browser
# 2. Redirect to Keycloak login page
# 3. After successful login, redirect back to CLI
# 4. Save token to ~/.frp/token.json
```

**What happens:**
1. CLI starts a local HTTP server on a random port
2. Opens browser to: `https://kuberde.example.com/auth/realms/kuberde/protocol/openid-connect/auth`
3. You login with username/password
4. Keycloak redirects to `http://localhost:<port>/callback`
5. CLI receives token and saves it

### Token Storage

Tokens are stored in `~/.frp/token.json`:

```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "expires_at": "2025-01-05T11:00:00Z"
}
```

**Token lifecycle:**
- Access tokens expire after 15 minutes
- Refresh tokens expire after 30 days
- CLI automatically refreshes tokens when needed

### Re-authentication

```bash
# Force re-login
kuberde-cli login --force

# Login with specific user
kuberde-cli login --username alice

# Check current authentication
kuberde-cli auth status
```

## Configuration Management

### View Configuration

```bash
# Show current config
kuberde-cli config show

# Get specific value
kuberde-cli config get server
```

### Update Configuration

```bash
# Set server URL
kuberde-cli config set server https://new-domain.com

# Set timeout
kuberde-cli config set timeout 30s

# Set default workspace
kuberde-cli config set workspace my-workspace
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `server` | KubeRDE server URL | None (required) |
| `timeout` | Connection timeout | 30s |
| `insecure` | Skip TLS verification | false |
| `workspace` | Default workspace | None |
| `agent` | Default agent | None |

### Multiple Profiles

Use different profiles for different environments:

```bash
# Create profile
kuberde-cli config create-profile production \
  --server https://kuberde.company.com

kuberde-cli config create-profile staging \
  --server https://staging.company.com

# Switch profiles
kuberde-cli config use-profile production

# List profiles
kuberde-cli config list-profiles

# Show active profile
kuberde-cli config current-profile
```

**Profile storage:**
```
~/.frp/
├── config.json           # Default profile
├── token.json            # Default profile token
├── profiles/
│   ├── production.json   # Production config
│   ├── production-token.json
│   ├── staging.json
│   └── staging-token.json
```

## Updating the CLI

### Check for Updates

```bash
# Check current version
kuberde-cli version

# Check for updates
kuberde-cli update check

# Sample output:
# Current version: v1.0.0
# Latest version:  v1.2.0
# Update available! Run 'kuberde-cli update' to upgrade.
```

### Update to Latest Version

```bash
# Update to latest
kuberde-cli update

# Update to specific version
kuberde-cli update --version v1.2.0

# Update from specific source
kuberde-cli update --source https://github.com/xsoloking/kube-rde/releases
```

### Manual Update

```bash
# Download latest release
VERSION=v1.2.0
OS=darwin  # or linux, windows
ARCH=amd64 # or arm64

curl -LO https://github.com/xsoloking/kube-rde/releases/download/${VERSION}/kuberde-cli-${OS}-${ARCH}

# Replace existing binary
chmod +x kuberde-cli-${OS}-${ARCH}
sudo mv kuberde-cli-${OS}-${ARCH} /usr/local/bin/kuberde-cli

# Verify
kuberde-cli version
```

### Homebrew Update (macOS)

```bash
# Update Homebrew formulae
brew update

# Upgrade kuberde-cli
brew upgrade kuberde-cli
```

### Version Compatibility

| CLI Version | Server Version | Compatible |
|-------------|----------------|------------|
| v1.0.x      | v1.0.x         | ✅ Yes     |
| v1.0.x      | v1.1.x         | ✅ Yes     |
| v1.0.x      | v2.0.x         | ❌ No      |
| v1.1.x      | v1.0.x         | ⚠️ Partial |

**Check compatibility:**
```bash
kuberde-cli version --check-server
```

## Troubleshooting

### Authentication Issues

**Problem: Login fails**

```bash
# Check server connectivity
curl -I https://kuberde.example.com

# Verify Keycloak is accessible
curl https://kuberde.example.com/auth/realms/kuberde/.well-known/openid-configuration

# Check for proxy/firewall
# Ensure ports 80/443 are accessible
```

**Problem: Token expired**

```bash
# Clear token and re-login
rm ~/.frp/token.json
kuberde-cli login
```

**Problem: Browser doesn't open**

```bash
# Use manual mode
kuberde-cli login --manual

# This will display a URL to open manually
```

### Connection Issues

**Problem: Connection timeout**

```bash
# Increase timeout
kuberde-cli config set timeout 60s

# Test connectivity
kuberde-cli ping
```

**Problem: SSL certificate error**

```bash
# For development/testing only - skip TLS verification
kuberde-cli config set insecure true

# For production - fix the certificate issue
# Add CA certificate to system trust store
```

### Configuration Issues

**Problem: Config file corrupt**

```bash
# Backup current config
cp ~/.frp/config.json ~/.frp/config.json.bak

# Reset to defaults
kuberde-cli config reset

# Reconfigure
kuberde-cli config set server https://kuberde.example.com
kuberde-cli login
```

**Problem: Permission denied**

```bash
# Fix permissions
chmod 600 ~/.frp/config.json
chmod 600 ~/.frp/token.json
chmod 700 ~/.frp
```

### Update Issues

**Problem: Update fails**

```bash
# Check permissions
ls -la $(which kuberde-cli)

# If installed to /usr/local/bin, need sudo
sudo kuberde-cli update

# Or update manually (see above)
```

**Problem: Version mismatch**

```bash
# Check server version
kuberde-cli api-version

# Check CLI version
kuberde-cli version

# Update to compatible version
kuberde-cli update --version v1.0.5
```

## Advanced Usage

### SSH Configuration

Use CLI as SSH ProxyCommand:

**~/.ssh/config:**
```
Host *.kuberde.example.com
    ProxyCommand kuberde-cli connect %h
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    ServerAliveInterval 30
    ServerAliveCountMax 3
```

**Usage:**
```bash
# SSH to workspace
ssh user-alice-dev.kuberde.example.com

# With specific user
ssh myuser@user-alice-dev.kuberde.example.com

# Port forwarding
ssh -L 8080:localhost:8080 user-alice-dev.kuberde.example.com

# SOCKS proxy
ssh -D 1080 user-alice-dev.kuberde.example.com
```

### API Access

Use CLI for API calls:

```bash
# Make API request
kuberde-cli api GET /api/workspaces

# With JSON payload
kuberde-cli api POST /api/workspaces -d '{
  "name": "my-workspace",
  "storageSize": "10Gi"
}'

# Save response
kuberde-cli api GET /api/workspaces > workspaces.json
```

### Scripting

Use CLI in scripts:

```bash
#!/bin/bash

# Check if logged in
if ! kuberde-cli auth status &>/dev/null; then
    echo "Please login first"
    kuberde-cli login
fi

# Get workspace list
WORKSPACES=$(kuberde-cli api GET /api/workspaces)

# Parse with jq
echo "$WORKSPACES" | jq -r '.[].name'

# Create workspace programmatically
kuberde-cli api POST /api/workspaces -d '{
  "name": "auto-workspace-'$(date +%s)'",
  "storageSize": "5Gi"
}'
```

### Environment Variables

Override configuration with env vars:

```bash
# Set server URL
export KUBERDE_SERVER=https://kuberde.example.com

# Set token manually
export KUBERDE_TOKEN=eyJhbGc...

# Skip TLS verification
export KUBERDE_INSECURE=true

# Use in commands
kuberde-cli workspaces list
```

### Debugging

Enable debug output:

```bash
# Verbose output
kuberde-cli -v workspaces list

# Debug mode
kuberde-cli --debug login

# Trace HTTP requests
kuberde-cli --trace api GET /api/workspaces
```

## Migration Scenarios

### Migrating to New Server

```bash
# Export current configuration
kuberde-cli config export > old-config.json

# Configure new server
kuberde-cli config set server https://new-server.com

# Login to new server
kuberde-cli login

# Import old workspace settings (if needed)
# This is application-specific
```

### Multiple Servers

```bash
# Production
kuberde-cli config create-profile prod \
  --server https://kuberde.company.com

# Staging
kuberde-cli config create-profile staging \
  --server https://staging.company.com

# Development
kuberde-cli config create-profile dev \
  --server http://kuberde.local

# Switch between them
kuberde-cli config use-profile prod
kuberde-cli workspaces list

kuberde-cli config use-profile dev
kuberde-cli workspaces list
```

## Best Practices

1. **Keep CLI Updated**
   ```bash
   # Check for updates weekly
   kuberde-cli update check
   ```

2. **Use Profiles for Multiple Environments**
   ```bash
   # Don't: constantly changing config
   # Do: use profiles
   kuberde-cli config create-profile <env>
   ```

3. **Secure Token Storage**
   ```bash
   # Ensure proper permissions
   chmod 600 ~/.frp/token.json
   ```

4. **Use SSH Config**
   ```bash
   # Better than remembering long commands
   # Configure once in ~/.ssh/config
   ```

5. **Script Common Tasks**
   ```bash
   # Create aliases or scripts for frequent operations
   alias kw='kuberde-cli workspaces'
   alias kl='kuberde-cli login'
   ```

## Next Steps

- [Using SSH with KubeRDE](SSH_USAGE.md)
- [Workspace Management](WORKSPACE_MANAGEMENT.md)
- [API Reference](../API.md)

## Support

- **Issues**: [GitHub Issues](https://github.com/xsoloking/kube-rde/issues)
- **Discussions**: [GitHub Discussions](https://github.com/xsoloking/kube-rde/discussions)
- **Documentation**: [Full Documentation](../INDEX.md)
