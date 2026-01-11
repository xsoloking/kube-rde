# Development Guide

Complete guide for developing KubeRDE locally.

## Table of Contents

- [Development Environment Setup](#development-environment-setup)
- [Local Development Workflows](#local-development-workflows)
- [Component Development](#component-development)
- [Debugging](#debugging)
- [Testing](#testing)
- [Code Quality](#code-quality)
- [Contributing](#contributing)

## Development Environment Setup

### Prerequisites

**Required:**
- Go 1.24 or later
- Node.js 18+ and npm
- Docker
- kubectl
- Local Kubernetes (kind, minikube, or Docker Desktop)

**Recommended:**
- VS Code with Go and TypeScript extensions
- Postman or curl for API testing
- PostgreSQL client (psql)

### Clone Repository

```bash
git clone https://github.com/xsoloking/kube-rde.git
cd kube-rde
```

### Install Dependencies

**Go dependencies:**
```bash
go mod download
go mod verify
```

**Web UI dependencies:**
```bash
cd web
npm install
cd ..
```

### IDE Setup

#### VS Code

Recommended extensions:
- Go (golang.go)
- TypeScript Vue Plugin (Vue.volar)
- Tailwind CSS IntelliSense
- ESLint
- Prettier

**settings.json:**
```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "workspace",
  "editor.formatOnSave": true,
  "[go]": {
    "editor.defaultFormatter": "golang.go"
  },
  "[typescript]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode"
  },
  "[typescriptreact]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode"
  }
}
```

#### GoLand / IntelliJ IDEA

1. Open project
2. Enable Go Modules support
3. Configure code style (gofmt)
4. Enable golangci-lint

## Local Development Workflows

### Option 1: Kubernetes Development

Deploy to local Kubernetes cluster:

```bash
# Create kind cluster
kind create cluster --name kuberde-dev

# Deploy using Helm with dev values
helm upgrade --install kuberde ./charts/kuberde \
  --namespace kuberde-dev \
  --create-namespace \
  -f ./charts/kuberde/values-dev.yaml

# Port-forward for local access
kubectl port-forward -n kuberde-dev svc/kuberde-server 8080:8080
kubectl port-forward -n kuberde-dev svc/kuberde-web 5173:80
```

### Auto-reload Setup

Install air for Go hot reload:

```bash
go install github.com/cosmtrek/air@latest
```

**.air.toml:**
```toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/server"
  cmd = "go build -o ./tmp/server ./cmd/server"
  delay = 1000
  exclude_dir = ["web", "assets", "tmp", "vendor", "testdata"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  poll = false
  poll_interval = 0
  rerun = false
  rerun_delay = 500
  send_interrupt = false
  stop_on_error = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  main_only = false
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
  keep_scroll = true
```

## Component Development

### Server Development

**File structure:**
```
cmd/server/
  main.go           # Main server entry point (~6500 lines)
pkg/
  models/           # Database models
  db/               # Database operations
  repositories/     # Data access layer
```

**Adding a new REST endpoint:**

1. Define handler function:
```go
func handleGetWorkspaces(w http.ResponseWriter, r *http.Request) {
    // Extract user from context
    claims := r.Context().Value("claims").(*Claims)
    username := claims.PreferredUsername

    // Get workspaces from database
    workspaces, err := workspaceRepo.FindByOwner(username)
    if err != nil {
        http.Error(w, "Failed to fetch workspaces", http.StatusInternalServerError)
        return
    }

    // Return JSON
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(workspaces)
}
```

2. Register route with authentication middleware:
```go
http.HandleFunc("/api/workspaces", authMiddleware(handleGetWorkspaces))
```

3. Test endpoint:
```bash
# Get token first
TOKEN=$(kuberde-cli auth token)

# Test endpoint
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/workspaces
```

**Adding database model:**

1. Define struct in `pkg/models/models.go`:
```go
type Workspace struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    Name        string    `gorm:"not null;uniqueIndex:idx_owner_name" json:"name"`
    Owner       string    `gorm:"not null;index;uniqueIndex:idx_owner_name" json:"owner"`
    StorageSize string    `gorm:"default:'10Gi'" json:"storageSize"`
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
}
```

2. Add repository methods in `pkg/repositories/`:
```go
func (r *WorkspaceRepository) Create(workspace *models.Workspace) error {
    return r.db.Create(workspace).Error
}

func (r *WorkspaceRepository) FindByOwner(owner string) ([]models.Workspace, error) {
    var workspaces []models.Workspace
    err := r.db.Where("owner = ?", owner).Find(&workspaces).Error
    return workspaces, err
}
```

3. Run migrations:
```bash
# Auto-migrate on server startup
# Or create migration file in deploy/migrations/
```

### Web UI Development

**File structure:**
```
web/
  src/
    pages/          # Page components
    components/     # Reusable components
    services/       # API client
    contexts/       # React contexts
    App.tsx         # Main app component
```

**Adding a new page:**

1. Create page component in `src/pages/`:
```typescript
// src/pages/MyNewPage.tsx
import React, { useEffect, useState } from 'react';
import { fetchWorkspaces } from '../services/api';

export const MyNewPage: React.FC = () => {
  const [workspaces, setWorkspaces] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchWorkspaces()
      .then(data => {
        setWorkspaces(data);
        setLoading(false);
      })
      .catch(err => {
        console.error('Failed to fetch workspaces:', err);
        setLoading(false);
      });
  }, []);

  if (loading) {
    return <div>Loading...</div>;
  }

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold mb-4">My New Page</h1>
      {/* Your content here */}
    </div>
  );
};
```

2. Add route in `App.tsx`:
```typescript
<Route path="/my-new-page" element={
  <ProtectedRoute>
    <MyNewPage />
  </ProtectedRoute>
} />
```

3. Add navigation link in `components/Sidebar.tsx`:
```typescript
<Link to="/my-new-page" className="nav-link">
  My New Page
</Link>
```

**Adding API client method:**

In `src/services/api.ts`:
```typescript
export const fetchWorkspaces = async (): Promise<Workspace[]> => {
  const response = await fetch(`${API_BASE_URL}/api/workspaces`, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error('Failed to fetch workspaces');
  }

  return response.json();
};
```

### Operator Development

**File structure:**
```
cmd/operator/
  main.go           # Operator entry point
  controller.go     # Reconciliation logic
  types.go          # CRD types
```

**Testing operator locally:**

```bash
# Build operator
make build-operator

# Run locally (out-of-cluster mode)
export KUBECONFIG=~/.kube/config
./kuberde-operator

# Watch logs
tail -f /tmp/kuberde-operator.log
```

**Creating test CRs:**

```yaml
# test-agent.yaml
apiVersion: kuberde.io/v1beta1
kind: RDEAgent
metadata:
  name: test-agent
  namespace: kuberde-dev
spec:
  owner: testuser
  template: default
  ttl: 3600
  resources:
    cpu: "1000m"
    memory: "2Gi"
```

```bash
kubectl apply -f test-agent.yaml
kubectl get rdeagents -n kuberde-dev
kubectl describe rdeagent test-agent -n kuberde-dev
```

### CLI Development

**File structure:**
```
cmd/cli/
  main.go           # CLI entry point
  cmd/              # Command implementations
    login.go
    connect.go
    config.go
```

**Testing CLI:**

```bash
# Build CLI
go build -o kuberde-cli ./cmd/cli

# Test login
./kuberde-cli login

# Test connect
./kuberde-cli connect user-testuser-dev
```

**Adding new CLI command:**

1. Create command file in `cmd/cli/cmd/`:
```go
// cmd/cli/cmd/workspaces.go
package cmd

import (
    "github.com/spf13/cobra"
)

var workspacesCmd = &cobra.Command{
    Use:   "workspaces",
    Short: "Manage workspaces",
    Run: func(cmd *cobra.Command, args []string) {
        // Implementation
    },
}

func init() {
    rootCmd.AddCommand(workspacesCmd)
}
```

## Debugging

### Server Debugging

**VS Code launch.json:**
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch Server",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/server",
      "env": {
        "DATABASE_URL": "postgres://kuberde:kuberde@localhost:5432/kuberde?sslmode=disable",
        "KEYCLOAK_URL": "http://localhost:8081/realms/kuberde"
      },
      "args": []
    }
  ]
}
```

**Delve debugger:**
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug server
dlv debug ./cmd/server -- --config config.yaml

# Commands in delve
(dlv) break main.main
(dlv) continue
(dlv) next
(dlv) print variable
```

### Web UI Debugging

**Browser DevTools:**
1. Open Chrome DevTools (F12)
2. Go to Sources tab
3. Set breakpoints in TypeScript files
4. Inspect Network tab for API calls

**React DevTools:**
```bash
# Install React DevTools browser extension
# Available for Chrome, Firefox, Edge
```

**Console logging:**
```typescript
console.log('Debug:', variable);
console.table(arrayData);
console.trace('Function call stack');
```

### Database Debugging

**Connect to PostgreSQL:**
```bash
# Using psql
psql postgres://kuberde:kuberde@localhost:5432/kuberde

# Common queries
\dt                           # List tables
\d users                      # Describe users table
SELECT * FROM users LIMIT 10; # Query users
```

**Enable query logging:**
```go
// In server main.go
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    Logger: logger.Default.LogMode(logger.Info), // Log all queries
})
```

### Network Debugging

**Inspect WebSocket traffic:**
```bash
# Using websocat
websocat -v ws://localhost:8080/ws

# Using wscat
wscat -c ws://localhost:8080/ws
```

**Inspect HTTP traffic:**
```bash
# Using curl with verbose output
curl -v http://localhost:8080/api/workspaces

# Using httpie
http -v localhost:8080/api/workspaces
```

## Testing

### Unit Tests

**Go unit tests:**
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test
go test -run TestWorkspaceCreate ./pkg/repositories/
```

**Example test:**
```go
func TestWorkspaceCreate(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    repo := NewWorkspaceRepository(db)

    // Test
    workspace := &models.Workspace{
        Name:  "test-workspace",
        Owner: "testuser",
    }
    err := repo.Create(workspace)

    // Assert
    if err != nil {
        t.Fatalf("Failed to create workspace: %v", err)
    }
    if workspace.ID == 0 {
        t.Error("Expected workspace ID to be set")
    }
}
```

**Web UI tests:**
```bash
cd web

# Run unit tests
npm test

# Run with coverage
npm test -- --coverage

# Run in watch mode
npm test -- --watch
```

### Integration Tests

**API integration tests:**
```bash
# Run integration tests
go test -tags=integration ./tests/integration/

# With test database
DATABASE_URL="postgres://test:test@localhost:5432/kuberde_test" \
  go test -tags=integration ./tests/integration/
```

### End-to-End Tests

**Using the CLI:**
```bash
#!/bin/bash
# e2e-test.sh

set -e

# Login
./kuberde-cli login

# Create workspace
curl -X POST http://localhost:8080/api/workspaces \
  -H "Authorization: Bearer $(cat ~/.frp/token.json | jq -r .access_token)" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-workspace","storageSize":"5Gi"}'

# Verify workspace exists
./kuberde-cli workspaces list | grep test-workspace

echo "E2E test passed!"
```

## Code Quality

### Linting

**Go linting:**
```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run

# Run with auto-fix
golangci-lint run --fix

# Configuration in .golangci.yml
```

**.golangci.yml:**
```yaml
linters:
  enable:
    - gofmt
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - structcheck
    - varcheck
    - ineffassign
    - deadcode
```

**TypeScript linting:**
```bash
cd web

# Run ESLint
npm run lint

# Fix auto-fixable issues
npm run lint:fix
```

### Formatting

**Go formatting:**
```bash
# Format all Go files
gofmt -w .

# Or use goimports (better)
go install golang.org/x/tools/cmd/goimports@latest
goimports -w .
```

**TypeScript/JavaScript formatting:**
```bash
cd web

# Format with Prettier
npm run format

# Check formatting
npm run format:check
```

### Pre-commit Hooks

Install pre-commit hooks:

```bash
# Install pre-commit
pip install pre-commit

# Install hooks
pre-commit install
```

**.pre-commit-config.yaml:**
```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
      - id: check-added-large-files

  - repo: https://github.com/golangci/golangci-lint
    rev: v1.54.0
    hooks:
      - id: golangci-lint
```

## Contributing

### Workflow

1. **Fork and clone**
```bash
git clone https://github.com/YOUR_USERNAME/kube-rde.git
cd kube-rde
git remote add upstream https://github.com/xsoloking/kube-rde.git
```

2. **Create feature branch**
```bash
git checkout -b feature/my-new-feature
```

3. **Make changes and commit**
```bash
# Make your changes
git add .
git commit -m "feat: add new feature"
```

4. **Run tests and linting**
```bash
make test
make lint
```

5. **Push and create PR**
```bash
git push origin feature/my-new-feature
# Create PR on GitHub
```

### Commit Message Guidelines

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add workspace templates
fix: correct agent connection timeout
docs: update installation guide
style: format code with gofmt
refactor: simplify authentication logic
test: add unit tests for workspace creation
chore: update dependencies
```

### Code Review Checklist

- [ ] Code follows project style guide
- [ ] Tests added for new functionality
- [ ] Documentation updated
- [ ] No security vulnerabilities introduced
- [ ] Performance impact considered
- [ ] Backward compatibility maintained

## Troubleshooting

### Common Issues

**Issue: Database connection fails**
```bash
# Check PostgreSQL is running
docker ps | grep postgres

# Check connection string
echo $DATABASE_URL

# Test connection
psql $DATABASE_URL -c "SELECT 1;"
```

**Issue: Keycloak not accessible**
```bash
# Check Keycloak container
docker logs kuberde-keycloak

# Verify URL
curl http://localhost:8081/realms/kuberde/.well-known/openid-configuration
```

**Issue: Web UI can't connect to API**
```bash
# Check CORS configuration in server
# Ensure KUBERDE_PUBLIC_URL is set correctly

# Check browser console for CORS errors
# F12 -> Console tab
```

**Issue: Agent can't connect to server**
```bash
# Check server WebSocket endpoint
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: test" \
  http://localhost:8080/ws

# Check agent logs
go run ./cmd/agent 2>&1 | tee agent.log
```

### Performance Profiling

**CPU profiling:**
```bash
# Enable pprof in server
import _ "net/http/pprof"

# Collect profile
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# Analyze
(pprof) top10
(pprof) web
```

**Memory profiling:**
```bash
go tool pprof http://localhost:8080/debug/pprof/heap

(pprof) top10
(pprof) list functionName
```

## Resources

- [Go Documentation](https://golang.org/doc/)
- [React Documentation](https://react.dev/)
- [Kubernetes Client-Go](https://github.com/kubernetes/client-go)
- [GORM Documentation](https://gorm.io/docs/)
- [Cobra CLI Framework](https://github.com/spf13/cobra)

## Next Steps

- [Testing Guide](TESTING.md)
- [Security Best Practices](../SECURITY.md)
- [Contributing Guidelines](../../CONTRIBUTING.md)
- [API Reference](../API.md)
