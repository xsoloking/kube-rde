package repositories

import (
	"kuberde/pkg/models"
	"gorm.io/gorm"
)

// UserQuotaRepository provides methods for accessing user quotas
type UserQuotaRepository interface {
	GetByUserID(userID string) (*models.UserQuota, error)
	Create(quota *models.UserQuota) error
	Update(quota *models.UserQuota) error
	Delete(userID string) error
	List() ([]models.UserQuota, error)
}

type userQuotaRepository struct {
	db *gorm.DB
}

// NewUserQuotaRepository creates a new UserQuotaRepository instance
func NewUserQuotaRepository(db *gorm.DB) UserQuotaRepository {
	return &userQuotaRepository{db: db}
}

// GetByUserID retrieves a user's quota by user ID
func (r *userQuotaRepository) GetByUserID(userID string) (*models.UserQuota, error) {
	var quota models.UserQuota
	err := r.db.Where("user_id = ?", userID).First(&quota).Error
	return &quota, err
}

// Create creates a new user quota
func (r *userQuotaRepository) Create(quota *models.UserQuota) error {
	return r.db.Create(quota).Error
}

// Update updates an existing user quota
func (r *userQuotaRepository) Update(quota *models.UserQuota) error {
	return r.db.Save(quota).Error
}

// Delete deletes a user's quota
func (r *userQuotaRepository) Delete(userID string) error {
	return r.db.Delete(&models.UserQuota{}, "user_id = ?", userID).Error
}

// List retrieves all user quotas
func (r *userQuotaRepository) List() ([]models.UserQuota, error) {
	var quotas []models.UserQuota
	err := r.db.Find(&quotas).Error
	return quotas, err
}
