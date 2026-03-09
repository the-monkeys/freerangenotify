package services

import (
	"context"
	"fmt"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type tenantService struct {
	tenantRepo  tenant.Repository
	memberRepo  tenant.MemberRepository
	authRepo    auth.Repository
	logger      *zap.Logger
}

// NewTenantService creates a new tenant service.
func NewTenantService(
	tenantRepo tenant.Repository,
	memberRepo tenant.MemberRepository,
	authRepo auth.Repository,
	logger *zap.Logger,
) tenant.Service {
	return &tenantService{
		tenantRepo: tenantRepo,
		memberRepo: memberRepo,
		authRepo:   authRepo,
		logger:     logger,
	}
}

func (s *tenantService) Create(ctx context.Context, req tenant.CreateRequest, createdBy string) (*tenant.Tenant, error) {
	slug := repository.SlugFromName(req.Name)
	existing, _ := s.tenantRepo.GetBySlug(ctx, slug)
	if existing != nil {
		return nil, errors.Conflict(fmt.Sprintf("tenant with slug %q already exists", slug))
	}

	t := &tenant.Tenant{
		ID:        "",
		Name:      req.Name,
		Slug:      slug,
		CreatedBy: createdBy,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.tenantRepo.Create(ctx, t); err != nil {
		return nil, errors.Internal("failed to create tenant", err)
	}

	// Add creator as owner
	member := &tenant.TenantMember{
		TenantID:  t.ID,
		UserID:    createdBy,
		UserEmail: "", // Will be looked up if needed
		Role:      "owner",
		InvitedBy: createdBy,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if admin, _ := s.authRepo.GetUserByID(ctx, createdBy); admin != nil {
		member.UserEmail = admin.Email
	}
	if err := s.memberRepo.Create(ctx, member); err != nil {
		s.logger.Warn("Failed to add creator as tenant owner", zap.Error(err))
	}

	return t, nil
}

func (s *tenantService) GetByID(ctx context.Context, id string) (*tenant.Tenant, error) {
	t, err := s.tenantRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errors.NotFound("Tenant", id)
	}
	return t, nil
}

func (s *tenantService) ListByUser(ctx context.Context, userID string) ([]*tenant.Tenant, error) {
	seen := make(map[string]struct{})
	var result []*tenant.Tenant

	// Tenants user created
	created, _ := s.tenantRepo.ListByCreatedBy(ctx, userID)
	for _, t := range created {
		if _, ok := seen[t.ID]; !ok {
			seen[t.ID] = struct{}{}
			result = append(result, t)
		}
	}

	// Tenants user is a member of
	members, _ := s.memberRepo.ListByUser(ctx, userID)
	for _, m := range members {
		if _, ok := seen[m.TenantID]; ok {
			continue
		}
		t, err := s.tenantRepo.GetByID(ctx, m.TenantID)
		if err != nil || t == nil {
			continue
		}
		seen[m.TenantID] = struct{}{}
		result = append(result, t)
	}

	return result, nil
}

func (s *tenantService) Update(ctx context.Context, id string, req tenant.UpdateRequest, userID string) (*tenant.Tenant, error) {
	_, role, err := s.HasAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if role != "owner" && role != "admin" {
		return nil, errors.Forbidden("only owners and admins can update the tenant")
	}

	t, err := s.tenantRepo.GetByID(ctx, id)
	if err != nil || t == nil {
		return nil, errors.NotFound("Tenant", id)
	}

	if req.Name != nil {
		t.Name = *req.Name
		t.Slug = repository.SlugFromName(*req.Name)
	}
	t.UpdatedAt = time.Now().UTC()

	if err := s.tenantRepo.Update(ctx, t); err != nil {
		return nil, errors.Internal("failed to update tenant", err)
	}
	return t, nil
}

func (s *tenantService) Delete(ctx context.Context, id string, userID string) error {
	_, role, err := s.HasAccess(ctx, id, userID)
	if err != nil {
		return err
	}
	if role != "owner" {
		return errors.Forbidden("only owners can delete the tenant")
	}

	return s.tenantRepo.Delete(ctx, id)
}

func (s *tenantService) InviteMember(ctx context.Context, tenantID string, req tenant.InviteMemberRequest, invitedBy string) (*tenant.TenantMember, error) {
	_, role, err := s.HasAccess(ctx, tenantID, invitedBy)
	if err != nil {
		return nil, err
	}
	if role != "owner" && role != "admin" {
		return nil, errors.Forbidden("only owners and admins can invite members")
	}

	// Resolve email to user ID — user must be registered in the dashboard
	admin, err := s.authRepo.GetUserByEmail(ctx, req.Email)
	if err != nil || admin == nil {
		return nil, errors.BadRequest("user with email " + req.Email + " not found — they must register first")
	}

	existing, _ := s.memberRepo.GetByTenantAndUser(ctx, tenantID, admin.UserID)
	if existing != nil {
		return nil, errors.Conflict("user is already a member")
	}

	member := &tenant.TenantMember{
		TenantID:  tenantID,
		UserID:    admin.UserID,
		UserEmail: admin.Email,
		Role:      req.Role,
		InvitedBy: invitedBy,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.memberRepo.Create(ctx, member); err != nil {
		return nil, errors.Internal("failed to add member", err)
	}
	return member, nil
}

func (s *tenantService) ListMembers(ctx context.Context, tenantID string) ([]*tenant.TenantMember, error) {
	return s.memberRepo.ListByTenant(ctx, tenantID)
}

func (s *tenantService) UpdateMemberRole(ctx context.Context, tenantID, memberID string, req tenant.UpdateMemberRoleRequest, userID string) (*tenant.TenantMember, error) {
	_, myRole, err := s.HasAccess(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if myRole != "owner" && myRole != "admin" {
		return nil, errors.Forbidden("only owners and admins can change roles")
	}

	member, err := s.memberRepo.GetByID(ctx, memberID)
	if err != nil || member == nil || member.TenantID != tenantID {
		return nil, errors.NotFound("Member", memberID)
	}

	if req.Role == "owner" && myRole != "owner" {
		return nil, errors.Forbidden("only owners can assign owner role")
	}

	member.Role = req.Role
	member.UpdatedAt = time.Now().UTC()
	if err := s.memberRepo.Update(ctx, member); err != nil {
		return nil, errors.Internal("failed to update member", err)
	}
	return member, nil
}

func (s *tenantService) RemoveMember(ctx context.Context, tenantID, memberID string, userID string) error {
	_, myRole, err := s.HasAccess(ctx, tenantID, userID)
	if err != nil {
		return err
	}

	member, err := s.memberRepo.GetByID(ctx, memberID)
	if err != nil || member == nil || member.TenantID != tenantID {
		return errors.NotFound("Member", memberID)
	}

	if member.UserID == userID {
		// Self-remove: any member can leave
	} else if myRole != "owner" && myRole != "admin" {
		return errors.Forbidden("only owners and admins can remove members")
	} else if member.Role == "owner" && myRole != "owner" {
		return errors.Forbidden("only owners can remove other owners")
	}

	return s.memberRepo.Delete(ctx, memberID)
}

func (s *tenantService) HasAccess(ctx context.Context, tenantID, userID string) (bool, string, error) {
	// Check membership
	member, _ := s.memberRepo.GetByTenantAndUser(ctx, tenantID, userID)
	if member != nil {
		return true, member.Role, nil
	}
	// Check if creator
	t, _ := s.tenantRepo.GetByID(ctx, tenantID)
	if t != nil && t.CreatedBy == userID {
		return true, "owner", nil
	}
	return false, "", nil
}
