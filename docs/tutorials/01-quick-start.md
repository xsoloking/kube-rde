# Tutorial 1: Quick Start Guide

**Time:** 5 minutes
**Difficulty:** Beginner
**Prerequisites:** Docker installed

## What You'll Learn

- How to install KubeRDE locally
- How to access the Web UI
- How to log in for the first time
- Where to go next

## Prerequisites

- Docker installed
- 4GB RAM available
- 10GB disk space
- Internet connection

**Verify Docker:**
```bash
docker --version
```

## Steps

### Step 1: Get KubeRDE

Clone the repository:

```bash
git clone https://github.com/xsoloking/kube-rde.git
cd kube-rde
```

### Step 2: Start KubeRDE

Using our one-command installer:

```bash
# For local development with kind
./scripts/quick-start-kind.sh
```

Or with minikube:

```bash
# For local development with minikube
./scripts/quick-start-minikube.sh
```


**What this does:**
- Starts PostgreSQL database
- Starts Keycloak authentication server
- Starts KubeRDE server
- Starts Web UI
- Configures local domain (for kind/minikube)

**Wait for services to start** (approximately 2 minutes):

```bash
# Check status
make dev-ps

# View logs
make dev-logs
```

### Step 3: Access Web UI

Open your browser and navigate to:

**Docker Compose:**
```
http://localhost:5173
```

**kind/minikube:**
```
http://kuberde.local
```

You should see the KubeRDE login page.

![Login Page](../media/screenshots/login-page.png)
*KubeRDE login page*

### Step 4: First Login

Use the default credentials:

```
Username: admin
Password: admin
```

**⚠️ Important:** Change this password immediately after first login!

After logging in, you'll be redirected to the dashboard.

![Dashboard](../media/screenshots/dashboard-overview.png)
*Main dashboard view*

### Step 5: Explore the Dashboard

The dashboard shows:
- **Workspaces**: Your development environments
- **Services**: Running services (SSH, Jupyter, etc.)
- **Resources**: CPU, memory, and storage usage
- **Quick Actions**: Create workspace, manage users

## Verification

Verify your installation is working:

1. **Web UI accessible:** ✓ You can see the dashboard
2. **Authentication working:** ✓ You logged in successfully
3. **No errors:** ✓ Check browser console (F12)

**Check backend status:**

```bash
# Docker Compose
curl http://localhost:8080/health

# kind/minikube
curl http://kuberde.local/health
```

Expected response:
```json
{"status":"healthy"}
```

## Troubleshooting

### Port Already in Use

```bash
# Check what's using port 5173 (Web UI)
lsof -i :5173

# Or port 8080 (Server)
lsof -i :8080

# Stop conflicting services or use different ports
```

### Services Won't Start

```bash
# Check Docker is running
docker ps

# Restart services
make dev-down
make dev-up

# Check logs for errors
make dev-logs
```

### Can't Access Web UI

**For Docker Compose:**
- Verify URL: http://localhost:5173
- Check firewall settings
- Try incognito mode

**For kind/minikube:**
- Verify `/etc/hosts` entry:
  ```bash
  cat /etc/hosts | grep kuberde
  ```
- Should see: `127.0.0.1 kuberde.local` (kind) or `<minikube-ip> kuberde.local` (minikube)

### Login Fails

```bash

```

## Next Steps

Now that KubeRDE is running, you're ready to:

1. **[Create Your First Workspace](02-first-workspace.md)** - Set up a development environment
2. **[Configure SSH Access](03-ssh-access.md)** - Connect via SSH
3. **Change admin password** - Go to Keycloak admin console

### Change Admin Password

1. Go to Keycloak admin console:
   - Docker Compose: http://localhost:8081
   - kind/minikube: http://kuberde.local/auth/admin

2. Login with `admin` / `admin`

3. Navigate to:
   - **Users** → **admin** → **Credentials**

4. Click **Reset Password**

5. Enter new password (strong password required)

6. Save

## What's Next?

Continue learning with:
- [Tutorial 2: Your First Workspace](02-first-workspace.md)
- [Tutorial 3: SSH Access](03-ssh-access.md)
- [Tutorial 4: Managing Services](04-managing-services.md)

## Additional Resources

- [Installation Guide](../guides/INSTALLATION.md) - Detailed installation docs
- [Local Development](../guides/LOCAL_DEVELOPMENT.md) - Development setup
- [Architecture Overview](../ARCHITECTURE.md) - How KubeRDE works
- [FAQ](../FAQ.md) - Common questions

## Quick Reference Commands

```bash
# Start KubeRDE
make dev-up

# Stop KubeRDE
make dev-down

# View logs
make dev-logs

# Check status
make dev-ps

# Reset everything
make dev-reset

# Access services
# Web UI:     http://localhost:5173
# Server:     http://localhost:8080
# Keycloak:   http://localhost:8081
# PostgreSQL: localhost:5432
```

## Video Tutorial

Watch the video version of this tutorial:

[![Quick Start Video](../media/videos/thumbnails/tutorial-01-thumbnail.png)](../media/videos/tutorial-01-quick-start.mp4)

---

**Congratulations!** 🎉 You've successfully installed KubeRDE and logged in. Continue to [Tutorial 2](02-first-workspace.md) to create your first workspace.
