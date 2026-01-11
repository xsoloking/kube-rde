package db

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"kuberde/pkg/models"
)

// UserRepository handles User operations
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByID fetches user by ID
func (r *UserRepository) FindByID(id string) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByUsername fetches user by username
func (r *UserRepository) FindByUsername(username string) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, "username = ?", username).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// Create creates a new user
func (r *UserRepository) Create(user *models.User) error {
	if user.ID == "" {
		return fmt.Errorf("user ID is required")
	}
	return r.db.Create(user).Error
}

// Update updates an existing user
func (r *UserRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

// Delete deletes a user
func (r *UserRepository) Delete(id string) error {
	return r.db.Delete(&models.User{}, "id = ?", id).Error
}

// GetAll fetches all users with pagination
func (r *UserRepository) GetAll(limit, offset int) ([]models.User, error) {
	var users []models.User
	if err := r.db.Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// Count counts all users
func (r *UserRepository) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&models.User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// WorkspaceRepository handles Workspace operations
type WorkspaceRepository struct {
	db *gorm.DB
}
// NewWorkspaceRepository creates a new workspace repository
func NewWorkspaceRepository(db *gorm.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

// FindByID fetches workspace by ID
func (r *WorkspaceRepository) FindByID(id string) (*models.Workspace, error) {
	var workspace models.Workspace
	if err := r.db.First(&workspace, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &workspace, nil
}

// FindByOwnerID fetches workspaces owned by a user
func (r *WorkspaceRepository) FindByOwnerID(ownerID string, limit, offset int) ([]models.Workspace, error) {
	var workspaces []models.Workspace
	if err := r.db.
		Where("owner_id = ?", ownerID).
		Preload("Services").
		Limit(limit).
		Offset(offset).
		Find(&workspaces).Error; err != nil {
		return nil, err
	}
	return workspaces, nil
}

// GetAll fetches all workspaces (admin only)
func (r *WorkspaceRepository) GetAll(limit, offset int) ([]models.Workspace, error) {
	var workspaces []models.Workspace
	if err := r.db.
		Preload("Owner").
		Preload("Services").
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&workspaces).Error; err != nil {
		return nil, err
	}
	return workspaces, nil
}

// Count counts all workspaces
func (r *WorkspaceRepository) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&models.Workspace{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
// Create creates a new workspace
func (r *WorkspaceRepository) Create(workspace *models.Workspace) error {
	if workspace.ID == "" {
		workspace.ID = uuid.New().String()
	}
	if workspace.OwnerID == "" {
		return fmt.Errorf("workspace owner_id is required")
	}
	return r.db.Create(workspace).Error
}

// Update updates an existing workspace
func (r *WorkspaceRepository) Update(workspace *models.Workspace) error {
	return r.db.Save(workspace).Error
}

// Delete deletes a workspace (cascades to services)
func (r *WorkspaceRepository) Delete(id string) error {
	return r.db.Delete(&models.Workspace{}, "id = ?", id).Error
}

// ServiceRepository handles Service operations
type ServiceRepository struct {
	db *gorm.DB
}

// NewServiceRepository creates a new service repository
func NewServiceRepository(db *gorm.DB) *ServiceRepository {
	return &ServiceRepository{db: db}
}

// FindByID fetches service by ID
func (r *ServiceRepository) FindByID(id string) (*models.Service, error) {
	var service models.Service
	if err := r.db.First(&service, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &service, nil
}

// FindByWorkspaceID fetches services in a workspace
func (r *ServiceRepository) FindByWorkspaceID(workspaceID string, limit, offset int) ([]models.Service, error) {
	var services []models.Service
	if err := r.db.
		Where("workspace_id = ?", workspaceID).
		Limit(limit).
		Offset(offset).
		Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

// Create creates a new service
func (r *ServiceRepository) Create(service *models.Service) error {
	if service.ID == "" {
		service.ID = uuid.New().String()
	}
	if service.WorkspaceID == "" {
		return fmt.Errorf("service workspace_id is required")
	}
	return r.db.Create(service).Error
}

// Update updates an existing service
func (r *ServiceRepository) Update(service *models.Service) error {
	return r.db.Save(service).Error
}

// UpdateStatus updates service status and heartbeat
func (r *ServiceRepository) UpdateStatus(id string, status string) error {
	now := time.Now()
	return r.db.Model(&models.Service{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":           status,
			"last_heartbeat":   now,
			"updated_at":       now,
		}).Error
}

// Delete deletes a service
func (r *ServiceRepository) Delete(id string) error {
	return r.db.Delete(&models.Service{}, "id = ?", id).Error
}

// Count counts all services
func (r *ServiceRepository) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&models.Service{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountActive counts running services
func (r *ServiceRepository) CountActive() (int64, error) {
	var count int64
	if err := r.db.Model(&models.Service{}).Where("status = ?", "running").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetAll fetches all services (for admin stats calculation)
func (r *ServiceRepository) GetAll() ([]models.Service, error) {
	var services []models.Service
	if err := r.db.Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

// AuditLogRepository handles AuditLog operations
type AuditLogRepository struct {
	db *gorm.DB
}

// NewAuditLogRepository creates a new audit log repository
func NewAuditLogRepository(db *gorm.DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

// LogAction creates an audit log entry
func (r *AuditLogRepository) LogAction(userID string, action string, resource string, resourceID string, oldData string, newData string) error {
	log := &models.AuditLog{
		ID:         uuid.New().String(),
		UserID:     userID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		OldData:    oldData,
		NewData:    newData,
		Timestamp:  time.Now(),
	}
	return r.db.Create(log).Error
}

// FindByUserID fetches audit logs for a user
func (r *AuditLogRepository) FindByUserID(userID string, limit, offset int) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	if err := r.db.
		Where("user_id = ?", userID).
		Order("timestamp DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// FindByResource fetches audit logs for a resource
func (r *AuditLogRepository) FindByResource(resource string, resourceID string, limit, offset int) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	if err := r.db.
		Where("resource = ? AND resource_id = ?", resource, resourceID).
		Order("timestamp DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// AuditLogFilter defines criteria for searching audit logs
type AuditLogFilter struct {
	UserID    string
	Action    string
	Resource  string
	StartDate *time.Time
	EndDate   *time.Time
}

// Search searches audit logs based on filter
func (r *AuditLogRepository) Search(filter AuditLogFilter, limit, offset int) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64
	query := r.db.Model(&models.AuditLog{})

	if filter.UserID != "" {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.Resource != "" {
		query = query.Where("resource = ?", filter.Resource)
	}
	if filter.StartDate != nil {
		query = query.Where("timestamp >= ?", filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("timestamp <= ?", filter.EndDate)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Preload("User").Order("timestamp DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
