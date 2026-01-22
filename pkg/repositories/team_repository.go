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
