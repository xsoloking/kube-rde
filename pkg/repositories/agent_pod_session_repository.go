package repositories

import (
	"time"

	"kuberde/pkg/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentPodSessionRepository provides CRUD operations for AgentPodSession records.
// These records map each agent ID to the server pod IP that holds its Yamux session,
// enabling request forwarding in multi-replica (HA) deployments.
type AgentPodSessionRepository interface {
	Upsert(s *models.AgentPodSession) error
	Touch(agentID string) error
	GetByAgentID(agentID string) (*models.AgentPodSession, error)
	Delete(agentID string) error
}

type agentPodSessionRepository struct {
	db *gorm.DB
}

// NewAgentPodSessionRepository creates a new AgentPodSessionRepository.
func NewAgentPodSessionRepository(db *gorm.DB) AgentPodSessionRepository {
	return &agentPodSessionRepository{db: db}
}

// Upsert creates or updates an AgentPodSession record.
func (r *agentPodSessionRepository) Upsert(s *models.AgentPodSession) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "agent_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"pod_ip", "pod_port", "updated_at"}),
	}).Create(s).Error
}

// Touch refreshes the UpdatedAt timestamp so other pods know the session is alive.
func (r *agentPodSessionRepository) Touch(agentID string) error {
	return r.db.Model(&models.AgentPodSession{}).
		Where("agent_id = ?", agentID).
		Update("updated_at", time.Now()).Error
}

// GetByAgentID retrieves the pod session record for an agent.
func (r *agentPodSessionRepository) GetByAgentID(agentID string) (*models.AgentPodSession, error) {
	var s models.AgentPodSession
	if err := r.db.Where("agent_id = ?", agentID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// Delete removes an agent's pod session record (called on agent disconnect).
func (r *agentPodSessionRepository) Delete(agentID string) error {
	return r.db.Where("agent_id = ?", agentID).Delete(&models.AgentPodSession{}).Error
}
