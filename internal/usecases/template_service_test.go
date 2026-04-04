package usecases

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
	"go.uber.org/zap"
)

// mockTemplateRepo implements template.Repository for unit testing.
type mockTemplateRepo struct {
	templates []*templateDomain.Template
}

func (m *mockTemplateRepo) Create(_ context.Context, t *templateDomain.Template) error {
	m.templates = append(m.templates, t)
	return nil
}
func (m *mockTemplateRepo) GetByID(_ context.Context, id string) (*templateDomain.Template, error) {
	for _, t := range m.templates {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("template not found")
}
func (m *mockTemplateRepo) GetByAppAndName(_ context.Context, appID, name, locale string) (*templateDomain.Template, error) {
	for _, t := range m.templates {
		if t.AppID == appID && t.Name == name && t.Locale == locale {
			return t, nil
		}
	}
	return nil, fmt.Errorf("template not found")
}
func (m *mockTemplateRepo) Update(_ context.Context, _ *templateDomain.Template) error { return nil }
func (m *mockTemplateRepo) Delete(_ context.Context, _ string) error                   { return nil }
func (m *mockTemplateRepo) Count(_ context.Context) (int64, error) {
	return int64(len(m.templates)), nil
}
func (m *mockTemplateRepo) CountByFilter(_ context.Context, f templateDomain.Filter) (int64, error) {
	count := int64(0)
	for _, t := range m.templates {
		if f.AppID != "" && t.AppID != f.AppID {
			continue
		}
		count++
	}
	return count, nil
}
func (m *mockTemplateRepo) GetVersions(_ context.Context, _, _, _ string) ([]*templateDomain.Template, error) {
	return nil, nil
}
func (m *mockTemplateRepo) GetByVersion(_ context.Context, _, _, _ string, _ int) (*templateDomain.Template, error) {
	return nil, nil
}
func (m *mockTemplateRepo) CreateVersion(_ context.Context, _ *templateDomain.Template) error {
	return nil
}

// List returns a paginated slice and total count, matching the repository contract.
func (m *mockTemplateRepo) List(_ context.Context, f templateDomain.Filter) ([]*templateDomain.Template, int64, error) {
	var filtered []*templateDomain.Template
	for _, t := range m.templates {
		if f.AppID != "" && t.AppID != f.AppID {
			continue
		}
		if f.Channel != "" && t.Channel != f.Channel {
			continue
		}
		if t.Status != "active" {
			continue
		}
		filtered = append(filtered, t)
	}

	total := int64(len(filtered))

	// Apply offset
	if f.Offset > 0 {
		if f.Offset >= len(filtered) {
			return []*templateDomain.Template{}, total, nil
		}
		filtered = filtered[f.Offset:]
	}

	// Apply limit
	if f.Limit > 0 && f.Limit < len(filtered) {
		filtered = filtered[:f.Limit]
	}

	return filtered, total, nil
}

func seedTemplates(repo *mockTemplateRepo, appID string, count int) {
	channels := []string{"email", "push", "sms", "webhook"}
	for i := 0; i < count; i++ {
		repo.templates = append(repo.templates, &templateDomain.Template{
			ID:        fmt.Sprintf("tmpl-%03d", i+1),
			AppID:     appID,
			Name:      fmt.Sprintf("template_%d", i+1),
			Channel:   channels[i%len(channels)],
			Body:      fmt.Sprintf("Body %d", i+1),
			Status:    "active",
			Locale:    "en",
			Version:   1,
			CreatedAt: time.Now(),
		})
	}
}

func TestTemplateService_List_ReturnsCorrectTotal(t *testing.T) {
	repo := &mockTemplateRepo{}
	logger := zap.NewNop()
	service := NewTemplateService(repo, logger)

	seedTemplates(repo, "app-001", 25)

	// Page 1: limit=10, offset=0 → 10 items, total=25
	results, total, err := service.List(context.Background(), templateDomain.Filter{
		AppID: "app-001", Limit: 10, Offset: 0,
	})
	require.NoError(t, err)
	assert.Len(t, results, 10)
	assert.Equal(t, int64(25), total, "total must reflect all matching templates, not page size")

	// Page 2: limit=10, offset=10 → 10 items, total still 25
	results, total, err = service.List(context.Background(), templateDomain.Filter{
		AppID: "app-001", Limit: 10, Offset: 10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 10)
	assert.Equal(t, int64(25), total)

	// Page 3: limit=10, offset=20 → 5 items, total still 25
	results, total, err = service.List(context.Background(), templateDomain.Filter{
		AppID: "app-001", Limit: 10, Offset: 20,
	})
	require.NoError(t, err)
	assert.Len(t, results, 5)
	assert.Equal(t, int64(25), total)
}

func TestTemplateService_List_TotalUnaffectedByOffset(t *testing.T) {
	repo := &mockTemplateRepo{}
	logger := zap.NewNop()
	service := NewTemplateService(repo, logger)

	seedTemplates(repo, "app-001", 30)

	// Regardless of offset, total should always be 30
	offsets := []int{0, 5, 10, 20, 29}
	for _, offset := range offsets {
		_, total, err := service.List(context.Background(), templateDomain.Filter{
			AppID: "app-001", Limit: 5, Offset: offset,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(30), total, "offset=%d should not affect total", offset)
	}
}

func TestTemplateService_List_FilteredTotal(t *testing.T) {
	repo := &mockTemplateRepo{}
	logger := zap.NewNop()
	service := NewTemplateService(repo, logger)

	// 25 templates: round-robin across 4 channels → ~6-7 email
	seedTemplates(repo, "app-001", 25)

	// Filter by channel=email
	results, total, err := service.List(context.Background(), templateDomain.Filter{
		AppID: "app-001", Channel: "email", Limit: 100, Offset: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(len(results)), total, "total should match filtered count when all fit in one page")
	for _, r := range results {
		assert.Equal(t, "email", r.Channel)
	}
}

func TestTemplateService_List_EmptyResult(t *testing.T) {
	repo := &mockTemplateRepo{}
	logger := zap.NewNop()
	service := NewTemplateService(repo, logger)

	results, total, err := service.List(context.Background(), templateDomain.Filter{
		AppID: "nonexistent", Limit: 10, Offset: 0,
	})
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.Equal(t, int64(0), total)
}

func TestTemplateService_List_OffsetBeyondTotal(t *testing.T) {
	repo := &mockTemplateRepo{}
	logger := zap.NewNop()
	service := NewTemplateService(repo, logger)

	seedTemplates(repo, "app-001", 5)

	results, total, err := service.List(context.Background(), templateDomain.Filter{
		AppID: "app-001", Limit: 10, Offset: 100,
	})
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.Equal(t, int64(5), total, "total should still report all matching templates even when offset exceeds count")
}

// ─── NormalizeTemplateBody Tests ─────────────────────────────────────

func TestNormalizeTemplateBody_BareVarBecomeDotted(t *testing.T) {
	input := "Hello {{name}}, you have {{count}} items"
	expected := "Hello {{.name}}, you have {{.count}} items"
	assert.Equal(t, expected, NormalizeTemplateBody(input))
}

func TestNormalizeTemplateBody_AlreadyDotted_Unchanged(t *testing.T) {
	input := "Hello {{.name}}, you have {{.count}} items"
	assert.Equal(t, input, NormalizeTemplateBody(input))
}

func TestNormalizeTemplateBody_MixedSyntax(t *testing.T) {
	input := "Hello {{.name}}, you have {{count}} items"
	expected := "Hello {{.name}}, you have {{.count}} items"
	assert.Equal(t, expected, NormalizeTemplateBody(input))
}

func TestNormalizeTemplateBody_KeywordsPreserved(t *testing.T) {
	input := "{{if .name}}Hello {{.name}}{{end}}"
	assert.Equal(t, input, NormalizeTemplateBody(input))
}

func TestNormalizeTemplateBody_BareKeywordsPreserved(t *testing.T) {
	// Bare keywords without dot should NOT get dot-prefixed.
	input := "{{if .show}}{{.name}}{{else}}hidden{{end}}"
	assert.Equal(t, input, NormalizeTemplateBody(input))
}

func TestNormalizeTemplateBody_WhitespaceVariants(t *testing.T) {
	input := "{{ name }} and {{  count  }}"
	expected := "{{ .name }} and {{  .count  }}"
	assert.Equal(t, expected, NormalizeTemplateBody(input))
}

func TestNormalizeTemplateBody_EmptyBody(t *testing.T) {
	assert.Equal(t, "", NormalizeTemplateBody(""))
}

func TestNormalizeTemplateBody_NoTemplateVars(t *testing.T) {
	input := "<html><body>Hello World</body></html>"
	assert.Equal(t, input, NormalizeTemplateBody(input))
}

func TestNormalizeTemplateBody_ComplexHTMLTemplate(t *testing.T) {
	input := `<div>{{company_name}}</div><img src="{{.logo_url}}">`
	expected := `<div>{{.company_name}}</div><img src="{{.logo_url}}">`
	assert.Equal(t, expected, NormalizeTemplateBody(input))
}
