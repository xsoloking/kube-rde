# Multi-Tenant Team Design (A2 Shared Control Plane)

Date: 2026-01-22

## Overview

Implement multi-tenant support for KubeRDE using a shared control plane architecture. Tenants are represented as "Teams" - organizational units that share a resource quota pool.

## Key Decisions

| Aspect | Decision |
|--------|----------|
| Tenant concept | Team |
| User-Team relation | Many users → One team (N:1) |
| Quota model | Team quota only (shared pool, no user-level quota) |
| Resource types | Defined in ResourceConfig, teams set limits per type |
| Network isolation | None (trusted internal teams) |
| Namespace | One per team (`kuberde-{team-name}`) |
| K8s enforcement | Native ResourceQuota |
| Backwards compatibility | Not required (new platform) |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   kuberde-system                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │  Server  │  │ Operator │  │PostgreSQL│  │Keycloak │ │
│  └────┬─────┘  └────┬─────┘  └──────────┘  └─────────┘ │
└───────┼─────────────┼────────────────────────────────────┘
        │             │ watches all team namespaces
        ▼             ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ kuberde-ai-team │  │kuberde-backend  │  │ kuberde-data    │
│                 │  │                 │  │                 │
│ - ResourceQuota │  │ - ResourceQuota │  │ - ResourceQuota │
│ - User1 agents  │  │ - User3 agents  │  │ - User5 agents  │
│ - User2 agents  │  │ - User4 agents  │  │ - User6 agents  │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

## Data Model

### Team (new)

```go
type Team struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    Name        string    `gorm:"uniqueIndex;not null" json:"name"`     // e.g., "ai-team"
    DisplayName string    `json:"displayName"`                           // e.g., "AI Research Team"
    Namespace   string    `gorm:"uniqueIndex;not null" json:"namespace"` // e.g., "kuberde-ai-team"
    Status      string    `json:"status"`                                // active, suspended
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
}
```

### TeamQuota (new)

```go
type TeamQuota struct {
    ID               uint   `gorm:"primaryKey" json:"id"`
    TeamID           uint   `gorm:"index;not null" json:"teamId"`
    ResourceConfigID uint   `gorm:"not null" json:"resourceConfigId"`
    Quota            int    `json:"quota"`  // limit value

    Team           Team           `gorm:"foreignKey:TeamID"`
    ResourceConfig ResourceConfig `gorm:"foreignKey:ResourceConfigID"`
}
```

### User (modified)

```go
type User struct {
    // ... existing fields ...
    TeamID  *uint  `json:"teamId"`            // NEW: foreign key to Team
    Team    Team   `gorm:"foreignKey:TeamID"` // NEW: relation
}
```

### Workspace (modified)

```go
type Workspace struct {
    // ... existing fields ...
    TeamID  uint   `json:"teamId"`            // NEW: foreign key to Team
    Team    Team   `gorm:"foreignKey:TeamID"` // NEW: relation
}
```

### Entity Relationships

```
Team (1) ◄─────── User (many)
  │
  │ has
  ▼
TeamQuota ─────► ResourceConfig (references types)
  │
  │ owns namespace
  ▼
Kubernetes Namespace
  └── ResourceQuota (enforced by K8s)
  └── Workspaces/Services (owned by users)
```

## Kubernetes Resource Mapping

When a Team is created, automatically provision:

```yaml
# 1. Namespace
apiVersion: v1
kind: Namespace
metadata:
  name: kuberde-ai-team
  labels:
    kuberde.io/team: ai-team

---
# 2. ResourceQuota (generated from TeamQuota entries)
apiVersion: v1
kind: ResourceQuota
metadata:
  name: team-quota
  namespace: kuberde-ai-team
spec:
  hard:
    requests.cpu: "100"
    requests.memory: "200Gi"
    requests.storage: "1Ti"
    pods: "50"
    requests.nvidia.com/gpu: "4"
```

## Control Flow

### Team Creation Flow

```
Admin creates Team (UI/API)
         │
         ▼
Server creates Team record in PostgreSQL
         │
         ▼
Server creates Kubernetes Namespace with label
         │
         ▼
Admin sets TeamQuota per resource type (UI/API)
         │
         ▼
Server generates & applies ResourceQuota to namespace
```

