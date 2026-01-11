# Tutorial 2: Your First Workspace

**Time:** 10 minutes
**Difficulty:** Beginner
**Prerequisites:** Completed [Tutorial 1: Quick Start](01-quick-start.md)

## What You'll Learn

- What is a workspace in KubeRDE
- How to create a workspace
- How to add services (SSH, Jupyter, VS Code)
- How to access your workspace
- How to delete a workspace

## Prerequisites

- KubeRDE running locally (from Tutorial 1)
- Logged in to Web UI
- Internet connection (for pulling Docker images)

## Understanding Workspaces

A **workspace** in KubeRDE is your personal development environment that includes:
- Dedicated Kubernetes pod
- Persistent storage
- Multiple services (SSH, Jupyter, VS Code, File Browser)
- Isolated network
- Resource limits (CPU, memory)

Think of it as your own server that you can access remotely, with all your tools pre-installed.

## Steps

### Step 1: Navigate to Workspaces

From the dashboard, click **"Workspaces"** in the left sidebar, or click **"Create Workspace"** button.

![Workspace List](../media/screenshots/workspace-list.png)
*Workspace listing page*

### Step 2: Create Workspace

Click **"Create Workspace"** button.

Fill in the workspace details:

**Basic Information:**
- **Name:** `my-dev-workspace` (lowercase, hyphens only)
- **Description:** `My first development environment`

**Template:**
- Select **"Default"** template
  - Includes Python, Node.js, Go
  - SSH server pre-configured
  - 10GB storage by default

**Resources:**
- **Storage:** `10Gi` (keep default)
- **CPU:** `1000m` (1 CPU core)
- **Memory:** `2Gi` (2GB RAM)

![Create Workspace Form](../media/screenshots/workspace-create.png)
*Workspace creation form*

Click **"Create"** button.

### Step 3: Wait for Provisioning

The system will:
1. Create Kubernetes resources
2. Pull Docker images (first time: ~2-3 minutes)
3. Start the agent pod
4. Mount persistent volume

**Status indicators:**
- 🟡 **Pending**: Creating resources
- 🟢 **Online**: Ready to use
- 🔴 **Error**: Something went wrong

![Workspace Provisioning](../media/screenshots/workspace-status-pending.png)
*Workspace being provisioned*

**Check status:**
- Refresh the page or wait for automatic update
- Status should change from "Pending" to "Online" in 1-3 minutes

### Step 4: Access Workspace Details

Once status is **Online**, click on the workspace name to view details.

You'll see:
- **Agent Status**: Online/Offline
- **Resource Usage**: CPU, memory, storage
- **Services**: Currently running services
- **Actions**: Add service, SSH, delete

![Workspace Detail](../media/screenshots/workspace-detail.png)
*Workspace detail view*

### Step 5: Add SSH Service

Click **"Add Service"** button.

**Service Configuration:**
- **Service Type:** `SSH`
- **Name:** `ssh-server`
- **Internal Port:** `22` (default SSH port)

**Advanced Settings** (keep defaults):
- **Resource Limits:** 100m CPU, 128Mi memory
- **Auto-start:** Enabled

Click **"Create Service"**.

The SSH service will start in a few seconds.

### Step 6: Add Jupyter Service

Add another service for Jupyter notebooks:

Click **"Add Service"** → **Create Service**

**Service Configuration:**
- **Service Type:** `Jupyter`
- **Name:** `jupyter-lab`
- **Internal Port:** `8888`

Click **"Create Service"**.

### Step 7: Access Jupyter

Once Jupyter service is **Running**:

1. Click on **"jupyter-lab"** service in the service list
2. Click **"Open in Browser"** button
3. A new tab opens with Jupyter Lab

![Jupyter in Browser](../media/screenshots/jupyter-notebook.png)
*Jupyter Lab running in workspace*

**Default token:** Check service logs or use default: `kuberde`

### Step 8: Test Your Workspace

In Jupyter Lab:

1. Create a new notebook
2. Run a simple Python command:
   ```python
   import sys
   print(f"Python {sys.version}")
   print("Hello from KubeRDE!")
   ```
3. Execute the cell (Shift+Enter)

You should see the output confirming Python is working.

### Step 9: Add File Browser (Optional)

For easy file management:

**Add Service:**
- **Type:** `File Browser`
- **Name:** `file-manager`
- **Port:** `8080`

Access via browser to upload/download files.

## Verification

