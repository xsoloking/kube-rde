# Tutorial 3: SSH Access

**Time:** 10 minutes
**Difficulty:** Beginner
**Prerequisites:** Completed [Tutorial 2: First Workspace](02-first-workspace.md)

## What You'll Learn

- How to install and configure KubeRDE CLI
- How to authenticate from the command line
- How to connect to workspaces via SSH
- How to use SSH keys for authentication
- How to configure SSH config for easy access

## Prerequisites

- KubeRDE running with at least one workspace
- SSH service added to workspace
- Terminal/Command Prompt access
- Internet connection

## Why SSH Access?

SSH access allows you to:
- Use your favorite terminal and shell
- Run command-line tools
- Transfer files with scp/rsync
- Port forward services
- Use VS Code Remote SSH extension
- Automate tasks with scripts

## Steps

### Step 1: Install KubeRDE CLI

The CLI tool handles authentication and SSH connections.

**macOS/Linux:**
```bash
# Download CLI binary
curl -LO https://github.com/xsoloking/kube-rde/releases/latest/download/kuberde-cli-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)

# Make executable
chmod +x kuberde-cli-*

# Move to PATH
sudo mv kuberde-cli-* /usr/local/bin/kuberde-cli

# Verify installation
kuberde-cli version
```

**Windows (PowerShell):**
```powershell
# Download from GitHub releases
Invoke-WebRequest -Uri "https://github.com/xsoloking/kube-rde/releases/latest/download/kuberde-cli-windows-amd64.exe" -OutFile "kuberde-cli.exe"

# Move to PATH (example)
Move-Item kuberde-cli.exe C:\Windows\System32\

# Verify
kuberde-cli version
```

Expected output:
```
kuberde-cli version v1.0.0
```

### Step 2: Configure Server URL

Tell the CLI where your KubeRDE server is:

**For local development:**
```bash
kuberde-cli config set server http://localhost:8080
```

**For kind/minikube:**
```bash
kuberde-cli config set server http://kuberde.local
```

**For production:**
```bash
kuberde-cli config set server https://kuberde.example.com
```

Verify configuration:
```bash
kuberde-cli config show
```

### Step 3: Login

Authenticate with KubeRDE:

```bash
kuberde-cli login
```

**What happens:**
1. Browser opens automatically
2. Redirects to Keycloak login page
3. Enter your credentials (admin/admin or your user)
4. Redirected back with success message
5. Token saved to `~/.frp/token.json`

