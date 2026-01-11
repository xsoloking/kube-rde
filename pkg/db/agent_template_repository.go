package db

import (
	"context"
	"gorm.io/gorm"
	"kuberde/pkg/models"
)

// AgentTemplateRepository handles database operations for agent templates
type AgentTemplateRepository struct {
	db *gorm.DB
}

// NewAgentTemplateRepository creates a new agent template repository
func NewAgentTemplateRepository(db *gorm.DB) *AgentTemplateRepository {
	return &AgentTemplateRepository{db: db}
}

// GetByID retrieves an agent template by ID
func (r *AgentTemplateRepository) GetByID(ctx context.Context, id string) (*models.AgentTemplate, error) {
	var template models.AgentTemplate
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&template).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

// GetByAgentType retrieves an agent template by agent type
func (r *AgentTemplateRepository) GetByAgentType(ctx context.Context, agentType string) (*models.AgentTemplate, error) {
	var template models.AgentTemplate
	if err := r.db.WithContext(ctx).Where("agent_type = ?", agentType).First(&template).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

// GetAll retrieves all agent templates (built-in and user-defined)
func (r *AgentTemplateRepository) GetAll(ctx context.Context) ([]models.AgentTemplate, error) {
	var templates []models.AgentTemplate
	if err := r.db.WithContext(ctx).Order("created_at asc").Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// Create creates a new agent template
func (r *AgentTemplateRepository) Create(ctx context.Context, template *models.AgentTemplate) error {
	return r.db.WithContext(ctx).Create(template).Error
}

// Update updates an existing agent template
func (r *AgentTemplateRepository) Update(ctx context.Context, template *models.AgentTemplate) error {
	return r.db.WithContext(ctx).Save(template).Error
}

// Delete deletes an agent template (only user-defined templates, not built-in)
func (r *AgentTemplateRepository) Delete(ctx context.Context, id string) error {
	// Built-in templates (tpl-ssh-001, tpl-file-001, tpl-coder-001, tpl-jupyter-001) cannot be deleted
	builtInIDs := map[string]bool{
		"tpl-ssh-001":     true,
		"tpl-file-001":    true,
		"tpl-coder-001":   true,
		"tpl-jupyter-001": true,
	}

	if builtInIDs[id] {
		return gorm.ErrInvalidData // Or return a custom error
	}

	return r.db.WithContext(ctx).Delete(&models.AgentTemplate{}, "id = ?", id).Error
}
