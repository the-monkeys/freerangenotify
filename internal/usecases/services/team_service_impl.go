package services

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"github.com/the-monkeys/freerangenotify/internal/agentdebug"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"go.uber.org/zap"
)

type teamService struct {
	repo     auth.MembershipRepository
	authRepo auth.Repository
	logger   *zap.Logger
}

// NewTeamService creates a new auth.TeamService.
func NewTeamService(repo auth.MembershipRepository, authRepo auth.Repository, logger *zap.Logger) auth.TeamService {
	return &teamService{repo: repo, authRepo: authRepo, logger: logger}
}

func (s *teamService) InviteMember(ctx context.Context, appID string, req *auth.InviteMemberRequest, inviterID, appName string) (*auth.AppMembership, error) {
	if _, ok := auth.RolePermissions[req.Role]; !ok {
		return nil, fmt.Errorf("invalid role: %s", req.Role)
	}

	// Resolve email to actual user ID if the user already has an account
	memberUserID := req.Email // fallback for users who haven't registered yet
	userExists := false
	if s.authRepo != nil {
		if existingUser, err := s.authRepo.GetUserByEmail(ctx, req.Email); err == nil && existingUser != nil {
			memberUserID = existingUser.UserID
			userExists = true
		}
	}

	// Prevent duplicate membership
	existing, err := s.repo.GetByAppAndUser(ctx, appID, memberUserID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("user %s is already a member of this application", req.Email)
	}

	membership := &auth.AppMembership{
		AppID:     appID,
		UserID:    memberUserID,
		UserEmail: req.Email,
		Role:      req.Role,
		InvitedBy: inviterID,
	}

	if err := s.repo.Create(ctx, membership); err != nil {
		return nil, fmt.Errorf("failed to create membership: %w", err)
	}

	s.logger.Info("Member invited",
		zap.String("app_id", appID),
		zap.String("email", req.Email),
		zap.String("role", string(req.Role)))

	agentdebug.Log(
		"pre-fix-rbac",
		"H1-membership-invite",
		"internal/usecases/services/team_service_impl.go:InviteMember",
		"created membership invite",
		map[string]any{
			"app_id":       appID,
			"inviter_id":   inviterID,
			"member_user":  memberUserID,
			"member_email": req.Email,
			"role":         req.Role,
		},
	)

	// Send invitation email (best-effort — don't fail the invite if email fails)
	if err := s.sendInvitationEmail(req.Email, appName, req.Role, userExists); err != nil {
		s.logger.Warn("Failed to send invitation email",
			zap.String("email", req.Email),
			zap.String("app_name", appName),
			zap.Error(err))
	}

	return membership, nil
}

func (s *teamService) UpdateRole(ctx context.Context, appID, membershipID string, req *auth.UpdateMemberRoleRequest) (*auth.AppMembership, error) {
	if _, ok := auth.RolePermissions[req.Role]; !ok {
		return nil, fmt.Errorf("invalid role: %s", req.Role)
	}

	membership, err := s.repo.GetByID(ctx, membershipID)
	if err != nil {
		return nil, fmt.Errorf("membership not found: %w", err)
	}

	if membership.AppID != appID {
		return nil, fmt.Errorf("membership does not belong to this application")
	}

	// Prevent demoting the last owner
	if membership.Role == auth.RoleOwner && req.Role != auth.RoleOwner {
		members, listErr := s.repo.ListByApp(ctx, appID)
		if listErr == nil {
			ownerCount := 0
			for _, m := range members {
				if m.Role == auth.RoleOwner {
					ownerCount++
				}
			}
			if ownerCount <= 1 {
				return nil, fmt.Errorf("cannot demote the last owner of the application")
			}
		}
	}

	membership.Role = req.Role
	if err := s.repo.Update(ctx, membership); err != nil {
		return nil, fmt.Errorf("failed to update membership: %w", err)
	}

	return membership, nil
}

func (s *teamService) RemoveMember(ctx context.Context, appID, membershipID string) error {
	membership, err := s.repo.GetByID(ctx, membershipID)
	if err != nil {
		return fmt.Errorf("membership not found: %w", err)
	}

	if membership.AppID != appID {
		return fmt.Errorf("membership does not belong to this application")
	}

	// Prevent removing the last owner
	if membership.Role == auth.RoleOwner {
		members, listErr := s.repo.ListByApp(ctx, appID)
		if listErr == nil {
			ownerCount := 0
			for _, m := range members {
				if m.Role == auth.RoleOwner {
					ownerCount++
				}
			}
			if ownerCount <= 1 {
				return fmt.Errorf("cannot remove the last owner of the application")
			}
		}
	}

	return s.repo.Delete(ctx, membershipID)
}

