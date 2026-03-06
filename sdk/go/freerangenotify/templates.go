package freerangenotify

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// TemplatesClient handles template operations.
type TemplatesClient struct {
	client *Client
}

// Create creates a new notification template.
func (t *TemplatesClient) Create(ctx context.Context, params CreateTemplateParams) (*Template, error) {
	var result Template
	if err := t.client.do(ctx, "POST", "/templates/", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Get retrieves a template by ID.
func (t *TemplatesClient) Get(ctx context.Context, templateID string) (*Template, error) {
	var result Template
	if err := t.client.do(ctx, "GET", "/templates/"+templateID, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Update modifies an existing template.
func (t *TemplatesClient) Update(ctx context.Context, templateID string, params UpdateTemplateParams) (*Template, error) {
	var result Template
	if err := t.client.do(ctx, "PUT", "/templates/"+templateID, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete removes a template.
func (t *TemplatesClient) Delete(ctx context.Context, templateID string) error {
	return t.client.do(ctx, "DELETE", "/templates/"+templateID, nil, nil)
}

// ListTemplatesOptions configures the template list query.
type ListTemplatesOptions struct {
	AppID   string
	Channel string
	Name    string
	Status  string
	Locale  string
	Limit   int
	Offset  int
}

// List returns a paginated list of templates.
func (t *TemplatesClient) List(ctx context.Context, opts ListTemplatesOptions) (*TemplateListResponse, error) {
	q := url.Values{}
	if opts.AppID != "" {
		q.Set("app_id", opts.AppID)
	}
	if opts.Channel != "" {
		q.Set("channel", opts.Channel)
	}
	if opts.Name != "" {
		q.Set("name", opts.Name)
	}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.Locale != "" {
		q.Set("locale", opts.Locale)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Offset > 0 {
		q.Set("offset", strconv.Itoa(opts.Offset))
	}

	var result TemplateListResponse
	if err := t.client.doWithQuery(ctx, "GET", "/templates/", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Library ──

// GetLibrary retrieves templates from the shared library, optionally filtered by category.
func (t *TemplatesClient) GetLibrary(ctx context.Context, category string) ([]Template, error) {
	q := url.Values{}
	if category != "" {
		q.Set("category", category)
	}

	var result struct {
		Templates []Template `json:"templates"`
	}
	if err := t.client.doWithQuery(ctx, "GET", "/templates/library", q, &result); err != nil {
		return nil, err
	}
	return result.Templates, nil
}

// CloneFromLibrary clones a library template into the application's template space.
func (t *TemplatesClient) CloneFromLibrary(ctx context.Context, name string, params CloneTemplateParams) (*Template, error) {
	var result Template
	if err := t.client.do(ctx, "POST", "/templates/library/"+name+"/clone", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Versioning ──

// GetVersions retrieves all versions of a template.
func (t *TemplatesClient) GetVersions(ctx context.Context, appID, name string) ([]Template, error) {
	var result struct {
		Versions []Template `json:"versions"`
	}
	if err := t.client.do(ctx, "GET", fmt.Sprintf("/templates/%s/%s/versions", appID, name), nil, &result); err != nil {
		return nil, err
	}
	return result.Versions, nil
}

// CreateVersion creates a new version of a template.
func (t *TemplatesClient) CreateVersion(ctx context.Context, appID, name string, params CreateVersionParams) (*Template, error) {
	var result Template
	if err := t.client.do(ctx, "POST", fmt.Sprintf("/templates/%s/%s/versions", appID, name), params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Rollback ──

// Rollback reverts a template to a specific previous version.
func (t *TemplatesClient) Rollback(ctx context.Context, templateID string, version int, updatedBy string) (*Template, error) {
	var result Template
	payload := map[string]interface{}{"version": version, "updated_by": updatedBy}
	if err := t.client.do(ctx, "POST", "/templates/"+templateID+"/rollback", payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Diff ──

// Diff compares two template versions and returns their differences.
func (t *TemplatesClient) Diff(ctx context.Context, templateID string, fromVersion, toVersion int) (*TemplateDiff, error) {
	q := url.Values{}
	q.Set("from", strconv.Itoa(fromVersion))
	q.Set("to", strconv.Itoa(toVersion))

	var result TemplateDiff
	if err := t.client.doWithQuery(ctx, "GET", "/templates/"+templateID+"/diff", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Render ──

// Render renders a template with the given data and returns the output.
func (t *TemplatesClient) Render(ctx context.Context, templateID string, data map[string]interface{}) (string, error) {
	var result struct {
		RenderedBody string `json:"rendered_body"`
	}
	payload := map[string]interface{}{"data": data}
	if err := t.client.do(ctx, "POST", "/templates/"+templateID+"/render", payload, &result); err != nil {
		return "", err
	}
	return result.RenderedBody, nil
}

// ── Send Test ──

// SendTest sends a test email using a rendered template.
func (t *TemplatesClient) SendTest(ctx context.Context, templateID, toEmail string, sampleData map[string]interface{}) error {
	payload := map[string]interface{}{
		"to_email":    toEmail,
		"sample_data": sampleData,
	}
	return t.client.do(ctx, "POST", "/templates/"+templateID+"/test", payload, nil)
}

// ── Content Controls ──

// GetControls returns the template's control definitions and current values.
func (t *TemplatesClient) GetControls(ctx context.Context, templateID string) (*ControlsResponse, error) {
	var resp ControlsResponse
	if err := t.client.do(ctx, "GET", "/templates/"+templateID+"/controls", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateControls saves validated control values for a template.
func (t *TemplatesClient) UpdateControls(ctx context.Context, templateID string, values ControlValues) error {
	return t.client.do(ctx, "PUT", "/templates/"+templateID+"/controls", values, nil)
}
