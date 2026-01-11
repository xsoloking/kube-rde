# KubeRDE Web UI

Modern React-based admin console for managing cloud development environments through Kubernetes.

## Features

- **User Management**: Create and manage users, assign roles, set resource quotas
- **Workspace Management**: Create and manage development workspaces with persistent storage
- **Service Management**: Deploy and manage services (SSH, Coder, Jupyter, File Server)
- **Dashboard**: Real-time resource usage monitoring and metrics visualization
- **Dark Mode**: Modern dark theme UI with Material Design icons

## Architecture

- **Framework**: React 19.2.3 with React Router 7.11.0
- **Styling**: Tailwind CSS (CDN) + Material Symbols Icons
- **Charts**: Recharts for data visualization
- **Build Tool**: Vite 6.2.0
- **Server**: Nginx with reverse proxy to backend API

## Local Development

### Prerequisites

- Node.js 18+
- npm or yarn

### Setup

1. Install dependencies:

   ```bash
   npm install
   ```

2. Start development server:

   ```bash
   npm run dev
   ```

   The app will be available at `http://localhost:3000`

3. Build for production:
   ```bash
   npm run build
   ```
   Output goes to `dist/` directory

## Docker Deployment

### Build Docker Image

```bash
docker build -f Dockerfile.web -t soloking/kuberde-web:latest .
```

### Run Docker Container

```bash
docker run -p 8080:80 \
  -e BACKEND_URL="http://kuberde-server:8080" \
  -e OIDC_AUTHORITY="https://sso.byai.uk/realms/kuberde" \
  -e OIDC_CLIENT_ID="kuberde-cli" \
  soloking/kuberde-web:latest
```

Then access at `http://localhost:8080`

## Kubernetes Deployment

### Prerequisites

- Kubernetes cluster with kuberde namespace
- kuberde-server running on the cluster
- Keycloak for OIDC authentication

### Deploy

```bash
# Create namespace (if not exists)
kubectl create namespace kuberde

# Deploy web UI
kubectl apply -f deploy/k8s/02-web.yaml

# Deploy ingress
kubectl apply -f deploy/k8s/05-ingress.yaml

# Verify deployment
kubectl get pods -n kuberde -l app=kuberde-web
kubectl get svc -n kuberde kuberde-web
```

### Access

The web UI will be available at:

- **Direct**: Through the Service (kubectl port-forward)
- **Ingress**: Through the configured ingress host (e.g., frp.byai.uk)

## Environment Variables

### Development

No special environment variables needed for local development.

### Production/Kubernetes

The following environment variables can be configured:

- **BACKEND_URL** (default: `http://kuberde-server:8080`)
  - Backend API server URL for proxying API requests

- **OIDC_AUTHORITY** (default: `https://sso.byai.uk/realms/kuberde`)
  - Keycloak realm URL for OpenID Connect authentication

- **OIDC_CLIENT_ID** (default: `kuberde-cli`)
  - OAuth 2.0 Client ID for OIDC flow

These are injected into the frontend via `config.js` at runtime.

## Project Structure

```
web/
├── pages/                  # Page components
│   ├── Dashboard.tsx
│   ├── Workspaces.tsx
│   ├── WorkspaceCreate.tsx
│   ├── WorkspaceDetail.tsx
│   ├── ServiceCreate.tsx
│   ├── ServiceDetail.tsx
│   ├── UserManagement.tsx
│   └── UserEdit.tsx
├── components/             # Reusable UI components
│   ├── Header.tsx         # Top navigation
│   └── Sidebar.tsx        # Left sidebar
├── App.tsx                # Main app with routing
├── types.ts               # TypeScript interfaces
├── constants.ts           # Mock data (demo purposes)
├── index.tsx              # React entry point
├── index.html             # HTML template
├── index.css              # Global styles
├── vite.config.ts         # Vite configuration
└── nginx.conf             # Nginx template for production
```

## Routing

The app uses HashRouter with the following routes:

- `/` - Dashboard with resource overview
- `/workspaces` - List all workspaces
- `/workspaces/create` - Create new workspace
- `/workspaces/:id` - Workspace details and service management
- `/workspaces/:workspaceId/services/create` - Create service in workspace
- `/services/:id` - Service details and management
- `/users` - User management (admin only)
- `/users/:id` - Edit user details

## UI Design

### Color Scheme

- Primary: `#1a79ff` (Technology Blue)
- Background: `#0f1723` (Dark)
- Surface: `#1e2128` (Surface Dark)
- Text Secondary: `#8da8ce`

### Typography

- Display: Space Grotesk
- Body: Noto Sans

### Status Indicators

- Running: Green
- Stopped: Gray
- Starting/Restarting: Blue (animated)
- Error: Red

## Current Status

This is the initial release of the web UI with:

- ✅ Full UI implementation with mock data
- ✅ All routing and navigation working
- ✅ Docker containerization
- ✅ Kubernetes manifest
- ⏳ Backend API integration (planned)
- ⏳ Real authentication (planned)
- ⏳ Real-time metrics (planned)

## Future Work

1. **Backend Integration**
   - Create API service layer
   - Implement actual API calls
   - Add error handling

2. **Authentication**
   - Implement OIDC login flow
   - Token management
   - Authorization checks

3. **Real-time Features**
   - WebSocket for logs and metrics
   - Real-time status updates
   - Event notifications

4. **Performance Optimization**
   - Code splitting for large bundles
   - Lazy loading of routes
   - Image optimization

## Build & Deployment Commands

```bash
# Development
npm install
npm run dev

# Production Build
npm run build

# Docker
docker build -f Dockerfile.web -t soloking/kuberde-web:latest .
docker push soloking/kuberde-web:latest

# Kubernetes Deploy
kubectl apply -f deploy/k8s/02-web.yaml
kubectl rollout status deployment/kuberde-web -n kuberde
```

## Troubleshooting

### Build fails with "vite: command not found"

Run `npm install` first to install dependencies.

### Docker build fails with "nginx.conf not found"

Ensure you're in the repository root directory when running the docker build command.

### Container not accessible after deployment

Check logs: `kubectl logs -n kuberde -l app=kuberde-web`

Check service: `kubectl get svc -n kuberde kuberde-web`

Check pod: `kubectl get pods -n kuberde -l app=kuberde-web`

## License

Part of the KubeRDE project.
