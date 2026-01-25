package repositories

import (
	"kuberde/pkg/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TeamQuotaRepository interface {
	Create(quota *models.TeamQuota) error
	GetByID(id uint) (*models.TeamQuota, error)
	GetByTeamID(teamID uint) (*models.TeamQuota, error)
	Update(quota *models.TeamQuota) error
	Delete(id uint) error
	DeleteByTeamID(teamID uint) error
	Upsert(quota *models.TeamQuota) error
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
	if err := r.db.First(&quota, id).Error; err != nil {
		return nil, err
	}
	return &quota, nil
}

func (r *teamQuotaRepository) GetByTeamID(teamID uint) (*models.TeamQuota, error) {
	var quota models.TeamQuota
	if err := r.db.Where("team_id = ?", teamID).First(&quota).Error; err != nil {
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

func (r *teamQuotaRepository) Upsert(quota *models.TeamQuota) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "team_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"cpu_cores", "memory_gi", "storage_quota", "gpu_quota", "updated_at"}),
	}).Create(quota).Error
}
