package usecases

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
	"go.uber.org/zap"
)

type TemplateService struct {
	repo   templateDomain.Repository
	logger *zap.Logger
}

func NewTemplateService(repo templateDomain.Repository, logger *zap.Logger) *TemplateService {
	return &TemplateService{
		repo:   repo,
		logger: logger,
	}
}

// Create creates a new template with validation
func (s *TemplateService) Create(ctx context.Context, req *templateDomain.CreateRequest) (*templateDomain.Template, error) {
	// Validate channel
	validChannels := map[string]bool{"push": true, "email": true, "sms": true, "webhook": true, "in_app": true}
	if !validChannels[req.Channel] {
		return nil, fmt.Errorf("invalid channel: %s", req.Channel)
	}

	// Validate variables in template
	if err := s.validateTemplateVariables(req.Body, req.Variables); err != nil {
		return nil, fmt.Errorf("template validation failed: %w", err)
	}

	// Check if template with same name already exists
	existing, err := s.repo.GetByAppAndName(ctx, req.AppID, req.Name, req.Locale)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("template with name '%s' already exists for this app and locale", req.Name)
	}

	tmpl := &templateDomain.Template{
		AppID:       req.AppID,
		Name:        req.Name,
		Description: req.Description,
		Channel:     req.Channel,
		Subject:     req.Subject,
		Body:        req.Body,
		Variables:   req.Variables,
		Metadata:    req.Metadata,
		Version:     1,
		Status:      "active",
		Locale:      req.Locale,
		CreatedBy:   req.CreatedBy,
		UpdatedBy:   req.CreatedBy,
	}

	if err := s.repo.Create(ctx, tmpl); err != nil {
		s.logger.Error("Failed to create template", zap.Error(err))
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	s.logger.Info("Created template",
		zap.String("id", tmpl.ID),
		zap.String("name", tmpl.Name),
		zap.String("app_id", tmpl.AppID))

	return tmpl, nil
}

// GetByID retrieves a template by ID
func (s *TemplateService) GetByID(ctx context.Context, id string) (*templateDomain.Template, error) {
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return tmpl, nil
}

// GetByName retrieves the latest active template by app ID, name, and locale
func (s *TemplateService) GetByName(ctx context.Context, appID, name, locale string) (*templateDomain.Template, error) {
	tmpl, err := s.repo.GetByAppAndName(ctx, appID, name, locale)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return tmpl, nil
}

// Update updates an existing template
func (s *TemplateService) Update(ctx context.Context, id string, req *templateDomain.UpdateRequest) (*templateDomain.Template, error) {
	// Get existing template
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	// Update fields if provided
	if req.Name != nil && *req.Name != "" {
		tmpl.Name = *req.Name
	}
	if req.Description != nil && *req.Description != "" {
		tmpl.Description = *req.Description
	}
	if req.Subject != nil {
		tmpl.Subject = *req.Subject
	}
	if req.Body != nil && *req.Body != "" {
		tmpl.Body = *req.Body
		// Validate new body
		if err := s.validateTemplateVariables(tmpl.Body, tmpl.Variables); err != nil {
			return nil, fmt.Errorf("template validation failed: %w", err)
		}
	}
	if req.Variables != nil && len(*req.Variables) > 0 {
		tmpl.Variables = *req.Variables
	}
	if req.Metadata != nil {
		tmpl.Metadata = req.Metadata
	}
	if req.Status != nil && *req.Status != "" {
		validStatuses := map[string]bool{"active": true, "inactive": true, "archived": true}
		if !validStatuses[*req.Status] {
			return nil, fmt.Errorf("invalid status: %s", *req.Status)
		}
		tmpl.Status = *req.Status
	}
	if req.UpdatedBy != "" {
		tmpl.UpdatedBy = req.UpdatedBy
	}

	if err := s.repo.Update(ctx, tmpl); err != nil {
		s.logger.Error("Failed to update template", zap.String("id", id), zap.Error(err))
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	s.logger.Info("Updated template", zap.String("id", id))
	return tmpl, nil
}

// Delete deletes a template (soft delete)
func (s *TemplateService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete template", zap.String("id", id), zap.Error(err))
		return fmt.Errorf("failed to delete template: %w", err)
	}

	s.logger.Info("Deleted template", zap.String("id", id))
	return nil
}

// List retrieves templates based on filter
func (s *TemplateService) List(ctx context.Context, filter templateDomain.Filter) ([]*templateDomain.Template, error) {
	templates, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	return templates, nil
}

// Render renders a template with provided data
func (s *TemplateService) Render(ctx context.Context, templateID string, data map[string]interface{}) (string, error) {
	// Get template
	tmpl, err := s.repo.GetByID(ctx, templateID)
	if err != nil {
		return "", fmt.Errorf("template not found: %w", err)
	}

	// Check if template is active
	if tmpl.Status != "active" {
		return "", fmt.Errorf("template is not active")
	}

	// Render template
	rendered, err := s.renderTemplate(tmpl.Body, data)
	if err != nil {
		s.logger.Error("Failed to render template",
			zap.String("id", templateID),
			zap.Error(err))
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return rendered, nil
}

// CreateVersion creates a new version of a template
func (s *TemplateService) CreateVersion(ctx context.Context, templateID, updatedBy string) (*templateDomain.Template, error) {
	// Get the current template
	current, err := s.repo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	// Create new version with same content
	tmpl := &templateDomain.Template{
		AppID:       current.AppID,
		Name:        current.Name,
		Description: current.Description,
		Channel:     current.Channel,
		Subject:     current.Subject,
		Body:        current.Body,
		Variables:   current.Variables,
		Metadata:    current.Metadata,
		Status:      "active",
		Locale:      current.Locale,
		CreatedBy:   updatedBy,
		UpdatedBy:   updatedBy,
	}

	if err := s.repo.CreateVersion(ctx, tmpl); err != nil {
		s.logger.Error("Failed to create template version",
			zap.String("template_id", templateID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create template version: %w", err)
	}

	s.logger.Info("Created template version",
		zap.String("id", tmpl.ID),
		zap.String("name", tmpl.Name),
		zap.Int("version", tmpl.Version))

	return tmpl, nil
}

// GetVersions retrieves all versions of a template
func (s *TemplateService) GetVersions(ctx context.Context, appID, name, locale string) ([]*templateDomain.Template, error) {
	versions, err := s.repo.GetVersions(ctx, appID, name, locale)
	if err != nil {
		return nil, fmt.Errorf("failed to get template versions: %w", err)
	}

	return versions, nil
}

// validateTemplateVariables validates that all variables used in the template body are defined
func (s *TemplateService) validateTemplateVariables(body string, variables []string) error {
	// Extract variables from template using {{variable}} pattern
	re := regexp.MustCompile(`\{\{\.?(\w+)\}\}`)
	matches := re.FindAllStringSubmatch(body, -1)

	usedVars := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			usedVars[match[1]] = true
		}
	}

	// Check if all used variables are defined
	definedVars := make(map[string]bool)
	for _, v := range variables {
		definedVars[v] = true
	}

	var undefinedVars []string
	for varName := range usedVars {
		if !definedVars[varName] {
			undefinedVars = append(undefinedVars, varName)
		}
	}

	if len(undefinedVars) > 0 {
		return fmt.Errorf("undefined variables in template: %s", strings.Join(undefinedVars, ", "))
	}

	return nil
}

// renderTemplate renders a Go template with provided data
func (s *TemplateService) renderTemplate(body string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("notification").Parse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
