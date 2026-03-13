package tenant

import (
	"context"
	"time"
)

// Tenant represents an organization at the platform level.
// Applications can belong to a tenant; tenant members get access to all apps in the tenant.
type Tenant struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	CreatedBy     string    `json:"created_by"`
	BillingTier   string    `json:"billing_tier"`
	LicenseKey    string    `json:"license_key"`
	ValidUntil    time.Time `json:"valid_until"`
	MaxApps       int       `json:"max_apps"`
	MaxThroughput int       `json:"max_throughput"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TenantMember links a user to a tenant with a role.
type TenantMember struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	Role      string    `json:"role"` // owner, admin, member
	InvitedBy string    `json:"invited_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateRequest is the input for creating a tenant.
type CreateRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

// UpdateRequest is the input for updating a tenant.
type UpdateRequest struct {
	Name *string `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
}

// InviteMemberRequest is the input for inviting a member to a tenant.
type InviteMemberRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"required,oneof=admin member"`
}

// UpdateMemberRoleRequest is the input for updating a tenant member's role.
type UpdateMemberRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=owner admin member"`
}

// Repository defines persistence operations for tenants.
type Repository interface {
	Create(ctx context.Context, t *Tenant) error
	GetByID(ctx context.Context, id string) (*Tenant, error)
	GetBySlug(ctx context.Context, slug string) (*Tenant, error)
	ListByCreatedBy(ctx context.Context, userID string) ([]*Tenant, error)
	Update(ctx context.Context, t *Tenant) error
	Delete(ctx context.Context, id string) error
}

// MemberRepository defines persistence operations for tenant members.
type MemberRepository interface {
	Create(ctx context.Context, m *TenantMember) error
	GetByID(ctx context.Context, id string) (*TenantMember, error)
	GetByTenantAndUser(ctx context.Context, tenantID, userID string) (*TenantMember, error)
	ListByTenant(ctx context.Context, tenantID string) ([]*TenantMember, error)
	ListByUser(ctx context.Context, userID string) ([]*TenantMember, error)
	Update(ctx context.Context, m *TenantMember) error
	Delete(ctx context.Context, id string) error
}

// Service defines the business logic interface for tenants.
type Service interface {
	Create(ctx context.Context, req CreateRequest, createdBy string) (*Tenant, error)
	GetByID(ctx context.Context, id string) (*Tenant, error)
	ListByUser(ctx context.Context, userID string) ([]*Tenant, error)
	Update(ctx context.Context, id string, req UpdateRequest, userID string) (*Tenant, error)
	Delete(ctx context.Context, id string, userID string) error
	InviteMember(ctx context.Context, tenantID string, req InviteMemberRequest, invitedBy string) (*TenantMember, error)
	ListMembers(ctx context.Context, tenantID string) ([]*TenantMember, error)
	UpdateMemberRole(ctx context.Context, tenantID, memberID string, req UpdateMemberRoleRequest, userID string) (*TenantMember, error)
	RemoveMember(ctx context.Context, tenantID, memberID string, userID string) error
	HasAccess(ctx context.Context, tenantID, userID string) (bool, string, error) // hasAccess, role, error
	UpgradeBilling(ctx context.Context, id string, tier string, validUntil time.Time, maxApps int, maxThroughput int) error
}
