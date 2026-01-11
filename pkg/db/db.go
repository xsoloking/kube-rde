package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"kuberde/pkg/models"
)

var DB *gorm.DB

// InitDB initializes database connection and runs migrations
func InitDB(dsn string) error {
	var err error

	// Connect to PostgreSQL
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("✓ Connected to PostgreSQL")

	// Run migrations using Goose
	if err := runMigrations(dsn); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("✓ Database migrations completed")

	return nil
}

// runMigrations executes all pending migrations
func runMigrations(dsn string) error {
	// Get the migration directory path
	migrationDir := filepath.Join("deploy", "migrations")

	// Check if migrations directory exists
	if _, err := os.Stat(migrationDir); os.IsNotExist(err) {
		log.Printf("warning: migrations directory not found at %s, skipping migrations", migrationDir)
		return nil
	}

	// Get raw database connection for Goose
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	// Set Goose dialect to PostgreSQL
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Run pending migrations
	if err := goose.Up(sqlDB, migrationDir); err != nil {
		return fmt.Errorf("failed to execute migrations: %w", err)
	}

	return nil
}

// AutoMigrate runs GORM auto migration (alternative to SQL migrations)
// This is useful as a fallback if Goose migrations aren't available
func AutoMigrate() error {
	return DB.AutoMigrate(
		&models.User{},
		&models.Workspace{},
		&models.Service{},
		&models.AuditLog{},
		&models.AgentTemplate{},
	)
}

// Close closes the database connection
func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Repository accessors
func UserRepo() *UserRepository {
	return NewUserRepository(DB)
}

func WorkspaceRepo() *WorkspaceRepository {
	return NewWorkspaceRepository(DB)
}

func ServiceRepo() *ServiceRepository {
	return NewServiceRepository(DB)
}

func AuditLogRepo() *AuditLogRepository {
	return NewAuditLogRepository(DB)
}

func AgentTemplateRepo() *AgentTemplateRepository {
	return NewAgentTemplateRepository(DB)
}
