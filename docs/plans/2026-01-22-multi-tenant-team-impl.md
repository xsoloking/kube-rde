# Multi-Tenant Team Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement multi-tenant team support with shared control plane architecture - teams own namespaces, users belong to teams, quotas enforced at team level.

**Architecture:** Add Team and TeamQuota models. Teams map to Kubernetes namespaces with ResourceQuota. Users belong to one team. Operator watches all team namespaces. Remove user-level quota enforcement.

**Tech Stack:** Go 1.24+, GORM, React 18+, TypeScript, Kubernetes client-go

---

## Task 1: Add Team Model

**Files:**
- Modify: `pkg/models/models.go`

**Step 1: Add Team struct after AuditLog**

Add to `pkg/models/models.go` after line 163 (after AuditLog struct):

```go
// Team represents an organizational unit that owns a Kubernetes namespace
type Team struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex;not null" json:"name"`     // e.g., "ai-team"
	DisplayName string    `json:"display_name"`                          // e.g., "AI Research Team"
	Namespace   string    `gorm:"uniqueIndex;not null" json:"namespace"` // e.g., "kuberde-ai-team"
	Status      string    `gorm:"default:'active'" json:"status"`        // active, suspended
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName specifies the table name for Team
func (Team) TableName() string {
	return "teams"
}
```

**Step 2: Add TeamQuota struct after Team**

```go
// TeamQuota represents a resource quota for a team
type TeamQuota struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	TeamID           uint           `gorm:"index;not null" json:"team_id"`
	ResourceConfigID int            `gorm:"not null" json:"resource_config_id"`
	Quota            int            `gorm:"not null;default:0" json:"quota"`
	Team             Team           `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	ResourceConfig   ResourceConfig `gorm:"foreignKey:ResourceConfigID" json:"resource_config,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// TableName specifies the table name for TeamQuota
func (TeamQuota) TableName() string {
	return "team_quotas"
}
```

**Step 3: Modify User struct to add TeamID**

Find the User struct and add TeamID field after `FullName`:

```go
type User struct {
	ID                 string           `gorm:"primaryKey" json:"id"`
	Username           string           `gorm:"uniqueIndex" json:"username"`
	Email              string           `json:"email"`
	FullName           string           `json:"full_name"`
	TeamID             *uint            `json:"team_id,omitempty"`                                // NEW: Team membership
	Team               *Team            `gorm:"foreignKey:TeamID" json:"team,omitempty"`         // NEW: Team relation
	DefaultWorkspaceID sql.NullString   `json:"default_workspace_id,omitempty"`
	// ... rest unchanged
}
```

**Step 4: Modify Workspace struct to add TeamID**

Find the Workspace struct and add TeamID field after `OwnerID`:

```go
type Workspace struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	OwnerID      string    `gorm:"index" json:"owner_id"`
	Owner        *User     `gorm:"foreignKey:OwnerID;references:ID" json:"owner,omitempty"`
	TeamID       *uint     `json:"team_id,omitempty"`                                 // NEW: Team ownership
	Team         *Team     `gorm:"foreignKey:TeamID" json:"team,omitempty"`           // NEW: Team relation
	// ... rest unchanged
}
```

**Step 5: Commit**

```bash
git add pkg/models/models.go
git commit -m "feat: add Team and TeamQuota models with User/Workspace relations"
```

---

## Task 2: Add Team Repository

**Files:**
- Create: `pkg/repositories/team_repository.go`

**Step 1: Create team repository file**

```go
package repositories

import (
	"kuberde/pkg/models"

	"gorm.io/gorm"
)

type TeamRepository interface {
	Create(team *models.Team) error
	GetByID(id uint) (*models.Team, error)
	GetByName(name string) (*models.Team, error)
	GetByNamespace(namespace string) (*models.Team, error)
	List() ([]models.Team, error)
	Update(team *models.Team) error
	Delete(id uint) error
	GetMembers(teamID uint) ([]models.User, error)
}

type teamRepository struct {
	db *gorm.DB
}

func NewTeamRepository(db *gorm.DB) TeamRepository {
	return &teamRepository{db: db}
}

func (r *teamRepository) Create(team *models.Team) error {
	return r.db.Create(team).Error
}

