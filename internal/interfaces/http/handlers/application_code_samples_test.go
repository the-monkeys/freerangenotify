package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

type codeSampleApps struct{ usecases.ApplicationService }

func (codeSampleApps) GetByID(context.Context, string) (*application.Application, error) {
	return &application.Application{AppID: "app-a", AdminUserID: "owner"}, nil
}

func integrationSamples(t *testing.T, base string) map[string]map[string]struct {
	Sender   struct{ Code string }
	Auth     *struct{ Code string }
	Receiver *struct{ Code string }
} {
	t.Helper()
	h := NewApplicationHandler(codeSampleApps{}, nil, nil, nil, validator.New(), zap.NewNop())
	require.NoError(t, h.SetCodeSampleBaseURL(base))
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user_id", "owner"); return c.Next() })
	app.Get("/:id/samples", h.GetCodeSamples)
	req := httptest.NewRequest("GET", "http://attacker.invalid/app-a/samples", nil)
	req.Header.Set("X-Forwarded-Host", "attacker.invalid")
	res, err := app.Test(req)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, 200, res.StatusCode)
	var out struct {
		Data map[string]map[string]struct {
			Sender   struct{ Code string }
			Auth     *struct{ Code string }
			Receiver *struct{ Code string }
		}
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	return out.Data
}
func TestIntegrationCodeSamples(t *testing.T) {
	samples := integrationSamples(t, "https://notify.example/prefix/v1/")
	for _, lang := range []string{"go", "python", "javascript", "java", "cpp", "rust", "ruby"} {
		for _, channel := range []string{"email", "sms", "whatsapp", "webhook", "sse"} {
			code := samples[lang][channel].Sender.Code
			require.NotEmpty(t, code)
			require.Contains(t, code, "YOUR_API_KEY")
			require.Contains(t, code, "https://notify.example/prefix/v1/notifications")
			require.NotContains(t, code, "/v1/v1")
			require.NotContains(t, code, "attacker.invalid")
		}
	}
	require.Contains(t, samples["javascript"]["sse"].Receiver.Code, "addEventListener('notification'")
	require.NotContains(t, samples["javascript"]["sse"].Receiver.Code, "onmessage")
	require.Contains(t, integrationSamples(t, "")["go"]["email"].Sender.Code, "YOUR_BASE_URL/v1")
	h := &ApplicationHandler{}
	for _, base := range []string{"javascript:alert(1)", "https://example.test/?x=1", "https://example.test/#x"} {
		require.Error(t, h.SetCodeSampleBaseURL(base))
	}
}
func TestIntegrationGoSamplesCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("generated snippet compilation runs explicitly outside short suite")
	}
	samples := integrationSamples(t, "https://notify.example/prefix")
	for channel, sample := range samples["go"] {
		t.Run(channel, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "main.go")
			require.NoError(t, os.WriteFile(path, []byte(sample.Sender.Code), 0600))
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, "go", "test", "-p", "2", path)
			cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, strings.TrimSpace(string(out)))
		})
	}
}