func (s *teamService) ListMembers(ctx context.Context, appID string) ([]*auth.AppMembership, error) {
	return s.repo.ListByApp(ctx, appID)
}

func (s *teamService) GetMembership(ctx context.Context, appID, userID string) (*auth.AppMembership, error) {
	return s.repo.GetByAppAndUser(ctx, appID, userID)
}

// sendInvitationEmail sends a team invitation email. The email content varies
// based on whether the invitee already has an account (log in) or needs to
// register first (create account).
func (s *teamService) sendInvitationEmail(email, appName string, role auth.Role, userExists bool) error {
	smtpHost := os.Getenv("FREERANGE_PROVIDERS_SMTP_HOST")
	smtpPortStr := os.Getenv("FREERANGE_PROVIDERS_SMTP_PORT")
	smtpUsername := os.Getenv("FREERANGE_PROVIDERS_SMTP_USERNAME")
	smtpPassword := os.Getenv("FREERANGE_PROVIDERS_SMTP_PASSWORD")
	smtpFromEmail := os.Getenv("FREERANGE_PROVIDERS_SMTP_FROM_EMAIL")
	smtpFromName := os.Getenv("FREERANGE_PROVIDERS_SMTP_FROM_NAME")

	if smtpHost == "" || smtpUsername == "" || smtpPassword == "" || smtpFromEmail == "" {
		return fmt.Errorf("SMTP not configured — invitation email not sent")
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

	roleDisplay := string(role)
	if len(roleDisplay) > 0 {
		roleDisplay = string(roleDisplay[0]-32) + roleDisplay[1:]
	}

	var ctaText, ctaLink, description string
	if userExists {
		ctaText = "Log In to Accept"
		ctaLink = frontendURL + "/login"
		description = "You already have a FreeRangeNotify account. Log in to access the application."
	} else {
		ctaText = "Create Your Account"
		ctaLink = frontendURL + "/register"
		description = "Create a FreeRangeNotify account with this email address to accept the invitation."
	}

	subject := fmt.Sprintf("You've been invited to %s — FreeRangeNotify", appName)

	body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>Team Invitation</title></head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="background-color: #f8f9fa; border-radius: 10px; padding: 30px; margin-bottom: 20px;">
        <h1 style="color: #2563eb; margin-top: 0;">You're Invited!</h1>
        <p>You've been invited to join <strong>%s</strong> as <strong style="color: #2563eb;">%s</strong>.</p>
        <p>%s</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="%s"
               style="background-color: #2563eb; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block; font-weight: bold;">
                %s
            </a>
        </div>
        <div style="background-color: #e9ecef; border-radius: 8px; padding: 16px; margin: 20px 0;">
            <p style="margin: 0 0 8px 0; font-weight: bold; color: #495057;">Your role: %s</p>
            <p style="margin: 0; color: #6c757d; font-size: 14px;">%s</p>
        </div>
        <hr style="border: none; border-top: 1px solid #dee2e6; margin: 30px 0;">
        <p style="color: #6c757d; font-size: 14px;">
            If you weren't expecting this invitation, you can safely ignore this email.
        </p>
    </div>
    <div style="text-align: center; color: #6c757d; font-size: 12px;">
        <p>&copy; 2026 FreeRangeNotify. All rights reserved.</p>
    </div>
</body>
</html>`, appName, roleDisplay, description, ctaLink, ctaText, roleDisplay, roleDescription(role))

	msg := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		smtpFromName, smtpFromEmail, email, subject, body)

	smtpAuth := smtp.PlainAuth("", smtpUsername, smtpPassword, smtpHost)
	addr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)

	if err := smtp.SendMail(addr, smtpAuth, smtpFromEmail, []string{email}, []byte(msg)); err != nil {
		return fmt.Errorf("failed to send invitation email: %w", err)
	}

	s.logger.Info("Invitation email sent",
		zap.String("email", email),
		zap.String("app_name", appName),
		zap.String("role", string(role)),
		zap.Bool("user_exists", userExists))

	return nil
}

func roleDescription(role auth.Role) string {
	switch role {
	case auth.RoleOwner:
		return "Full control — manage the application, team members, templates, and notifications."
	case auth.RoleAdmin:
		return "Manage team members, templates, send notifications, and view logs and audit trails."
	case auth.RoleEditor:
		return "Create and edit templates, send notifications, and view logs."
	case auth.RoleViewer:
		return "View notification logs and application activity."
	default:
		return ""
	}
}
