package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// SSHKey represents an SSH public key
type SSHKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"public_key"`
	Fingerprint string    `json:"fingerprint"`
	AddedAt     time.Time `json:"added_at"`
}

// User represents a user synced from Keycloak
type User struct {
	ID                   string           `gorm:"primaryKey" json:"id"`
	Username             string           `gorm:"uniqueIndex" json:"username"`
	Email                string           `json:"email"`
	FullName             string           `json:"full_name"`
	DefaultWorkspaceID   sql.NullString   `json:"default_workspace_id,omitempty"`
	SSHKeys              *json.RawMessage `gorm:"type:jsonb" json:"ssh_keys,omitempty"` // Array of SSHKey objects
	Workspaces           []Workspace      `gorm:"foreignKey:OwnerID;references:ID" json:"workspaces,omitempty"`
	Services             []Service        `gorm:"foreignKey:CreatedByID;references:ID" json:"services,omitempty"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
}

// Workspace represents a user's workspace containing services
type Workspace struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     string    `gorm:"index" json:"owner_id"`
	Owner       *User     `gorm:"foreignKey:OwnerID;references:ID" json:"owner,omitempty"`
	Services     []Service `gorm:"foreignKey:WorkspaceID;references:ID" json:"services,omitempty"`
	StorageSize  string    `gorm:"default:'50Gi'" json:"storage_size"`          // PVC storage capacity (e.g., "50Gi", "100Gi")
	StorageClass string    `gorm:"default:'standard'" json:"storage_class"`     // Kubernetes StorageClass for PVC
	PVCName      string    `json:"pvc_name"`                                    // Name of the associated PVC in Kubernetes
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AgentTemplate represents a template for creating agent services
type AgentTemplate struct {
	ID                  string           `gorm:"primaryKey" json:"id"`
	Name                string           `json:"name"`
	AgentType           string           `gorm:"uniqueIndex" json:"agent_type"` // ssh, file, coder, jupyter
	Description         string           `json:"description"`
	DockerImage         string           `json:"docker_image"`
	DefaultLocalTarget  string           `json:"default_local_target"`
	DefaultExternalPort int              `json:"default_external_port"`
	StartupArgs         string           `json:"startup_args"`                    // Startup parameters
	EnvVars             *json.RawMessage `gorm:"type:jsonb" json:"env_vars"`      // Environment variables
	SecurityContext     *json.RawMessage `gorm:"type:jsonb" json:"security_context"` // Security context (uid, gid)
	VolumeMounts        *json.RawMessage `gorm:"type:jsonb" json:"volume_mounts"` // Volume mount configuration
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at"`
}

