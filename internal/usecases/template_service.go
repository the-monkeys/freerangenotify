package usecases

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
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
	// Default locale to "en" if not provided
	if req.Locale == "" {
		req.Locale = "en"
	}

	// Validate channel
	validChannels := map[string]bool{
		"push": true, "email": true, "sms": true, "webhook": true,
		"in_app": true, "sse": true, "slack": true, "discord": true, "whatsapp": true,
	}
	if !validChannels[req.Channel] {
		return nil, fmt.Errorf("invalid channel: %s", req.Channel)
	}

	// Auto-detect variables from body if not explicitly provided
	if len(req.Variables) == 0 {
		req.Variables = extractVariables(req.Body)
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
		AppID:         req.AppID,
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		Description:   req.Description,
		Channel:       req.Channel,
		WebhookTarget: req.WebhookTarget,
		Subject:       req.Subject,
		Body:          req.Body,
		Variables:     req.Variables,
		Metadata:      req.Metadata,
		Version:       1,
		Status:        "active",
		Locale:        req.Locale,
		CreatedBy:     req.CreatedBy,
		UpdatedBy:     req.CreatedBy,
	}

	// Set Webhooks from domain model
	// (Ensure they are passed through to repo)

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

// GetByID retrieves a template by ID and verifies app ownership
func (s *TemplateService) GetByID(ctx context.Context, id, appID string) (*templateDomain.Template, error) {
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	if tmpl.AppID != appID {
		return nil, fmt.Errorf("template not found")
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

// Update updates an existing template in-place.
// The template ID remains stable across edits. Use CreateVersion explicitly
// to snapshot the current state into a new version document.
func (s *TemplateService) Update(ctx context.Context, id, appID string, req *templateDomain.UpdateRequest) (*templateDomain.Template, error) {
	// Get existing template
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	// Verify ownership
	if tmpl.AppID != appID {
		return nil, fmt.Errorf("template not found")
	}

	// Apply field updates
	if req.Name != nil && *req.Name != "" {
		tmpl.Name = *req.Name
	}
	if req.Description != nil && *req.Description != "" {
		tmpl.Description = *req.Description
	}
	if req.WebhookTarget != nil {
		tmpl.WebhookTarget = *req.WebhookTarget
	}
	if req.Subject != nil {
		tmpl.Subject = *req.Subject
	}
	if req.Body != nil && *req.Body != "" {
		tmpl.Body = *req.Body
		// Auto-detect variables from new body if variables not explicitly overridden
		if req.Variables == nil || len(*req.Variables) == 0 {
			tmpl.Variables = extractVariables(tmpl.Body)
		}
		// Validate new body against variables
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
	if req.Controls != nil {
		tmpl.Controls = *req.Controls
	}
	if req.ControlValues != nil {
		tmpl.ControlValues = req.ControlValues
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

	s.logger.Info("Updated template",
		zap.String("id", tmpl.ID),
		zap.String("name", tmpl.Name),
		zap.Int("version", tmpl.Version))
	return tmpl, nil
}

// Delete permanently removes a template from the datastore.
func (s *TemplateService) Delete(ctx context.Context, id, appID string) error {
	// Verify ownership before delete
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("template not found: %w", err)
	}
	if tmpl.AppID != appID {
		return fmt.Errorf("template not found")
	}

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

// Render renders a template with provided data.
// When editable is true, text-content variables are wrapped in contenteditable
// spans so the frontend preview can support inline editing. Attribute variables
// (inside HTML tags like src=, href=) are returned separately for sidebar editing.
func (s *TemplateService) Render(ctx context.Context, templateID, appID string, data map[string]interface{}, editable bool) (string, []templateDomain.AttributeVar, error) {
	// Get template
	tmpl, err := s.repo.GetByID(ctx, templateID)
	if err != nil {
		return "", nil, fmt.Errorf("template not found: %w", err)
	}

	// Verify ownership
	if tmpl.AppID != appID {
		return "", nil, fmt.Errorf("template not found")
	}

	// Check if template is active
	if tmpl.Status != "active" {
		return "", nil, fmt.Errorf("template is not active")
	}

	body := tmpl.Body
	var attrVars []templateDomain.AttributeVar
	if editable {
		attrVars = classifyAttributeVariables(body)
		body = wrapEditableVariables(body)
	}

	// Render template
	rendered, err := s.renderTemplate(body, data)
	if err != nil {
		s.logger.Error("Failed to render template",
			zap.String("id", templateID),
			zap.Error(err))
		return "", nil, fmt.Errorf("failed to render template: %w", err)
	}

	return rendered, attrVars, nil
}

// CreateVersion creates a new version of a template
func (s *TemplateService) CreateVersion(ctx context.Context, templateID, appID, updatedBy string) (*templateDomain.Template, error) {
	// Get the current template
	current, err := s.repo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	// Verify ownership
	if current.AppID != appID {
		return nil, fmt.Errorf("template not found")
	}

	// Create new version with same content
	tmpl := &templateDomain.Template{
		AppID:         current.AppID,
		Name:          current.Name,
		Description:   current.Description,
		Channel:       current.Channel,
		WebhookTarget: current.WebhookTarget,
		Subject:       current.Subject,
		Body:          current.Body,
		Variables:     current.Variables,
		Metadata:      current.Metadata,
		Status:        "active",
		Locale:        current.Locale,
		CreatedBy:     updatedBy,
		UpdatedBy:     updatedBy,
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

// GetByVersion retrieves a specific version of a template by its version number.
func (s *TemplateService) GetByVersion(ctx context.Context, appID, name, locale string, version int) (*templateDomain.Template, error) {
	if version < 1 {
		return nil, fmt.Errorf("version must be >= 1")
	}

	tmpl, err := s.repo.GetByVersion(ctx, appID, name, locale, version)
	if err != nil {
		return nil, fmt.Errorf("failed to get template version %d: %w", version, err)
	}

	return tmpl, nil
}

// Rollback creates a new version whose content is copied from the specified target version.
// It never deletes history — the rollback result is a new version.
func (s *TemplateService) Rollback(ctx context.Context, templateID, appID string, targetVersion int, updatedBy string) (*templateDomain.Template, error) {
	current, err := s.repo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	if current.AppID != appID {
		return nil, fmt.Errorf("template not found")
	}

	target, err := s.repo.GetByVersion(ctx, current.AppID, current.Name, current.Locale, targetVersion)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", targetVersion, err)
	}

	rollback := &templateDomain.Template{
		AppID:         current.AppID,
		Name:          current.Name,
		Description:   target.Description,
		Channel:       target.Channel,
		WebhookTarget: target.WebhookTarget,
		Subject:       target.Subject,
		Body:          target.Body,
		Variables:     target.Variables,
		Metadata:      target.Metadata,
		Locale:        current.Locale,
		Status:        "active",
		CreatedBy:     updatedBy,
		UpdatedBy:     updatedBy,
	}

	if err := s.repo.CreateVersion(ctx, rollback); err != nil {
		s.logger.Error("Failed to rollback template",
			zap.String("template_id", templateID),
			zap.Int("target_version", targetVersion),
			zap.Error(err))
		return nil, fmt.Errorf("failed to rollback template: %w", err)
	}

	s.logger.Info("Template rolled back",
		zap.String("template_id", templateID),
		zap.String("name", current.Name),
		zap.Int("from_version", current.Version),
		zap.Int("to_version", targetVersion),
		zap.Int("new_version", rollback.Version))

	return rollback, nil
}

// Diff compares two versions of a template and returns field-level changes.
func (s *TemplateService) Diff(ctx context.Context, appID, name, locale string, fromVersion, toVersion int) (*templateDomain.TemplateDiff, error) {
	from, err := s.repo.GetByVersion(ctx, appID, name, locale, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", fromVersion, err)
	}
	to, err := s.repo.GetByVersion(ctx, appID, name, locale, toVersion)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", toVersion, err)
	}

	if from.AppID != appID || to.AppID != appID {
		return nil, fmt.Errorf("template not found")
	}

	diff := &templateDomain.TemplateDiff{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	}

	if from.Subject != to.Subject {
		diff.Changes = append(diff.Changes, templateDomain.FieldChange{Field: "subject", From: from.Subject, To: to.Subject})
	}
	if from.Body != to.Body {
		diff.Changes = append(diff.Changes, templateDomain.FieldChange{Field: "body", From: from.Body, To: to.Body})
	}
	if from.Description != to.Description {
		diff.Changes = append(diff.Changes, templateDomain.FieldChange{Field: "description", From: from.Description, To: to.Description})
	}
	if from.Channel != to.Channel {
		diff.Changes = append(diff.Changes, templateDomain.FieldChange{Field: "channel", From: from.Channel, To: to.Channel})
	}
	if from.WebhookTarget != to.WebhookTarget {
		diff.Changes = append(diff.Changes, templateDomain.FieldChange{Field: "webhook_target", From: from.WebhookTarget, To: to.WebhookTarget})
	}
	if !reflect.DeepEqual(from.Variables, to.Variables) {
		diff.Changes = append(diff.Changes, templateDomain.FieldChange{Field: "variables", From: from.Variables, To: to.Variables})
	}
	if !reflect.DeepEqual(from.Metadata, to.Metadata) {
		diff.Changes = append(diff.Changes, templateDomain.FieldChange{Field: "metadata", From: from.Metadata, To: to.Metadata})
	}

	if diff.Changes == nil {
		diff.Changes = []templateDomain.FieldChange{}
	}

	s.logger.Info("Template diff computed",
		zap.String("name", name),
		zap.Int("from", fromVersion),
		zap.Int("to", toVersion),
		zap.Int("changes", len(diff.Changes)))

	return diff, nil
}

// renderSubjectTemplate renders a subject line template with data.
func (s *TemplateService) RenderSubject(subjectTmpl string, data map[string]interface{}) string {
	rendered, err := s.renderTemplate(subjectTmpl, data)
	if err != nil {
		return subjectTmpl // Fallback to raw subject on error
	}
	return rendered
}

// extractVariables parses Go template variables ({{.varName}}) from body text.
// Returns a deduplicated, sorted slice of variable names.
func extractVariables(body string) []string {
	re := regexp.MustCompile(`\{\{\s*\.?(\w+)\s*\}\}`)
	matches := re.FindAllStringSubmatch(body, -1)

	seen := make(map[string]struct{})
	var vars []string
	for _, match := range matches {
		if len(match) > 1 {
			name := match[1]
			if _, exists := seen[name]; !exists {
				seen[name] = struct{}{}
				vars = append(vars, name)
			}
		}
	}
	sort.Strings(vars)
	return vars
}

// validateTemplateVariables validates that all variables used in the template body are defined
func (s *TemplateService) validateTemplateVariables(body string, variables []string) error {
	// Go template action keywords that are not variable references
	templateKeywords := map[string]bool{
		"if": true, "else": true, "end": true, "range": true,
		"with": true, "define": true, "template": true, "block": true,
		"nil": true, "not": true, "and": true, "or": true,
		"eq": true, "ne": true, "lt": true, "le": true, "gt": true, "ge": true,
		"print": true, "printf": true, "println": true, "len": true,
		"index": true, "call": true, "html": true, "js": true, "urlquery": true,
	}

	// Extract variables from template using {{.variable}} pattern
	re := regexp.MustCompile(`\{\{\.?(\w+)\}\}`)
	matches := re.FindAllStringSubmatch(body, -1)

	usedVars := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 && !templateKeywords[match[1]] {
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

// wrapEditableVariables preprocesses a template body so that {{.varName}}
// placeholders appearing in HTML text content are wrapped in
// <span contenteditable data-frn-var="varName" class="frn-editable">...</span>.
// Variables inside HTML attributes (e.g. href="{{.url}}") are left untouched.
func wrapEditableVariables(body string) string {
	varPattern := regexp.MustCompile(`\{\{\s*\.?(\w+)\s*\}\}`)

	matches := varPattern.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return body
	}

	// Go template keywords that are not variable references.
	keywords := map[string]bool{
		"if": true, "else": true, "end": true, "range": true,
		"with": true, "define": true, "template": true, "block": true,
		"nil": true, "not": true, "and": true, "or": true,
		"eq": true, "ne": true, "lt": true, "le": true, "gt": true, "ge": true,
		"print": true, "printf": true, "println": true, "len": true,
		"index": true, "call": true, "html": true, "js": true, "urlquery": true,
	}

	var result strings.Builder
	result.Grow(len(body) + len(matches)*80) // pre-allocate
	lastEnd := 0

	for _, match := range matches {
		start := match[0]
		end := match[1]
		varNameStart := match[2]
		varNameEnd := match[3]
		varName := body[varNameStart:varNameEnd]

		result.WriteString(body[lastEnd:start])

		if keywords[varName] || isInsideHTMLTag(body, start) {
			result.WriteString(body[start:end])
		} else {
			result.WriteString(`<span contenteditable="true" data-frn-var="`)
			result.WriteString(varName)
			result.WriteString(`" class="frn-editable">`)
			result.WriteString(body[start:end])
			result.WriteString(`</span>`)
		}

		lastEnd = end
	}

	result.WriteString(body[lastEnd:])
	return result.String()
}

// isInsideHTMLTag returns true if the byte at position pos sits between
// an unmatched '<' and '>' — i.e. inside an HTML tag definition.
// Template expressions ({{ ... }}) are skipped so that '<' or '>' used as
// comparison operators inside template actions don't confuse the check.
func isInsideHTMLTag(body string, pos int) bool {
	inTag := false
	for i := 0; i < pos; i++ {
		// Skip template expressions.
		if i+1 < len(body) && body[i] == '{' && body[i+1] == '{' {
			j := i + 2
			for j+1 < len(body) {
				if body[j] == '}' && body[j+1] == '}' {
					i = j + 1
					break
				}
				j++
			}
			continue
		}
		if body[i] == '<' {
			inTag = true
		} else if body[i] == '>' {
			inTag = false
		}
	}
	return inTag
}

// classifyAttributeVariables scans a template body and returns variables that
// appear inside HTML attributes (src=, href=, etc.) — these cannot be edited
// inline via contenteditable and need a separate sidebar UI.
// Each variable is classified as "image" (inside <img src>), "url" (inside href),
// or "attribute" (any other HTML attribute).
func classifyAttributeVariables(body string) []templateDomain.AttributeVar {
	varPattern := regexp.MustCompile(`\{\{\s*\.?(\w+)\s*\}\}`)
	matches := varPattern.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return nil
	}

	keywords := map[string]bool{
		"if": true, "else": true, "end": true, "range": true,
		"with": true, "define": true, "template": true, "block": true,
		"nil": true, "not": true, "and": true, "or": true,
		"eq": true, "ne": true, "lt": true, "le": true, "gt": true, "ge": true,
		"print": true, "printf": true, "println": true, "len": true,
		"index": true, "call": true, "html": true, "js": true, "urlquery": true,
	}

	seen := make(map[string]bool)
	var result []templateDomain.AttributeVar

	for _, match := range matches {
		start := match[0]
		varNameStart := match[2]
		varNameEnd := match[3]
		varName := body[varNameStart:varNameEnd]

		if keywords[varName] || seen[varName] {
			continue
		}
		if !isInsideHTMLTag(body, start) {
			continue // text-content variable — handled inline by contenteditable
		}

		seen[varName] = true
		varType := classifyAttrVarType(body, start)
		result = append(result, templateDomain.AttributeVar{
			Name: varName,
			Type: varType,
		})
	}

	return result
}

// classifyAttrVarType determines the type of an attribute variable by looking
// at the surrounding HTML context. Returns "image", "url", or "attribute".
func classifyAttrVarType(body string, pos int) string {
	// Find the opening '<' of the current tag
	tagStart := strings.LastIndex(body[:pos], "<")
	if tagStart < 0 {
		return "attribute"
	}
	tagContent := strings.ToLower(body[tagStart:pos])

	// Check if inside an <img> tag's src attribute
	if strings.HasPrefix(tagContent, "<img") && strings.Contains(tagContent, "src") {
		return "image"
	}
	// Check for background-image in style attributes
	if strings.Contains(tagContent, "background-image") || strings.Contains(tagContent, "background:") {
		return "image"
	}
	// Check if inside an href attribute
	if strings.Contains(tagContent, "href") {
		return "url"
	}
	// Check if inside a src attribute on non-img tags (video, source, etc.)
	if strings.Contains(tagContent, "src") {
		return "url"
	}
	// Check for action attribute (forms)
	if strings.Contains(tagContent, "action") {
		return "url"
	}
	return "attribute"
}

// renderTemplate renders a Go template with provided data
func (s *TemplateService) renderTemplate(body string, data map[string]interface{}) (string, error) {
	// Support both {{var}} and {{.var}} by ensuring data is accessible as dot
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
