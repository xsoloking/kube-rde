# Screenshots

This directory contains product screenshots for documentation.

## Required Screenshots

Place the following screenshots in this directory:

### Main UI
- [ ] `dashboard-overview.png` - Main dashboard with workspace overview
- [ ] `workspace-list.png` - Workspace listing page
- [ ] `workspace-create.png` - Workspace creation form
- [ ] `workspace-detail.png` - Workspace detail view with services
- [ ] `service-create.png` - Service creation form
- [ ] `service-detail.png` - Service detail and configuration
- [ ] `user-management.png` - User management interface
- [ ] `audit-logs.png` - Audit logs view

### Features
- [ ] `ssh-connection.png` - Terminal showing SSH connection
- [ ] `jupyter-notebook.png` - Jupyter notebook running in workspace
- [ ] `vscode-browser.png` - VS Code in browser
- [ ] `file-browser.png` - File browser interface
- [ ] `resource-monitoring.png` - Resource usage graphs
- [ ] `template-management.png` - Agent template management
- [ ] `quota-management.png` - User quota configuration

### Keycloak/Auth
- [ ] `login-page.png` - Login screen
- [ ] `keycloak-admin.png` - Keycloak admin console
- [ ] `user-profile.png` - User profile settings

### Kubernetes/Operator
- [ ] `kubernetes-dashboard.png` - K8s dashboard showing KubeRDE resources
- [ ] `rdeagent-crd.png` - RDEAgent custom resource example
- [ ] `operator-logs.png` - Operator logs and reconciliation

### Mobile/Responsive
- [ ] `mobile-dashboard.png` - Mobile view
- [ ] `tablet-workspace.png` - Tablet view

## File Naming

Use lowercase with hyphens, descriptive names:
```
feature-name-context.png
```

Examples:
- `dashboard-overview.png`
- `workspace-create-form.png`
- `ssh-connection-terminal.png`

## Specifications

- **Format:** PNG (preferred) or JPG
- **Resolution:** Minimum 1920x1080 for desktop
- **File size:** < 500KB (use optimization tools)
- **Include UI chrome:** Yes (show browser/application context)

## Optimization

Before committing, optimize images:

```bash
# Using pngquant
pngquant --quality=65-80 *.png

# Using ImageOptim (macOS)
imageoptim *.png

# Using optipng
optipng -o7 *.png
```

## Usage in Documentation

Reference screenshots in markdown:

```markdown
![Dashboard Overview](./screenshots/dashboard-overview.png)
*The main dashboard showing active workspaces and resource usage*
```

## Annotations

Use arrows, boxes, or highlights to draw attention to specific features:
- Red for important actions
- Blue for information
- Green for success states
- Yellow for warnings

Tools: Skitch, Annotate, macOS Markup, GIMP