// Service represents a tunnel/proxy service in a workspace
type Service struct {
	ID            string         `gorm:"primaryKey" json:"id"`
	WorkspaceID   string         `gorm:"index" json:"workspace_id"`
	Workspace     *Workspace     `json:"workspace,omitempty"`
	Name          string         `json:"name"`
	LocalTarget   string         `json:"local_target"` // e.g., 127.0.0.1:22
	ExternalPort  int            `json:"external_port"`
	AgentID       string         `json:"agent_id"`
	Status        string         `json:"status"` // running, stopped, error
	CreatedByID   string         `json:"created_by_id"`
	CreatedBy     *User          `json:"created_by,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	LastHeartbeat sql.NullTime   `json:"last_heartbeat,omitempty"`
	// Template-related fields
	AgentType   sql.NullString   `gorm:"index" json:"-"`  // ssh, file, coder, jupyter (copied from template at creation)
	TemplateID  sql.NullString   `gorm:"index" json:"-"` // Reference to AgentTemplate
	Template    *AgentTemplate   `gorm:"foreignKey:TemplateID;references:ID" json:"template,omitempty"`
	StartupArgs sql.NullString   `json:"-"` // User customization of startup args
	EnvVars     *json.RawMessage `gorm:"type:jsonb" json:"env_vars,omitempty"` // User customization of env vars
	// Resource configuration
	CPUCores         sql.NullString   `gorm:"column:cpu_cores" json:"cpu_cores,omitempty"`           // e.g., "4", "2"
	MemoryGiB        sql.NullString   `gorm:"column:memory_gib" json:"memory_gib,omitempty"`         // e.g., "16", "8"
	GPUCount         sql.NullInt64    `gorm:"column:gpu_count" json:"gpu_count,omitempty"`           // Number of GPUs
	GPUModel         sql.NullString   `gorm:"column:gpu_model" json:"gpu_model,omitempty"`           // e.g., "NVIDIA A100"
	GPUResourceName  sql.NullString   `gorm:"column:gpu_resource_name" json:"gpu_resource_name,omitempty"`   // e.g., "nvidia.com/gpu"
	GPUNodeSelector  *json.RawMessage `gorm:"type:jsonb;column:gpu_node_selector" json:"gpu_node_selector,omitempty"` // e.g., {"nvidia.com/model": "A100"}
	TTL              sql.NullString   `gorm:"column:ttl" json:"ttl,omitempty"` // Idle timeout duration (e.g., "24h", "8h", "30m"). "0" to disable.
	IsPinned         bool             `gorm:"default:false" json:"is_pinned"`  // Whether to show on workspace card
}

// MarshalJSON implements custom JSON serialization for Service
// to properly handle sql.NullString fields
func (s *Service) MarshalJSON() ([]byte, error) {
	type Alias Service
	return json.Marshal(&struct {
		*Alias
		AgentType       *string `json:"agent_type,omitempty"`
		TemplateID      *string `json:"template_id,omitempty"`
		StartupArgs     *string `json:"startup_args,omitempty"`
		LastHeartbeat   *string `json:"last_heartbeat,omitempty"`
		CPUCores        *string `json:"cpu_cores,omitempty"`
		MemoryGiB       *string `json:"memory_gib,omitempty"`
		GPUCount        *int64  `json:"gpu_count,omitempty"`
		GPUModel        *string `json:"gpu_model,omitempty"`
		GPUResourceName *string `json:"gpu_resource_name,omitempty"`
		TTL             *string `json:"ttl,omitempty"`
	}{
		Alias:           (*Alias)(s),
		AgentType:       nullStringToPtr(s.AgentType),
		TemplateID:      nullStringToPtr(s.TemplateID),
		StartupArgs:     nullStringToPtr(s.StartupArgs),
		LastHeartbeat:   nullTimeToPtr(s.LastHeartbeat),
		CPUCores:        nullStringToPtr(s.CPUCores),
		MemoryGiB:       nullStringToPtr(s.MemoryGiB),
		GPUCount:        nullInt64ToPtr(s.GPUCount),
		GPUModel:        nullStringToPtr(s.GPUModel),
		GPUResourceName: nullStringToPtr(s.GPUResourceName),
		TTL:             nullStringToPtr(s.TTL),
	})
}

// Helper function to convert sql.NullString to *string
func nullStringToPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

// Helper function to convert sql.NullTime to *string
func nullTimeToPtr(nt sql.NullTime) *string {
	if nt.Valid {
		str := nt.Time.Format(time.RFC3339)
		return &str
	}
	return nil
}

// Helper function to convert sql.NullInt64 to *int64
func nullInt64ToPtr(ni sql.NullInt64) *int64 {
	if ni.Valid {
		return &ni.Int64
	}
	return nil
}

// AuditLog represents an action taken by a user
type AuditLog struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	UserID     string    `gorm:"index" json:"user_id"`
	User       *User     `json:"user,omitempty"`
	Action     string    `json:"action"`      // create, update, delete
	Resource   string    `json:"resource"`    // user, workspace, service
	ResourceID string    `json:"resource_id"`
	OldData    string    `json:"old_data"`    // JSON
	NewData    string    `json:"new_data"`    // JSON
	Timestamp  time.Time `json:"timestamp"`
}

// ResourceConfig is a singleton configuration for system-wide resource defaults
type ResourceConfig struct {
	ID              int       `gorm:"primaryKey;default:1" json:"id"`
	DefaultCPUCores int       `gorm:"not null;default:8" json:"default_cpu_cores"`
	DefaultMemoryGi int       `gorm:"not null;default:16" json:"default_memory_gi"`
	StorageClasses  string    `gorm:"type:jsonb;not null;default:'[]'" json:"storage_classes"`
	GPUTypes        string    `gorm:"type:jsonb;not null;default:'[]'" json:"gpu_types"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UserQuota represents per-user resource quotas
type UserQuota struct {
	UserID       string           `gorm:"primaryKey" json:"user_id"`
	CPUCores     int              `gorm:"not null" json:"cpu_cores"`
	MemoryGi     int              `gorm:"not null" json:"memory_gi"`
	StorageQuota *json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"storage_quota"` // []UserStorageQuotaItem
	GPUQuota     *json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"gpu_quota"`     // []UserGPUQuotaItem
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

type UserStorageQuotaItem struct {
	Name    string `json:"name"`
	LimitGi int    `json:"limit_gi"`
}

type UserGPUQuotaItem struct {
	Name      string `json:"name"`
	ModelName string `json:"model_name"`
	Limit     int    `json:"limit"`
}

// TableName specifies the table name for AuditLog
func (AuditLog) TableName() string {
	return "audit_logs"
}

// TableName specifies the table name for ResourceConfig
func (ResourceConfig) TableName() string {
	return "resource_configs"
}

// TableName specifies the table name for UserQuota
func (UserQuota) TableName() string {
	return "user_quotas"
}
