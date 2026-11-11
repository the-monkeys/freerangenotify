package contracts

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	documentation "github.com/the-monkeys/freerangenotify/internal/documentation"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDocumentedRoutesMatchRouter(t *testing.T) {
	raw, e := os.ReadFile("../../docs/swagger.json")
	require.NoError(t, e)
	var doc struct {
		Paths map[string]map[string]interface{}
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	raw, e = os.ReadFile("../../examples/docs/internal-routes.json")
	require.NoError(t, e)
	var exclusions map[string]string
	require.NoError(t, json.Unmarshal(raw, &exclusions))
	actual := map[string]bool{}
	for _, key := range documentation.AllRouteInventory() {
		actual[key] = true
		if reason := exclusions[key]; reason != "" {
			require.NotContains(t, key, "*")
			continue
		}
		parts := strings.SplitN(key, " ", 2)
		require.Contains(t, doc.Paths[parts[1]], strings.ToLower(parts[0]), key)
	}
	for path, ops := range doc.Paths {
		for method := range ops {
			if !strings.HasPrefix(method, "x-") {
				require.True(t, actual[strings.ToUpper(method)+" "+path], "documented nonexistent route %s %s", method, path)
			}
		}
	}
	for key, reason := range exclusions {
		require.NotEmpty(t, reason)
		require.True(t, actual[key], "stale exclusion %s", key)
	}
	enabled := documentation.RouteInventory(true)
	disabled := documentation.RouteInventory(false)
	require.Greater(t, len(enabled), len(disabled))
	require.Contains(t, disabled, "GET /v1/sse")
	require.NotContains(t, disabled, "POST /v1/otp/send")
}

func TestPublicGeneratedSpecification(t *testing.T) {
	app := documentation.NewInventoryRouter(true)
	defer app.Shutdown()
	response, e := app.Test(httptest.NewRequest("GET", "/v1/swagger/doc.json", nil))
	require.NoError(t, e)
	defer response.Body.Close()
	require.Equal(t, 200, response.StatusCode)
	raw, e := io.ReadAll(response.Body)
	require.NoError(t, e)
	expected, e := os.ReadFile("../../docs/swagger.json")
	require.NoError(t, e)
	require.JSONEq(t, string(expected), string(raw))
}