### Workspace Creation Flow

```
User creates Workspace (UI/API)
         │
         ▼
Server looks up user's team
         │
         ▼
Server gets team's namespace ("kuberde-ai-team")
         │
         ▼
Server creates RDEAgent CR in team namespace
         │
         ▼
Operator (watching all team namespaces) detects CR
         │
         ▼
Operator creates Deployment + PVC in same namespace
         │
         ▼
K8s ResourceQuota enforces team limits automatically
         │
         ▼
If quota exceeded → Pod stays Pending, error shown to user
```

### Agent ID Format

```
Current:  user-{owner}-{workspace}
New:      {team}-{owner}-{workspace}

Example:  ai-team-alice-dev-env
```

## UI Changes

### New Pages

| Page | Purpose | Access |
|------|---------|--------|
| TeamManagement | List/create/delete teams | Admin only |
| TeamDetail | View team info, members, quota usage | Admin only |
| TeamQuotaEdit | Set quota per resource type (auto-shows all types) | Admin only |

### Modified Pages

| Page | Changes |
|------|---------|
| UserManagement | Add team column display |
| UserEdit | Add team assignment dropdown |
| Dashboard | Show user's team info and quota usage |
| WorkspaceCreate | Auto-use user's team namespace |

### Removed

- UserEdit quota section (no individual quota)
- User SYNC button (no user-level quota)

### TeamQuotaEdit UI

Auto-displays all resource types from ResourceConfig:

```
┌─────────────────────────────────────────────────┐
│  Team Quota: AI Team                            │
├─────────────────────────────────────────────────┤
│  Resource Type          │  Quota    │  Used     │
├─────────────────────────────────────────────────┤
│  CPU (cores)            │  [100]    │  45       │
│  Memory (Gi)            │  [200]    │  120      │
│  Storage (Gi)           │  [1000]   │  500      │
│  A100 GPU               │  [4]      │  2        │
│  H100 GPU (NEW)         │  [  ]     │  0        │
└─────────────────────────────────────────────────┘
                                      [Save]
```

No SYNC button needed - new resource types appear automatically.

## Operator Changes

| Aspect | Current | New |
|--------|---------|-----|
| Watch scope | Single namespace | All namespaces with label `kuberde.io/team` |
| RDEAgent namespace | Fixed "kuberde" | Dynamic from user's team |
| RBAC | Namespace-scoped Role | ClusterRole |

### ClusterRole for Operator

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kuberde-operator
rules:
- apiGroups: ["kuberde.io"]
  resources: ["rdeagents", "rdeagents/status"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch"]
```

## Database Schema

```sql
-- Create teams table
CREATE TABLE teams (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) UNIQUE NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create team_quotas table
CREATE TABLE team_quotas (
    id SERIAL PRIMARY KEY,
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    resource_config_id INTEGER NOT NULL REFERENCES resource_configs(id),
    quota INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(team_id, resource_config_id)
);

-- Add team_id to users
ALTER TABLE users ADD COLUMN team_id INTEGER REFERENCES teams(id);

-- Add team_id to workspaces
ALTER TABLE workspaces ADD COLUMN team_id INTEGER REFERENCES teams(id);
```

## Implementation Estimate

| Component | Changes | Lines |
|-----------|---------|-------|
| Models | Add Team, TeamQuota; modify User, Workspace | ~150 |
| Server API | Team CRUD, TeamQuota CRUD, modify workspace creation | ~500 |
| Operator | ClusterRole RBAC, watch all team namespaces | ~100 |
| Web UI | 3 new pages, modify UserEdit, Dashboard | ~800 |
| **Total** | | **~1550 lines** |

Estimated time: **3 weeks**

## Not Included (Removed from Original A2)

- UserQuota logic (team quota only)
- NetworkPolicy templates (no isolation needed)
- Multi-realm Keycloak (single realm with team groups)
- User quota sync button
- Migration scripts (no historical data)
- Backwards compatibility handling
