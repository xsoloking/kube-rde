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
