package usecases

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type environmentService struct {
	envRepo  environment.Repository
	tmplRepo template.Repository
	wfRepo   workflow.Repository
	logger   *zap.Logger
}

// NewEnvironmentService creates a new environment service.
func NewEnvironmentService(
	envRepo environment.Repository,
	tmplRepo template.Repository,
	wfRepo workflow.Repository,
	logger *zap.Logger,
) environment.Service {
	return &environmentService{
		envRepo:  envRepo,
		tmplRepo: tmplRepo,
		wfRepo:   wfRepo,
		logger:   logger,
	}
}

func (s *environmentService) Create(ctx context.Context, req environment.CreateRequest) (*environment.Environment, error) {
	// Check for duplicate environment name within the app
	existing, _ := s.envRepo.ListByApp(ctx, req.AppID)
	for _, e := range existing {
		if e.Name == req.Name {
			return nil, errors.Conflict(fmt.Sprintf("environment %q already exists for this app", req.Name))
		}
	}

	slug := slugFromName(req.Name)
	env := &environment.Environment{
		ID:        uuid.New().String(),
		AppID:     req.AppID,
		Name:      req.Name,
		Slug:      slug,
		APIKey:    fmt.Sprintf("frn_%s_%s", slug, generateEnvKey(24)),
		IsDefault: req.Name == "production",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.envRepo.Create(ctx, env); err != nil {
		return nil, errors.Internal("failed to create environment", err)
	}

	s.logger.Info("Environment created",
		zap.String("env_id", env.ID),
		zap.String("app_id", env.AppID),
		zap.String("name", env.Name),
	)
	return env, nil
}

func (s *environmentService) Get(ctx context.Context, id string) (*environment.Environment, error) {
	env, err := s.envRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if env == nil {
		return nil, errors.NotFound("Environment", id)
	}
	return env, nil
}

func (s *environmentService) GetByAPIKey(ctx context.Context, apiKey string) (*environment.Environment, error) {
	env, err := s.envRepo.GetByAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	if env == nil {
		return nil, errors.NotFound("Environment", "api_key")
	}
	return env, nil
}

func (s *environmentService) ListByApp(ctx context.Context, appID string) ([]environment.Environment, error) {
	return s.envRepo.ListByApp(ctx, appID)
}

func (s *environmentService) Delete(ctx context.Context, id string) error {
	env, err := s.envRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if env == nil {
		return errors.NotFound("Environment", id)
	}
	if env.IsDefault {
		return errors.BadRequest("cannot delete the default (production) environment")
	}
	return s.envRepo.Delete(ctx, id)
}

func (s *environmentService) Promote(ctx context.Context, appID string, req environment.PromoteRequest) (*environment.PromoteResult, error) {
	// Validate source and target belong to the same app
	source, err := s.envRepo.GetByID(ctx, req.SourceEnvID)
	if err != nil || source == nil {
		return nil, errors.NotFound("Environment", req.SourceEnvID)
	}
	target, err := s.envRepo.GetByID(ctx, req.TargetEnvID)
	if err != nil || target == nil {
		return nil, errors.NotFound("Environment", req.TargetEnvID)
	}
	if source.AppID != appID || target.AppID != appID {
		return nil, errors.BadRequest("source and target environments must belong to the same app")
	}

	result := &environment.PromoteResult{}

	for _, resource := range req.Resources {
		switch resource {
		case "templates":
			count, err := s.promoteTemplates(ctx, appID, source.ID, target.ID)
			if err != nil {
				return nil, err
			}
			result.TemplatesPromoted = count

		case "workflows":
			count, err := s.promoteWorkflows(ctx, appID, source.ID, target.ID)
			if err != nil {
				return nil, err
			}
			result.WorkflowsPromoted = count

		default:
			return nil, errors.BadRequest(fmt.Sprintf("unknown resource type: %s", resource))
		}
	}

	s.logger.Info("Environment promotion completed",
		zap.String("app_id", appID),
		zap.String("source", source.Name),
		zap.String("target", target.Name),
		zap.Int("templates", result.TemplatesPromoted),
		zap.Int("workflows", result.WorkflowsPromoted),
	)

	return result, nil
}

func (s *environmentService) promoteTemplates(ctx context.Context, appID, sourceEnvID, targetEnvID string) (int, error) {
	// List all templates in source environment
	filter := template.Filter{
		AppID:         appID,
		EnvironmentID: sourceEnvID,
		Limit:         1000,
	}
	templates, err := s.tmplRepo.List(ctx, filter)
	if err != nil {
		return 0, errors.Internal("failed to list source templates", err)
	}

	count := 0
	for _, tmpl := range templates {
		// Check if same-named template exists in target
		existing, _ := s.tmplRepo.GetByAppAndName(ctx, appID, tmpl.Name, tmpl.Locale)

		if existing != nil && existing.EnvironmentID == targetEnvID {
			// Update existing template in target
			existing.Subject = tmpl.Subject
			existing.Body = tmpl.Body
			existing.Variables = tmpl.Variables
			existing.Metadata = tmpl.Metadata
			existing.Controls = tmpl.Controls
			existing.ControlValues = tmpl.ControlValues
			existing.UpdatedAt = time.Now().UTC()
			if err := s.tmplRepo.Update(ctx, existing); err != nil {
				return count, errors.Internal("failed to update promoted template", err)
			}
		} else {
			// Clone to target environment
			clone := *tmpl
			clone.ID = uuid.New().String()
			clone.EnvironmentID = targetEnvID
			clone.Version = 1
			clone.CreatedAt = time.Now().UTC()
			clone.UpdatedAt = time.Now().UTC()
			if err := s.tmplRepo.Create(ctx, &clone); err != nil {
				return count, errors.Internal("failed to create promoted template", err)
			}
		}
		count++
	}
	return count, nil
}

func (s *environmentService) promoteWorkflows(ctx context.Context, appID, sourceEnvID, targetEnvID string) (int, error) {
	if s.wfRepo == nil {
		return 0, nil
	}

	wfs, _, err := s.wfRepo.ListWorkflows(ctx, appID, sourceEnvID, 1000, 0)
	if err != nil {
		return 0, errors.Internal("failed to list source workflows", err)
	}

	count := 0
	for _, wf := range wfs {
		// Only promote workflows from the source environment
		if wf.EnvironmentID != sourceEnvID {
			continue
		}
		clone := *wf
		clone.ID = uuid.New().String()
		clone.EnvironmentID = targetEnvID
		clone.Version = 1
		clone.CreatedAt = time.Now().UTC()
		clone.UpdatedAt = time.Now().UTC()
		if err := s.wfRepo.CreateWorkflow(ctx, &clone); err != nil {
			return count, errors.Internal("failed to create promoted workflow", err)
		}
		count++
	}
	return count, nil
}

// slugFromName converts an environment name to a short slug.
func slugFromName(name string) string {
	switch name {
	case "development":
		return "dev"
	case "staging":
		return "stg"
	case "production":
		return "prod"
	default:
		return name
	}
}

// generateEnvKey generates a URL-safe random string of the given byte length.
func generateEnvKey(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:n]
}
