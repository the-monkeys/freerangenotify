package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/digest"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// ImportHandler handles cross-app resource linking.
type ImportHandler struct {
	linkRepo       resourcelink.Repository
	appService     usecases.ApplicationService
	membershipRepo auth.MembershipRepository
	appRepo        application.Repository
	userRepo       user.Repository
	logger         *zap.Logger

	// Feature-gated — nil when disabled
	templateService *usecases.TemplateService
	workflowService workflow.Service
	digestService   digest.Service
	topicService    topic.Service
}

func NewImportHandler(
	linkRepo resourcelink.Repository,
	appService usecases.ApplicationService,
	membershipRepo auth.MembershipRepository,
	appRepo application.Repository,
	userRepo user.Repository,
	logger *zap.Logger,
) *ImportHandler {
	return &ImportHandler{
		linkRepo:       linkRepo,
		appService:     appService,
		membershipRepo: membershipRepo,
		appRepo:        appRepo,
		userRepo:       userRepo,
		logger:         logger,
	}
}

func (h *ImportHandler) SetTemplateService(ts *usecases.TemplateService)  { h.templateService = ts }
func (h *ImportHandler) SetWorkflowService(ws workflow.Service)           { h.workflowService = ws }
func (h *ImportHandler) SetDigestService(ds digest.Service)               { h.digestService = ds }
func (h *ImportHandler) SetTopicService(ts topic.Service)                 { h.topicService = ts }

// Import handles POST /v1/apps/:id/import
func (h *ImportHandler) Import(c *fiber.Ctx) error {
	targetAppID := c.Params("id")
	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		return errors.Unauthorized("authentication required")
	}

	var req resourcelink.ImportRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("invalid request body")
	}
	if req.SourceAppID == "" {
		return errors.BadRequest("source_app_id is required")
	}
	if len(req.ResourceTypes) == 0 {
		return errors.BadRequest("at least one resource type is required")
	}
	if req.SourceAppID == targetAppID {
		return errors.BadRequest("source and target app cannot be the same")
	}
	for _, rt := range req.ResourceTypes {
		if !resourcelink.ValidTypes[rt] {
			return errors.BadRequest("invalid resource type: " + string(rt))
		}
	}

	if err := h.verifyAdminAccess(c, targetAppID, userID); err != nil {
		return err
	}
	if err := h.verifyAdminAccess(c, req.SourceAppID, userID); err != nil {
		return err
	}

	result := resourcelink.ImportResult{
		Linked:  make(map[resourcelink.ResourceType]int),
		Skipped: make(map[resourcelink.ResourceType]int),
	}

	for _, rt := range req.ResourceTypes {
		ids, err := h.listResourceIDs(c, req.SourceAppID, rt)
		if err != nil {
			h.logger.Error("Failed to list source resources",
				zap.String("source_app_id", req.SourceAppID),
				zap.String("type", string(rt)), zap.Error(err))
			continue
		}

		var batch []*resourcelink.Link
		for _, rid := range ids {
			exists, _ := h.linkRepo.Exists(c.Context(), targetAppID, rt, rid)
			if exists {
				result.Skipped[rt]++
				continue
			}
			batch = append(batch, &resourcelink.Link{
				LinkID:       uuid.New().String(),
				TargetAppID:  targetAppID,
				SourceAppID:  req.SourceAppID,
				ResourceType: rt,
				ResourceID:   rid,
				LinkedBy:     userID,
				LinkedAt:     time.Now().UTC(),
			})
		}

		if len(batch) > 0 {
			if err := h.linkRepo.BulkCreate(c.Context(), batch); err != nil {
				h.logger.Error("Failed to create links",
					zap.String("type", string(rt)), zap.Error(err))
				continue
			}
		}
		result.Linked[rt] = len(batch)
	}

	h.logger.Info("Cross-app import completed",
		zap.String("source", req.SourceAppID),
		zap.String("target", targetAppID),
		zap.Any("linked", result.Linked),
		zap.Any("skipped", result.Skipped))

	return c.JSON(fiber.Map{"success": true, "data": result})
}

// ListLinks handles GET /v1/apps/:id/links
func (h *ImportHandler) ListLinks(c *fiber.Ctx) error {
	appID := c.Params("id")
	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		return errors.Unauthorized("authentication required")
	}
	if err := h.verifyAdminAccess(c, appID, userID); err != nil {
		return err
	}

	var rtFilter *resourcelink.ResourceType
	if rt := c.Query("resource_type"); rt != "" {
		typed := resourcelink.ResourceType(rt)
		if !resourcelink.ValidTypes[typed] {
			return errors.BadRequest("invalid resource_type filter")
		}
		rtFilter = &typed
	}

	links, err := h.linkRepo.ListByTarget(c.Context(), appID, rtFilter)
	if err != nil {
		return errors.Internal("failed to list links", err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    fiber.Map{"links": links, "total_count": len(links)},
	})
}

