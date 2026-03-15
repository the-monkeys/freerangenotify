package services

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/dashboard_notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/platform"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type tenantService struct {
	tenantRepo  tenant.Repository
	memberRepo  tenant.MemberRepository
	authRepo    auth.Repository
	licenseRepo license.Repository
	notifier    dashboard_notification.Notifier
	logger      *zap.Logger
}

// NewTenantService creates a new tenant service.
func NewTenantService(
	tenantRepo tenant.Repository,
	memberRepo tenant.MemberRepository,
	authRepo auth.Repository,
	licenseRepo license.Repository,
	notifier dashboard_notification.Notifier,
	logger *zap.Logger,
) tenant.Service {
	return &tenantService{
		tenantRepo:  tenantRepo,
		memberRepo:  memberRepo,
		authRepo:    authRepo,
		licenseRepo: licenseRepo,
		notifier:    notifier,
		logger:      logger,
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

	// Send invitation email (best-effort)
	inviterName := ""
	if inv, _ := s.authRepo.GetUserByID(ctx, invitedBy); inv != nil {
		inviterName = inv.FullName
	}
	if err := s.sendOrgInvitationEmail(ctx, tenantID, req.Email, req.Role, inviterName); err != nil {
		s.logger.Warn("Failed to send org invitation email", zap.String("email", req.Email), zap.Error(err))
	}

	// Create in-app notification and publish via SSE (best-effort)
	if s.notifier != nil {
		t, _ := s.tenantRepo.GetByID(ctx, tenantID)
		orgName := tenantID
		if t != nil {
			orgName = t.Name
		}
		if inviterName == "" {
			inviterName = "Someone"
		}
		title, body := platform.RenderOrgInviteInApp(orgName, inviterName, platform.FormatRoleDisplay(req.Role))
		if nErr := s.notifier.NotifyUser(ctx, admin.UserID, title, body, "org_invite", map[string]interface{}{
			"tenant_id": tenantID, "role": req.Role, "invited_by": invitedBy,
		}); nErr != nil {
			s.logger.Warn("Failed to create dashboard notification for org invite", zap.Error(nErr))
		}
	}

	return member, nil
}

// sendOrgInvitationEmail sends an organization invitation email using platform templates.
func (s *tenantService) sendOrgInvitationEmail(ctx context.Context, tenantID, email, role string, inviterName string) error {
	smtpHost := os.Getenv("FREERANGE_PROVIDERS_SMTP_HOST")
	smtpPortStr := os.Getenv("FREERANGE_PROVIDERS_SMTP_PORT")
	smtpUsername := os.Getenv("FREERANGE_PROVIDERS_SMTP_USERNAME")
	smtpPassword := os.Getenv("FREERANGE_PROVIDERS_SMTP_PASSWORD")
	smtpFromEmail := os.Getenv("FREERANGE_PROVIDERS_SMTP_FROM_EMAIL")
	smtpFromName := os.Getenv("FREERANGE_PROVIDERS_SMTP_FROM_NAME")

	if smtpHost == "" || smtpUsername == "" || smtpPassword == "" || smtpFromEmail == "" {
		return fmt.Errorf("SMTP not configured — org invitation email not sent")
	}

	smtpPort := 587
	if smtpPortStr != "" {
		if port, err := strconv.Atoi(smtpPortStr); err == nil {
			smtpPort = port
		}
	}
	if smtpFromName == "" {
		smtpFromName = "FreeRangeNotify"
	}

	frontendURL := os.Getenv("FREERANGE_FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	t, _ := s.tenantRepo.GetByID(ctx, tenantID)
	orgName := tenantID
	if t != nil {
		orgName = t.Name
	}
	if inviterName == "" {
		inviterName = "Someone"
	}

	// User always exists (we require registration for org invites)
	data := platform.OrgInviteEmailDataWithURL(orgName, inviterName, role, email, frontendURL, true)
	subject, body, err := platform.RenderOrgInviteEmail(data)
	if err != nil {
		return fmt.Errorf("render org invite email: %w", err)
	}

	msg := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		smtpFromName, smtpFromEmail, email, subject, body)

	auth := smtp.PlainAuth("", smtpUsername, smtpPassword, smtpHost)
	addr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)
	if err := smtp.SendMail(addr, auth, smtpFromEmail, []string{email}, []byte(msg)); err != nil {
		return fmt.Errorf("failed to send org invitation email: %w", err)
	}
	return nil
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

// UpgradeBilling updates the billing fields on the tenant.
func (s *tenantService) UpgradeBilling(ctx context.Context, id string, tier string, validUntil time.Time, maxApps int, maxThroughput int) error {
	t, err := s.tenantRepo.GetByID(ctx, id)
	if err != nil || t == nil {
		return errors.NotFound("Tenant", id)
	}

	t.BillingTier = tier
	t.ValidUntil = validUntil
	t.MaxApps = maxApps
	t.MaxThroughput = maxThroughput
	t.UpdatedAt = time.Now().UTC()

	if err := s.tenantRepo.Update(ctx, t); err != nil {
		return errors.Internal("failed to update tenant billing", err)
	}

	// Also update or create the License Subscription so the Hosted Checker instantly unblocks API traffic
	if s.licenseRepo != nil {
		// Mock logic directly creates an active subscription
		sub := &license.Subscription{
			ID:                 "sub_" + id, // simplified mock ID
			TenantID:           id,
			AppID:              "",
			Plan:               tier,
			Status:             license.SubscriptionStatusActive,
			CurrentPeriodStart: time.Now().UTC(),
			CurrentPeriodEnd:   validUntil,
			CreatedAt:          time.Now().UTC(),
			UpdatedAt:          time.Now().UTC(),
		}

		// UPSERT approach: Try to create, if it fails because it exists, ignore or update (mock simplifies this).
		_ = s.licenseRepo.Create(ctx, sub)
	}

	return nil
}
