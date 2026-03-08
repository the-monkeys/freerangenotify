package services

import (
	"context"
	"fmt"

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

func (s *teamService) InviteMember(ctx context.Context, appID string, req *auth.InviteMemberRequest, inviterID string) (*auth.AppMembership, error) {
	// Validate role
	if _, ok := auth.RolePermissions[req.Role]; !ok {
		return nil, fmt.Errorf("invalid role: %s", req.Role)
	}

	// Resolve email to actual user ID if the user already has an account
	memberUserID := req.Email // fallback for users who haven't registered yet
	if s.authRepo != nil {
		if existingUser, err := s.authRepo.GetUserByEmail(ctx, req.Email); err == nil && existingUser != nil {
			memberUserID = existingUser.UserID
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
