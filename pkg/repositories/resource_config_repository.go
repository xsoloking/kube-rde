package repositories

import (
	"gorm.io/gorm"
	"kuberde/pkg/models"
)

// ResourceConfigRepository provides methods for accessing resource configuration
type ResourceConfigRepository interface {
	GetConfig() (*models.ResourceConfig, error)
	UpdateConfig(config *models.ResourceConfig) error
}

type resourceConfigRepository struct {
	db *gorm.DB
}

// NewResourceConfigRepository creates a new ResourceConfigRepository instance
func NewResourceConfigRepository(db *gorm.DB) ResourceConfigRepository {
	return &resourceConfigRepository{db: db}
}

// GetConfig retrieves the singleton resource configuration
func (r *resourceConfigRepository) GetConfig() (*models.ResourceConfig, error) {
	var config models.ResourceConfig
	err := r.db.First(&config, 1).Error
	return &config, err
}

// UpdateConfig updates the singleton resource configuration
func (r *resourceConfigRepository) UpdateConfig(config *models.ResourceConfig) error {
	config.ID = 1 // Ensure singleton constraint
	return r.db.Save(config).Error
}