func (r *teamRepository) GetByID(id uint) (*models.Team, error) {
	var team models.Team
	if err := r.db.First(&team, id).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func (r *teamRepository) GetByName(name string) (*models.Team, error) {
	var team models.Team
	if err := r.db.Where("name = ?", name).First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func (r *teamRepository) GetByNamespace(namespace string) (*models.Team, error) {
	var team models.Team
	if err := r.db.Where("namespace = ?", namespace).First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func (r *teamRepository) List() ([]models.Team, error) {
	var teams []models.Team
	if err := r.db.Order("name ASC").Find(&teams).Error; err != nil {
		return nil, err
	}
	return teams, nil
}

func (r *teamRepository) Update(team *models.Team) error {
	return r.db.Save(team).Error
}

func (r *teamRepository) Delete(id uint) error {
	return r.db.Delete(&models.Team{}, id).Error
}

func (r *teamRepository) GetMembers(teamID uint) ([]models.User, error) {
	var users []models.User
	if err := r.db.Where("team_id = ?", teamID).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
```

**Step 2: Commit**

```bash
git add pkg/repositories/team_repository.go
git commit -m "feat: add TeamRepository for CRUD operations"
```

---

## Task 3: Add TeamQuota Repository

**Files:**
- Create: `pkg/repositories/team_quota_repository.go`

**Step 1: Create team quota repository file**

```go
package repositories

import (
	"kuberde/pkg/models"

	"gorm.io/gorm"
)

type TeamQuotaRepository interface {
	Create(quota *models.TeamQuota) error
	GetByID(id uint) (*models.TeamQuota, error)
	GetByTeamID(teamID uint) ([]models.TeamQuota, error)
	GetByTeamAndResource(teamID uint, resourceConfigID int) (*models.TeamQuota, error)
	Update(quota *models.TeamQuota) error
	Delete(id uint) error
	DeleteByTeamID(teamID uint) error
	UpsertBatch(teamID uint, quotas []models.TeamQuota) error
}

type teamQuotaRepository struct {
	db *gorm.DB
}

func NewTeamQuotaRepository(db *gorm.DB) TeamQuotaRepository {
	return &teamQuotaRepository{db: db}
}

func (r *teamQuotaRepository) Create(quota *models.TeamQuota) error {
	return r.db.Create(quota).Error
}

func (r *teamQuotaRepository) GetByID(id uint) (*models.TeamQuota, error) {
	var quota models.TeamQuota
	if err := r.db.Preload("ResourceConfig").First(&quota, id).Error; err != nil {
		return nil, err
	}
	return &quota, nil
}

func (r *teamQuotaRepository) GetByTeamID(teamID uint) ([]models.TeamQuota, error) {
	var quotas []models.TeamQuota
	if err := r.db.Preload("ResourceConfig").Where("team_id = ?", teamID).Find(&quotas).Error; err != nil {
		return nil, err
	}
	return quotas, nil
}

func (r *teamQuotaRepository) GetByTeamAndResource(teamID uint, resourceConfigID int) (*models.TeamQuota, error) {
	var quota models.TeamQuota
	if err := r.db.Where("team_id = ? AND resource_config_id = ?", teamID, resourceConfigID).First(&quota).Error; err != nil {
		return nil, err
	}
	return &quota, nil
}

func (r *teamQuotaRepository) Update(quota *models.TeamQuota) error {
	return r.db.Save(quota).Error
}

func (r *teamQuotaRepository) Delete(id uint) error {
	return r.db.Delete(&models.TeamQuota{}, id).Error
}

func (r *teamQuotaRepository) DeleteByTeamID(teamID uint) error {
	return r.db.Where("team_id = ?", teamID).Delete(&models.TeamQuota{}).Error
}

func (r *teamQuotaRepository) UpsertBatch(teamID uint, quotas []models.TeamQuota) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, q := range quotas {
			q.TeamID = teamID
			result := tx.Where("team_id = ? AND resource_config_id = ?", teamID, q.ResourceConfigID).
				Assign(models.TeamQuota{Quota: q.Quota}).
				FirstOrCreate(&q)
			if result.Error != nil {
				return result.Error
			}
		}
		return nil
	})
}
```

**Step 2: Commit**

```bash
git add pkg/repositories/team_quota_repository.go
git commit -m "feat: add TeamQuotaRepository for team resource quota management"
```

---

## Task 4: Add Database Auto-Migration for Team Tables

**Files:**
- Modify: `pkg/db/db.go`

**Step 1: Add Team and TeamQuota to AutoMigrate**

Find the AutoMigrate call and add the new models:

```go
err = db.AutoMigrate(
	&models.User{},
	&models.Workspace{},
	&models.Service{},
	&models.AgentTemplate{},
	&models.AuditLog{},
	&models.ResourceConfig{},
	&models.UserQuota{},
	&models.Team{},      // NEW
	&models.TeamQuota{}, // NEW
)
```

**Step 2: Commit**

```bash
git add pkg/db/db.go
git commit -m "feat: add Team and TeamQuota to database auto-migration"
```

---

## Task 5: Add Team API Handlers - Part 1 (List & Create)

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Add team repository variable**

Find the repository instances section (around line 71) and add:

```go
// Repository instances
resourceConfigRepo repositories.ResourceConfigRepository
userQuotaRepo      repositories.UserQuotaRepository
teamRepo           repositories.TeamRepository      // NEW
teamQuotaRepo      repositories.TeamQuotaRepository // NEW
```

**Step 2: Initialize team repositories in main()**

Find where repositories are initialized and add:

```go
teamRepo = repositories.NewTeamRepository(db)
teamQuotaRepo = repositories.NewTeamQuotaRepository(db)
```

**Step 3: Register team API routes**

Find the route registration section and add:

```go
// Team management (admin only)
http.HandleFunc("/api/admin/teams", adminMiddleware(handleTeams))
http.HandleFunc("/api/admin/teams/", adminMiddleware(handleTeamByID))
```

**Step 4: Add handleTeams handler**

```go
func handleTeams(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleListTeams(w, r)
	case http.MethodPost:
		handleCreateTeam(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleListTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := teamRepo.List()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list teams: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"teams": teams,
	})
}

func handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	// Generate namespace name
	namespace := fmt.Sprintf("kuberde-%s", req.Name)

	team := &models.Team{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Namespace:   namespace,
		Status:      "active",
	}

	if err := teamRepo.Create(team); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create team: %v", err), http.StatusInternalServerError)
		return
	}

	// Create Kubernetes namespace
	if err := createTeamNamespace(team); err != nil {
		log.Printf("Warning: Failed to create namespace for team %s: %v", team.Name, err)
		// Don't fail the request, namespace can be created later
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(team)
}

func createTeamNamespace(team *models.Team) error {
	if k8sClientset == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: team.Namespace,
			Labels: map[string]string{
				"kuberde.io/team":      team.Name,
				"kuberde.io/component": "team-namespace",
			},
		},
	}

	_, err := k8sClientset.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
}
```

**Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: add Team list and create API handlers"
```

---

## Task 6: Add Team API Handlers - Part 2 (Get, Update, Delete)

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Add handleTeamByID handler**

```go
func handleTeamByID(w http.ResponseWriter, r *http.Request) {
	// Extract team ID from path: /api/admin/teams/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/teams/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Team ID required", http.StatusBadRequest)
		return
	}

	teamID, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		http.Error(w, "Invalid team ID", http.StatusBadRequest)
		return
	}

	// Check for sub-routes
	if len(parts) > 1 {
		switch parts[1] {
		case "quota":
			handleTeamQuota(w, r, uint(teamID))
			return
		case "members":
			handleTeamMembers(w, r, uint(teamID))
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		handleGetTeam(w, r, uint(teamID))
	case http.MethodPut:
		handleUpdateTeam(w, r, uint(teamID))
	case http.MethodDelete:
		handleDeleteTeam(w, r, uint(teamID))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleGetTeam(w http.ResponseWriter, r *http.Request, teamID uint) {
	team, err := teamRepo.GetByID(teamID)
	if err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	// Get team members count
	members, _ := teamRepo.GetMembers(teamID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"team":         team,
		"member_count": len(members),
	})
}

func handleUpdateTeam(w http.ResponseWriter, r *http.Request, teamID uint) {
	team, err := teamRepo.GetByID(teamID)
	if err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		Status      string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.DisplayName != "" {
		team.DisplayName = req.DisplayName
	}
	if req.Status != "" {
		team.Status = req.Status
	}

	if err := teamRepo.Update(team); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update team: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(team)
}

func handleDeleteTeam(w http.ResponseWriter, r *http.Request, teamID uint) {
	team, err := teamRepo.GetByID(teamID)
	if err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	// Check if team has members
	members, _ := teamRepo.GetMembers(teamID)
	if len(members) > 0 {
		http.Error(w, "Cannot delete team with members", http.StatusConflict)
		return
	}

	// Delete team quotas first
	teamQuotaRepo.DeleteByTeamID(teamID)

	// Delete team
	if err := teamRepo.Delete(teamID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete team: %v", err), http.StatusInternalServerError)
		return
	}

	// Optionally delete namespace (be careful in production)
	// deleteTeamNamespace(team.Namespace)

	w.WriteHeader(http.StatusNoContent)
}

func handleTeamMembers(w http.ResponseWriter, r *http.Request, teamID uint) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	members, err := teamRepo.GetMembers(teamID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get team members: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"members": members,
	})
}
```