// Unlink handles DELETE /v1/apps/:id/links/:link_id
func (h *ImportHandler) Unlink(c *fiber.Ctx) error {
	appID := c.Params("id")
	linkID := c.Params("link_id")
	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		return errors.Unauthorized("authentication required")
	}
	if err := h.verifyAdminAccess(c, appID, userID); err != nil {
		return err
	}
	if err := h.linkRepo.Delete(c.Context(), linkID); err != nil {
		return errors.Internal("failed to remove link", err)
	}
	return c.JSON(fiber.Map{"success": true, "message": "link removed"})
}

// UnlinkAll handles DELETE /v1/apps/:id/links
func (h *ImportHandler) UnlinkAll(c *fiber.Ctx) error {
	appID := c.Params("id")
	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		return errors.Unauthorized("authentication required")
	}
	if err := h.verifyAdminAccess(c, appID, userID); err != nil {
		return err
	}
	if err := h.linkRepo.DeleteAllByTarget(c.Context(), appID); err != nil {
		return errors.Internal("failed to remove links", err)
	}
	return c.JSON(fiber.Map{"success": true, "message": "all links removed"})
}

func (h *ImportHandler) verifyAdminAccess(c *fiber.Ctx, appID, userID string) error {
	app, err := h.appService.GetByID(c.Context(), appID)
	if err != nil {
		return errors.NotFound("application", appID)
	}
	if app.AdminUserID == userID {
		return nil
	}
	if h.membershipRepo != nil {
		m, mErr := h.membershipRepo.GetByAppAndUser(c.Context(), appID, userID)
		if mErr == nil && m != nil && (m.Role == auth.RoleAdmin || m.Role == auth.RoleOwner) {
			return nil
		}
	}
	return errors.Forbidden("admin or owner access required on both source and target applications")
}

func (h *ImportHandler) listResourceIDs(c *fiber.Ctx, appID string, rt resourcelink.ResourceType) ([]string, error) {
	ctx := c.Context()
	switch rt {
	case resourcelink.TypeUser:
		users, err := h.userRepo.List(ctx, user.UserFilter{AppID: appID, Limit: 10000})
		if err != nil {
			return nil, err
		}
		ids := make([]string, len(users))
		for i, u := range users {
			ids[i] = u.UserID
		}
		return ids, nil

	case resourcelink.TypeTemplate:
		if h.templateService == nil {
			return nil, nil
		}
		tmpls, err := h.templateService.List(ctx, templateDomain.Filter{AppID: appID, Limit: 10000})
		if err != nil {
			return nil, err
		}
		ids := make([]string, len(tmpls))
		for i, t := range tmpls {
			ids[i] = t.ID
		}
		return ids, nil

	case resourcelink.TypeWorkflow:
		if h.workflowService == nil {
			return nil, nil
		}
		wfs, _, err := h.workflowService.List(ctx, appID, "", 10000, 0)
		if err != nil {
			return nil, err
		}
		ids := make([]string, len(wfs))
		for i, w := range wfs {
			ids[i] = w.ID
		}
		return ids, nil

	case resourcelink.TypeDigest:
		if h.digestService == nil {
			return nil, nil
		}
		rules, _, err := h.digestService.List(ctx, appID, "", 10000, 0)
		if err != nil {
			return nil, err
		}
		ids := make([]string, len(rules))
		for i, r := range rules {
			ids[i] = r.ID
		}
		return ids, nil

	case resourcelink.TypeTopic:
		if h.topicService == nil {
			return nil, nil
		}
		topics, _, err := h.topicService.List(ctx, appID, "", 10000, 0)
		if err != nil {
			return nil, err
		}
		ids := make([]string, len(topics))
		for i, t := range topics {
			ids[i] = t.ID
		}
		return ids, nil

	case resourcelink.TypeProvider:
		app, err := h.appService.GetByID(ctx, appID)
		if err != nil {
			return nil, err
		}
		if len(app.Settings.CustomProviders) == 0 {
			return nil, nil
		}
		ids := make([]string, len(app.Settings.CustomProviders))
		for i, p := range app.Settings.CustomProviders {
			ids[i] = p.ProviderID
		}
		return ids, nil
	}
	return nil, nil
}