Verify your workspace is working:

1. ✓ **Workspace status:** Online
2. ✓ **SSH service:** Running
3. ✓ **Jupyter service:** Running and accessible
4. ✓ **Can execute code:** Python cell ran successfully

## Workspace Management

### View Resource Usage

In the workspace detail page:
- **CPU Usage:** Real-time CPU consumption
- **Memory Usage:** Current memory usage
- **Storage:** Disk usage

### Stop/Start Workspace

**Stop workspace:**
```
Workspace Detail → Actions → Stop
```
- Preserves data
- Stops all services
- Releases compute resources

**Start workspace:**
```
Workspace Detail → Actions → Start
```
- Restarts all services
- Mounts existing storage

### Delete Workspace

**⚠️ Warning:** This is permanent!

To delete a workspace:

1. Go to **Workspace Detail**
2. Click **"Delete"** button
3. Confirm deletion
4. All data will be lost (unless backed up)

**Backup before deleting:**
- Download important files via File Browser
- Or connect via SSH and copy data

## Troubleshooting

### Workspace Stuck in "Pending"

```bash
# Check Kubernetes pods
kubectl get pods -n kuberde

# Check operator logs
kubectl logs -n kuberde -l app.kubernetes.io/name=kuberde-operator -f

# Check agent logs
kubectl logs -n kuberde -l kuberde.io/workspace=my-dev-workspace
```

Common causes:
- Image pull timeout (slow internet)
- Insufficient cluster resources
- PVC provisioning issues

### Can't Access Services

**Check service status:**
1. Go to Workspace Detail
2. Verify service shows "Running"
3. Click on service for more details

**Check browser console:**
- Press F12
- Look for network errors
- Check for CORS issues

**Verify authentication:**
- You must be logged in
- Token must be valid
- Try logging out and back in

### Jupyter Token Issues

**Find Jupyter token:**
```bash
# Using CLI
kuberde-cli exec my-dev-workspace jupyter-lab -- \
  jupyter lab list

# Or check service logs in Web UI
```

**Reset Jupyter token:**
```bash
kuberde-cli exec my-dev-workspace jupyter-lab -- \
  jupyter lab password
```

### Storage Full

**Check storage usage:**
```
Workspace Detail → Storage Usage
```

**Clean up space:**
```bash
# Connect via SSH
kuberde-cli connect my-dev-workspace

# Check disk usage
df -h

# Find large files
du -sh * | sort -h

# Clean up
rm -rf unnecessary-files/
```

## Next Steps

Now that you have a working workspace:

1. **[Tutorial 3: SSH Access](03-ssh-access.md)** - Connect via SSH from your terminal
2. **[Tutorial 4: Managing Services](04-managing-services.md)** - Add VS Code, customize services
3. **[Tutorial 5: Resource Configuration](05-resource-configuration.md)** - Adjust CPU, memory, GPU

## Best Practices

### Naming Conventions

Use descriptive workspace names:
- ✅ Good: `ml-training`, `web-dev`, `data-analysis`
- ❌ Bad: `workspace1`, `test`, `temp`

### Resource Sizing

Start small, scale up if needed:
- **Light work:** 500m CPU, 1Gi RAM
- **Development:** 1000m CPU, 2Gi RAM
- **Data science:** 2000m CPU, 4Gi RAM, GPU optional
- **ML training:** 4000m+ CPU, 8Gi+ RAM, GPU recommended

### Data Backup

Regularly backup important data:
- Use File Browser to download
- Connect via SSH and rsync
- Commit code to Git repositories

### Workspace Lifecycle

- **Development:** Keep workspace running during active work
- **Idle:** Stop workspace to save resources
- **Completed:** Delete workspace and create new for next project

## Additional Resources

- [Workspace Management Guide](../guides/WORKSPACE_MANAGEMENT.md)
- [Service Types Reference](../reference/SERVICE_TYPES.md)
- [Resource Limits](../reference/RESOURCE_LIMITS.md)
- [Storage Management](../guides/STORAGE_MANAGEMENT.md)

## Video Tutorial

Watch the video version:

[![First Workspace Video](../media/videos/thumbnails/tutorial-02-thumbnail.png)](../media/videos/tutorial-02-workspace-management.mp4)

---

**Congratulations!** 🎉 You've created your first workspace and added services. Continue to [Tutorial 3](03-ssh-access.md) to learn how to connect via SSH from your terminal.
