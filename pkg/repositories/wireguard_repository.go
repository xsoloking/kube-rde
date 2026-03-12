package repositories

import (
	"kuberde/pkg/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WireguardRepository provides CRUD operations for AgentWireguardPeer records.
type WireguardRepository interface {
	Upsert(peer *models.AgentWireguardPeer) error
	GetByAgentID(agentID string) (*models.AgentWireguardPeer, error)
	Delete(agentID string) error
}

type wireguardRepository struct {
	db *gorm.DB
}

// NewWireguardRepository creates a new WireguardRepository.
func NewWireguardRepository(db *gorm.DB) WireguardRepository {
	return &wireguardRepository{db: db}
}

// Upsert creates or updates a WireGuard peer record.
func (r *wireguardRepository) Upsert(peer *models.AgentWireguardPeer) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "agent_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"public_key", "endpoints", "updated_at"}),
	}).Create(peer).Error
}

// GetByAgentID retrieves a WireGuard peer by agent ID.
func (r *wireguardRepository) GetByAgentID(agentID string) (*models.AgentWireguardPeer, error) {
	var peer models.AgentWireguardPeer
	if err := r.db.Where("agent_id = ?", agentID).First(&peer).Error; err != nil {
		return nil, err
	}
	return &peer, nil
}

// Delete removes a WireGuard peer record.
func (r *wireguardRepository) Delete(agentID string) error {
	return r.db.Where("agent_id = ?", agentID).Delete(&models.AgentWireguardPeer{}).Error
}