**Manual login (if browser doesn't open):**
```bash
kuberde-cli login --manual
```

This will display a URL to open manually.

**Verify authentication:**
```bash
kuberde-cli auth status
```

Expected output:
```
✓ Authenticated as: admin
Token expires: 2024-01-05 15:00:00
```

### Step 4: List Your Workspaces

See available workspaces:

```bash
kuberde-cli workspaces list
```

Expected output:
```
NAME                 OWNER    STATUS    SERVICES
my-dev-workspace     admin    Online    ssh-server, jupyter-lab
```

### Step 5: Connect via SSH

Connect to your workspace:

```bash
kuberde-cli connect my-dev-workspace
```

Or using the full agent ID:
```bash
kuberde-cli connect user-admin-my-dev-workspace
```

**First connection:**
```
The authenticity of host 'user-admin-my-dev-workspace' can't be established.
ED25519 key fingerprint is SHA256:...
Are you sure you want to continue connecting (yes/no/[fingerprint])? yes
```

Type `yes` and press Enter.

**You're in!**
```
admin@my-dev-workspace:~$
```

Try some commands:
```bash
# Check Python version
python --version

# List files
ls -la

# Check system info
uname -a

# Exit
exit
```

### Step 6: Setup SSH Keys (Recommended)

For passwordless authentication, add your SSH public key:

**Generate SSH key (if you don't have one):**
```bash
ssh-keygen -t ed25519 -C "your_email@example.com"
```

Press Enter for default location and passphrase.

**Copy your public key:**
```bash
# macOS/Linux
cat ~/.ssh/id_ed25519.pub
```

**Add to KubeRDE:**

1. Go to Web UI → **Profile** → **SSH Keys**
2. Click **"Add SSH Key"**
3. Paste your public key
4. Give it a name: "My Laptop"
5. Click **"Add"**

Or via CLI:
```bash
kuberde-cli ssh-key add ~/.ssh/id_ed25519.pub --name "My Laptop"
```

**Test passwordless connection:**
```bash
ssh user-admin-my-dev-workspace.kuberde.local
```

No password prompt!

### Step 7: Configure SSH Config (Optional but Recommended)

Make connections even easier with SSH config:

**Edit `~/.ssh/config`:**
```bash
nano ~/.ssh/config
```

**Add configuration:**
```
# KubeRDE Workspaces
Host *.kuberde.local
    ProxyCommand kuberde-cli connect %h
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    ServerAliveInterval 30
    ServerAliveCountMax 3

# Specific workspace alias
Host mydev
    HostName user-admin-my-dev-workspace.kuberde.local
    ProxyCommand kuberde-cli connect user-admin-my-dev-workspace
```

**Now connect easily:**
```bash
# Using full name
ssh user-admin-my-dev-workspace.kuberde.local

# Using alias
ssh mydev
```

### Step 8: Advanced SSH Features

**Port Forwarding:**

Forward Jupyter from workspace to local machine:
```bash
ssh -L 8888:localhost:8888 mydev
```

Now access Jupyter at http://localhost:8888

**SOCKS Proxy:**

Create a SOCKS proxy through workspace:
```bash
ssh -D 1080 mydev
```

Configure browser to use proxy at localhost:1080

**File Transfer:**

Upload files:
```bash
scp local-file.txt mydev:~/
```

Download files:
```bash
scp mydev:~/remote-file.txt ./
```

Sync directories:
```bash
rsync -avz ./local-dir/ mydev:~/remote-dir/
```

**Run Remote Commands:**
```bash
# Execute command and exit
ssh mydev "python --version"

# Run script
ssh mydev "bash -s" < local-script.sh
```

## Verification

Verify SSH access is working:

1. ✓ **CLI installed:** `kuberde-cli version` works
2. ✓ **Authenticated:** `kuberde-cli auth status` shows user
3. ✓ **Can connect:** `kuberde-cli connect workspace-name` works
4. ✓ **SSH keys work:** Passwordless connection (if configured)
5. ✓ **SSH config:** Can connect via `ssh mydev`

## Troubleshooting

### CLI Not Found

```bash
# Check PATH
echo $PATH

# Find where CLI is installed
which kuberde-cli

# Add to PATH in ~/.bashrc or ~/.zshrc
export PATH=$PATH:/usr/local/bin
```

### Login Fails

```bash
# Check server URL
kuberde-cli config get server

# Try manual login
kuberde-cli login --manual

# Force re-login
kuberde-cli login --force

# Check Keycloak is accessible
curl http://kuberde.local/auth/realms/kuberde/.well-known/openid-configuration
```

### Connection Refused

**Check workspace is online:**
```bash
kuberde-cli workspaces list
```

Status should be "Online".

**Check SSH service is running:**
- Go to Web UI → Workspace Detail
- Verify SSH service shows "Running"

**Check agent connectivity:**
```bash


kubectl logs -n kuberde -l kuberde.io/workspace=my-dev-workspace
```

### Authentication Failed

**Token expired:**
```bash
# Remove old token
rm ~/.frp/token.json

# Login again
kuberde-cli login
```

**Wrong SSH key:**
```bash
# List your SSH keys
kuberde-cli ssh-key list

# Add correct key
kuberde-cli ssh-key add ~/.ssh/id_ed25519.pub
```

### Host Key Verification Failed

```bash
# Remove old host key
ssh-keygen -R user-admin-my-dev-workspace.kuberde.local

# Or disable checking in SSH config (not recommended for production)
StrictHostKeyChecking no
```

## VS Code Remote SSH Integration

Use VS Code to develop in your workspace:

1. **Install Extension:**
   - Install "Remote - SSH" extension in VS Code

2. **Configure:**
   - Open Command Palette (Cmd/Ctrl+Shift+P)
   - "Remote-SSH: Connect to Host"
   - Select "mydev" or enter full hostname

3. **Start Coding:**
   - VS Code opens connected to workspace
   - Full IDE features available
   - Extensions run remotely

See [Tutorial 17: VS Code Remote Development](17-vscode-remote.md) for detailed setup.

## Next Steps

Now that you can access workspaces via SSH:

1. **[Tutorial 4: Managing Services](04-managing-services.md)** - Add more services
2. **[Tutorial 5: Resource Configuration](05-resource-configuration.md)** - Optimize resources
3. **[Tutorial 17: VS Code Remote](17-vscode-remote.md)** - Develop in VS Code

## Best Practices

### SSH Key Management

- **Use separate keys** for different machines
- **Name keys descriptively** in Web UI
- **Rotate keys** periodically
- **Don't share** private keys

### Connection Efficiency

- **Use SSH config** for quick access
- **Keep connections alive** with ServerAliveInterval
- **Use connection multiplexing**:
  ```
  ControlMaster auto
  ControlPath ~/.ssh/control-%r@%h:%p
  ControlPersist 10m
  ```

### Security

- **Don't disable host key checking** in production
- **Use strong passphrases** for SSH keys
- **Logout when done** to free resources
- **Monitor active sessions** in Web UI

## Additional Resources

- [CLI Configuration Guide](../guides/CLI_CONFIGURATION.md)
- [SSH Configuration Guide](../guides/SSH_USAGE.md)
- [Security Best Practices](../SECURITY.md)
- [Troubleshooting Guide](../guides/TROUBLESHOOTING.md)

## Quick Reference

```bash
# Install CLI
curl -LO https://github.com/xsoloking/kube-rde/releases/latest/download/kuberde-cli-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)

# Configure
kuberde-cli config set server http://kuberde.local

# Login
kuberde-cli login

# List workspaces
kuberde-cli workspaces list

# Connect
kuberde-cli connect my-dev-workspace

# SSH config
Host *.kuberde.local
    ProxyCommand kuberde-cli connect %h

# Connect with alias
ssh mydev

# Port forward
ssh -L 8888:localhost:8888 mydev

# File transfer
scp file.txt mydev:~/
```

## Video Tutorial

Watch the video version:

[![SSH Access Video](../media/videos/thumbnails/tutorial-03-thumbnail.png)](../media/videos/tutorial-03-ssh-access.mp4)

---

**Congratulations!** 🎉 You can now access your workspaces from the command line. Continue to [Tutorial 4](04-managing-services.md) to learn about managing services.