**Step 2: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: add Team get, update, delete, and members API handlers"
```

---

## Task 7: Add TeamQuota API Handlers

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Add handleTeamQuota handler**

```go
func handleTeamQuota(w http.ResponseWriter, r *http.Request, teamID uint) {
	switch r.Method {
	case http.MethodGet:
		handleGetTeamQuota(w, r, teamID)
	case http.MethodPut:
		handleUpdateTeamQuota(w, r, teamID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleGetTeamQuota(w http.ResponseWriter, r *http.Request, teamID uint) {
	// Get team to verify it exists
	team, err := teamRepo.GetByID(teamID)
	if err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	// Get all resource configs
	resourceConfig, err := resourceConfigRepo.Get()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get resource config: %v", err), http.StatusInternalServerError)
		return
	}

	// Get existing team quotas
	existingQuotas, err := teamQuotaRepo.GetByTeamID(teamID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get team quotas: %v", err), http.StatusInternalServerError)
		return
	}

	// Build quota map for lookup
	quotaMap := make(map[int]int)
	for _, q := range existingQuotas {
		quotaMap[q.ResourceConfigID] = q.Quota
	}

	// Parse GPU types from resource config
	var gpuTypes []struct {
		ModelName      string `json:"model_name"`
		ResourceName   string `json:"resource_name"`
		NodeLabelKey   string `json:"node_label_key"`
		NodeLabelValue string `json:"node_label_value"`
		Limit          int    `json:"limit"`
	}
	json.Unmarshal([]byte(resourceConfig.GPUTypes), &gpuTypes)

	// Build response with all resource types
	type QuotaItem struct {
		ResourceConfigID int    `json:"resource_config_id"`
		ResourceType     string `json:"resource_type"`
		ResourceName     string `json:"resource_name"`
		DisplayName      string `json:"display_name"`
		Quota            int    `json:"quota"`
		Unit             string `json:"unit"`
	}

	quotaItems := []QuotaItem{
		{ResourceConfigID: 1, ResourceType: "cpu", ResourceName: "cpu", DisplayName: "CPU Cores", Quota: quotaMap[1], Unit: "cores"},
		{ResourceConfigID: 2, ResourceType: "memory", ResourceName: "memory", DisplayName: "Memory", Quota: quotaMap[2], Unit: "Gi"},
		{ResourceConfigID: 3, ResourceType: "storage", ResourceName: "storage", DisplayName: "Storage", Quota: quotaMap[3], Unit: "Gi"},
	}

	// Add GPU types
	for i, gpu := range gpuTypes {
		quotaItems = append(quotaItems, QuotaItem{
			ResourceConfigID: 100 + i, // GPU resource IDs start at 100
			ResourceType:     "gpu",
			ResourceName:     gpu.ResourceName,
			DisplayName:      gpu.ModelName,
			Quota:            quotaMap[100+i],
			Unit:             "units",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"team":   team,
		"quotas": quotaItems,
	})
}

func handleUpdateTeamQuota(w http.ResponseWriter, r *http.Request, teamID uint) {
	// Verify team exists
	team, err := teamRepo.GetByID(teamID)
	if err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	var req struct {
		Quotas []struct {
			ResourceConfigID int `json:"resource_config_id"`
			Quota            int `json:"quota"`
		} `json:"quotas"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Convert to TeamQuota models
	quotas := make([]models.TeamQuota, len(req.Quotas))
	for i, q := range req.Quotas {
		quotas[i] = models.TeamQuota{
			TeamID:           teamID,
			ResourceConfigID: q.ResourceConfigID,
			Quota:            q.Quota,
		}
	}

	// Upsert quotas
	if err := teamQuotaRepo.UpsertBatch(teamID, quotas); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update team quotas: %v", err), http.StatusInternalServerError)
		return
	}

	// Apply ResourceQuota to Kubernetes namespace
	if err := applyTeamResourceQuota(team, req.Quotas); err != nil {
		log.Printf("Warning: Failed to apply ResourceQuota for team %s: %v", team.Name, err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Team quotas updated successfully",
	})
}

func applyTeamResourceQuota(team *models.Team, quotas []struct {
	ResourceConfigID int `json:"resource_config_id"`
	Quota            int `json:"quota"`
}) error {
	if k8sClientset == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	// Build ResourceQuota spec
	hard := corev1.ResourceList{}

	for _, q := range quotas {
		if q.Quota <= 0 {
			continue
		}
		switch q.ResourceConfigID {
		case 1: // CPU
			hard[corev1.ResourceRequestsCPU] = resource.MustParse(fmt.Sprintf("%d", q.Quota))
			hard[corev1.ResourceLimitsCPU] = resource.MustParse(fmt.Sprintf("%d", q.Quota))
		case 2: // Memory
			hard[corev1.ResourceRequestsMemory] = resource.MustParse(fmt.Sprintf("%dGi", q.Quota))
			hard[corev1.ResourceLimitsMemory] = resource.MustParse(fmt.Sprintf("%dGi", q.Quota))
		case 3: // Storage
			hard[corev1.ResourceRequestsStorage] = resource.MustParse(fmt.Sprintf("%dGi", q.Quota))
		default:
			// GPU types (ID >= 100)
			// Would need to look up resource name from config
		}
	}

	if len(hard) == 0 {
		return nil
	}

	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "team-quota",
			Namespace: team.Namespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hard,
		},
	}

	_, err := k8sClientset.CoreV1().ResourceQuotas(team.Namespace).Create(context.Background(), rq, metav1.CreateOptions{})
	if err != nil && strings.Contains(err.Error(), "already exists") {
		_, err = k8sClientset.CoreV1().ResourceQuotas(team.Namespace).Update(context.Background(), rq, metav1.UpdateOptions{})
	}
	return err
}
```

**Step 2: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: add TeamQuota API handlers with K8s ResourceQuota integration"
```

---

## Task 8: Modify Workspace Creation to Use Team

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Update handleCreateWorkspace to set team from user**

Find `handleCreateWorkspace` function (around line 3710) and modify to:

1. Look up user's team
2. Set workspace.TeamID
3. Use team's namespace for RDEAgent creation

Add this logic after getting the user info:

```go
// Get user's team
var user models.User
if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
	http.Error(w, "User not found", http.StatusNotFound)
	return
}

if user.TeamID == nil {
	http.Error(w, "User is not assigned to a team", http.StatusBadRequest)
	return
}

// Get team for namespace
team, err := teamRepo.GetByID(*user.TeamID)
if err != nil {
	http.Error(w, "Team not found", http.StatusNotFound)
	return
}

// Set workspace team
workspace.TeamID = user.TeamID
```

Then update the namespace used for PVC/RDEAgent creation from `kuberdeNamespace` to `team.Namespace`.

**Step 2: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: workspace creation uses user's team namespace"
```

---

## Task 9: Update Agent ID Format

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Update agent ID generation**

Find where agent IDs are generated (search for `user-%s-%s`) and update format to include team:

```go
// Old format: user-{owner}-{workspace}
// New format: {team}-{owner}-{workspace}
agentID := fmt.Sprintf("%s-%s-%s", team.Name, username, serviceName)
```

**Step 2: Update agent ID parsing (if exists)**

Search for places that parse agent IDs and update to handle new format.

**Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: update agent ID format to include team name"
```

---

## Task 10: Frontend - Add Team Types and API Client

**Files:**
- Modify: `web/services/api.ts`

**Step 1: Add Team types**

Add after the UserQuota interface:

```typescript
// Team Types
export interface Team {
  id: number;
  name: string;
  display_name: string;
  namespace: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface TeamQuotaItem {
  resource_config_id: number;
  resource_type: string;
  resource_name: string;
  display_name: string;
  quota: number;
  unit: string;
}

export interface CreateTeamRequest {
  name: string;
  display_name: string;
}

export interface UpdateTeamRequest {
  display_name?: string;
  status?: string;
}

export interface UpdateTeamQuotaRequest {
  quotas: Array<{
    resource_config_id: number;
    quota: number;
  }>;
}
```

**Step 2: Add Team API client**

```typescript
// Teams API (admin only)
export const teamsApi = {
  list: () => api.get<{ teams: Team[] }>('/api/admin/teams').then((res) => res.teams),
  get: (id: number) => api.get<{ team: Team; member_count: number }>(`/api/admin/teams/${id}`),
  create: (team: CreateTeamRequest) => api.post<Team>('/api/admin/teams', team),
  update: (id: number, team: UpdateTeamRequest) => api.put<Team>(`/api/admin/teams/${id}`, team),
  delete: (id: number) => api.delete(`/api/admin/teams/${id}`),
  getMembers: (id: number) => api.get<{ members: User[] }>(`/api/admin/teams/${id}/members`),
  getQuota: (id: number) =>
    api.get<{ team: Team; quotas: TeamQuotaItem[] }>(`/api/admin/teams/${id}/quota`),
  updateQuota: (id: number, quota: UpdateTeamQuotaRequest) =>
    api.put(`/api/admin/teams/${id}/quota`, quota),
};
```

**Step 3: Update User interface to include team**

```typescript
export interface User {
  id: string;
  username: string;
  email: string;
  full_name?: string;
  name?: string;
  roles: string[];
  enabled: boolean;
  created: number;
  created_at?: string;
  ssh_keys?: SSHKey[];
  team_id?: number;    // NEW
  team?: Team;         // NEW
}
```

**Step 4: Commit**

```bash
git add web/services/api.ts
git commit -m "feat: add Team types and API client"
```

---

## Task 11: Frontend - Create TeamManagement Page

**Files:**
- Create: `web/pages/TeamManagement.tsx`

**Step 1: Create the page**

```tsx
import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { teamsApi, Team } from '../services/api';

export default function TeamManagement() {
  const [teams, setTeams] = useState<Team[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newTeam, setNewTeam] = useState({ name: '', display_name: '' });
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    loadTeams();
  }, []);

  const loadTeams = async () => {
    try {
      setLoading(true);
      const data = await teamsApi.list();
      setTeams(data);
    } catch (err) {
      setError('Failed to load teams');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!newTeam.name.trim()) return;
    try {
      setCreating(true);
      await teamsApi.create(newTeam);
      setShowCreateModal(false);
      setNewTeam({ name: '', display_name: '' });
      loadTeams();
    } catch (err) {
      setError('Failed to create team');
      console.error(err);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to delete this team?')) return;
    try {
      await teamsApi.delete(id);
      loadTeams();
    } catch (err) {
      setError('Failed to delete team. Make sure the team has no members.');
      console.error(err);
    }
  };

  if (loading) {
    return <div className="p-6">Loading...</div>;
  }

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Team Management</h1>
        <button
          onClick={() => setShowCreateModal(true)}
          className="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700"
        >
          Create Team
        </button>
      </div>

      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
          <button onClick={() => setError(null)} className="float-right">&times;</button>
        </div>
      )}

      <div className="bg-white shadow rounded-lg overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Display Name</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Namespace</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {teams.map((team) => (
              <tr key={team.id}>
                <td className="px-6 py-4 whitespace-nowrap">{team.name}</td>
                <td className="px-6 py-4 whitespace-nowrap">{team.display_name}</td>
                <td className="px-6 py-4 whitespace-nowrap font-mono text-sm">{team.namespace}</td>
                <td className="px-6 py-4 whitespace-nowrap">
                  <span className={`px-2 py-1 rounded text-xs ${
                    team.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                  }`}>
                    {team.status}
                  </span>
                </td>
                <td className="px-6 py-4 whitespace-nowrap space-x-2">
                  <Link
                    to={`/admin/teams/${team.id}/quota`}
                    className="text-blue-600 hover:text-blue-800"
                  >
                    Quota
                  </Link>
                  <button
                    onClick={() => handleDelete(team.id)}
                    className="text-red-600 hover:text-red-800"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Create Team Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center">
          <div className="bg-white p-6 rounded-lg w-96">
            <h2 className="text-xl font-bold mb-4">Create Team</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-1">Name (lowercase, no spaces)</label>
                <input
                  type="text"
                  value={newTeam.name}
                  onChange={(e) => setNewTeam({ ...newTeam, name: e.target.value.toLowerCase().replace(/\s/g, '-') })}
                  className="w-full border rounded px-3 py-2"
                  placeholder="ai-team"
                />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1">Display Name</label>
                <input
                  type="text"
                  value={newTeam.display_name}
                  onChange={(e) => setNewTeam({ ...newTeam, display_name: e.target.value })}
                  className="w-full border rounded px-3 py-2"
                  placeholder="AI Research Team"
                />
              </div>
            </div>
            <div className="flex justify-end space-x-2 mt-6">
              <button
                onClick={() => setShowCreateModal(false)}
                className="px-4 py-2 border rounded hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={creating || !newTeam.name.trim()}
                className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
              >
                {creating ? 'Creating...' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add web/pages/TeamManagement.tsx
git commit -m "feat: add TeamManagement page"
```

---

## Task 12: Frontend - Create TeamQuotaEdit Page

**Files:**
- Create: `web/pages/TeamQuotaEdit.tsx`

**Step 1: Create the page**

```tsx
import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { teamsApi, Team, TeamQuotaItem } from '../services/api';

export default function TeamQuotaEdit() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [team, setTeam] = useState<Team | null>(null);
  const [quotas, setQuotas] = useState<TeamQuotaItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (id) {
      loadTeamQuota(parseInt(id));
    }
  }, [id]);

  const loadTeamQuota = async (teamId: number) => {
    try {
      setLoading(true);
      const data = await teamsApi.getQuota(teamId);
      setTeam(data.team);
      setQuotas(data.quotas);
    } catch (err) {
      setError('Failed to load team quota');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleQuotaChange = (index: number, value: number) => {
    const newQuotas = [...quotas];
    newQuotas[index] = { ...newQuotas[index], quota: value };
    setQuotas(newQuotas);
  };

  const handleSave = async () => {
    if (!id) return;
    try {
      setSaving(true);
      await teamsApi.updateQuota(parseInt(id), {
        quotas: quotas.map((q) => ({
          resource_config_id: q.resource_config_id,
          quota: q.quota,
        })),
      });
      navigate('/admin/teams');
    } catch (err) {
      setError('Failed to save team quota');
      console.error(err);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <div className="p-6">Loading...</div>;
  }

  if (!team) {
    return <div className="p-6">Team not found</div>;
  }

  return (
    <div className="p-6">
      <div className="mb-6">
        <h1 className="text-2xl font-bold">Team Quota: {team.display_name || team.name}</h1>
        <p className="text-gray-600">Namespace: {team.namespace}</p>
      </div>

      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
          <button onClick={() => setError(null)} className="float-right">&times;</button>
        </div>
      )}

      <div className="bg-white shadow rounded-lg overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Resource</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Type</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Quota</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Unit</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {quotas.map((quota, index) => (
              <tr key={quota.resource_config_id}>
                <td className="px-6 py-4 whitespace-nowrap font-medium">{quota.display_name}</td>
                <td className="px-6 py-4 whitespace-nowrap">
                  <span className={`px-2 py-1 rounded text-xs ${
                    quota.resource_type === 'gpu' ? 'bg-purple-100 text-purple-800' :
                    quota.resource_type === 'cpu' ? 'bg-blue-100 text-blue-800' :
                    quota.resource_type === 'memory' ? 'bg-green-100 text-green-800' :
                    'bg-gray-100 text-gray-800'
                  }`}>
                    {quota.resource_type}
                  </span>
                </td>
                <td className="px-6 py-4 whitespace-nowrap">
                  <input
                    type="number"
                    min="0"
                    value={quota.quota}
                    onChange={(e) => handleQuotaChange(index, parseInt(e.target.value) || 0)}
                    className="w-32 border rounded px-3 py-1"
                  />
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-gray-500">{quota.unit}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex justify-end space-x-2 mt-6">
        <button
          onClick={() => navigate('/admin/teams')}
          className="px-4 py-2 border rounded hover:bg-gray-50"
        >
          Cancel
        </button>
        <button
          onClick={handleSave}
          disabled={saving}
          className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
        >
          {saving ? 'Saving...' : 'Save Quota'}
        </button>
      </div>
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add web/pages/TeamQuotaEdit.tsx
git commit -m "feat: add TeamQuotaEdit page"
```

---

## Task 13: Frontend - Add Routes for Team Pages

**Files:**
- Modify: `web/App.tsx`

**Step 1: Import new pages**

```tsx
import TeamManagement from './pages/TeamManagement';
import TeamQuotaEdit from './pages/TeamQuotaEdit';
```

**Step 2: Add routes**

Add these routes inside the admin routes section:

```tsx
<Route path="/admin/teams" element={<TeamManagement />} />
<Route path="/admin/teams/:id/quota" element={<TeamQuotaEdit />} />
```

**Step 3: Commit**

```bash
git add web/App.tsx
git commit -m "feat: add routes for team management pages"
```

---

## Task 14: Frontend - Add Team Navigation to Sidebar

**Files:**
- Modify: `web/components/Sidebar.tsx`

**Step 1: Add Teams link in admin section**

Find the admin navigation section and add:

```tsx
{isAdmin && (
  <NavLink
    to="/admin/teams"
    className={({ isActive }) =>
      `flex items-center px-4 py-2 text-sm ${isActive ? 'bg-gray-200' : 'hover:bg-gray-100'}`
    }
  >
    <UsersIcon className="w-5 h-5 mr-3" />
    Teams
  </NavLink>
)}
```

**Step 2: Commit**

```bash
git add web/components/Sidebar.tsx
git commit -m "feat: add Teams navigation link to sidebar"
```

---

## Task 15: Frontend - Modify UserEdit to Show/Set Team

**Files:**
- Modify: `web/pages/UserEdit.tsx`

**Step 1: Add team state and fetch**

```tsx
const [teams, setTeams] = useState<Team[]>([]);
const [selectedTeamId, setSelectedTeamId] = useState<number | null>(null);

// In useEffect, also fetch teams
useEffect(() => {
  loadTeams();
}, []);

const loadTeams = async () => {
  try {
    const data = await teamsApi.list();
    setTeams(data);
  } catch (err) {
    console.error('Failed to load teams:', err);
  }
};
```

**Step 2: Add team selector in the form**

```tsx
<div>
  <label className="block text-sm font-medium mb-1">Team</label>
  <select
    value={selectedTeamId || ''}
    onChange={(e) => setSelectedTeamId(e.target.value ? parseInt(e.target.value) : null)}
    className="w-full border rounded px-3 py-2"
  >
    <option value="">No Team</option>
    {teams.map((team) => (
      <option key={team.id} value={team.id}>
        {team.display_name || team.name}
      </option>
    ))}
  </select>
</div>
```

**Step 3: Remove user quota editing section**

Find and remove the section that allows editing individual user quotas. Keep display-only if needed.

**Step 4: Commit**

```bash
git add web/pages/UserEdit.tsx
git commit -m "feat: add team selector to UserEdit, remove user quota editing"
```

---

## Task 16: Operator - Update to Watch All Team Namespaces

**Files:**
- Modify: `cmd/operator/main.go`

**Step 1: Change informer to watch all namespaces**

Find the informer setup and modify to watch all namespaces:

```go
// Old: watches single namespace
// factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, time.Second*60)

// New: watch all namespaces with label selector
factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
    dynClient,
    time.Second*60,
    metav1.NamespaceAll,  // Watch all namespaces
    func(options *metav1.ListOptions) {
        options.LabelSelector = "kuberde.io/team"  // Only team namespaces
    },
)
```

**Step 2: Update deployment creation to use CR's namespace**

Find where Deployment is created and ensure it uses the RDEAgent's namespace:

```go
func (c *Controller) createDeployment(agent *unstructured.Unstructured) error {
    namespace := agent.GetNamespace()  // Use agent's namespace, not hardcoded
    // ...
}
```

**Step 3: Commit**

```bash
git add cmd/operator/main.go
git commit -m "feat: operator watches all team namespaces"
```

---

## Task 17: Update RBAC for Cluster-Wide Operator

**Files:**
- Modify: `deploy/k8s/04-operator.yaml` or `charts/kuberde/templates/operator/`

**Step 1: Change Role to ClusterRole**

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
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch", "delete"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch"]
```

**Step 2: Change RoleBinding to ClusterRoleBinding**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kuberde-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kuberde-operator
subjects:
- kind: ServiceAccount
  name: kuberde-operator
  namespace: kuberde
```

**Step 3: Commit**

```bash
git add deploy/k8s/04-operator.yaml charts/kuberde/templates/operator/
git commit -m "feat: update operator RBAC to cluster-wide"
```

---

## Task 18: Final Integration Test

**Step 1: Build and deploy**

```bash
make build
make docker-build
make deploy
```

**Step 2: Test team creation**

1. Login as admin
2. Navigate to Teams
3. Create a team "ai-team"
4. Verify namespace created: `kubectl get ns kuberde-ai-team`

**Step 3: Test quota setting**

1. Click Quota for ai-team
2. Set CPU: 100, Memory: 200, Storage: 500
3. Save
4. Verify: `kubectl get resourcequota -n kuberde-ai-team`

**Step 4: Test user assignment**

1. Edit a user
2. Select ai-team
3. Save

**Step 5: Test workspace creation**

1. Login as the team user
2. Create workspace
3. Verify it's created in kuberde-ai-team namespace

**Step 6: Commit test results/fixes**

```bash
git add -A
git commit -m "test: verify multi-tenant team implementation"
```

---

## Summary

| Task | Component | Description |
|------|-----------|-------------|
| 1 | Models | Add Team, TeamQuota, modify User/Workspace |
| 2-3 | Repository | Team and TeamQuota repositories |
| 4 | Database | Auto-migration for new tables |
| 5-7 | Server API | Team CRUD, TeamQuota CRUD |
| 8-9 | Server | Workspace uses team namespace, agent ID format |
| 10 | Frontend | Types and API client |
| 11-12 | Frontend | TeamManagement and TeamQuotaEdit pages |
| 13-14 | Frontend | Routes and navigation |
| 15 | Frontend | UserEdit with team selector |
| 16-17 | Operator | Multi-namespace watch, ClusterRole |
| 18 | Integration | End-to-end testing |
