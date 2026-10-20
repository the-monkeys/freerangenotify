package docgen

import (
	"context"
	"encoding/json"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedSpecifications(t *testing.T) {
	root := filepath.Join("..", "..")
	require.NoError(t, Generate(root, true))
	a, e := os.ReadFile(filepath.Join(root, "docs/swagger.json"))
	require.NoError(t, e)
	b, e := os.ReadFile(filepath.Join(root, "ui/public/swagger.json"))
	require.NoError(t, e)
	require.Equal(t, a, b)
	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(a, &doc))
	require.Equal(t, "2.0", doc["swagger"])
	defs := doc["definitions"].(map[string]interface{})
	send := defs["dto.SendNotificationRequest"].(map[string]interface{})
	required, _ := send["required"].([]interface{})
	require.NotContains(t, required, "template_id")
	require.NotContains(t, required, "priority")
	schemes := doc["securityDefinitions"].(map[string]interface{})
	require.Contains(t, schemes, "ApiKeyAuth")
	require.Contains(t, schemes, "BearerAuth")
	paths := doc["paths"].(map[string]interface{})
	require.Contains(t, paths, "/v1/sse")
	require.Contains(t, paths, "/v1/sse/tokens")
	upload := paths["/v1/files"].(map[string]interface{})["post"].(map[string]interface{})
	require.Contains(t, upload["consumes"], "multipart/form-data")
}

func TestRetainedOpenAPIViews(t *testing.T) {
	for _, name := range []string{"swagger", "otp", "whatsapp-rich"} {
		t.Run(name, func(t *testing.T) {
			loader := openapi3.NewLoader()
			loader.IsExternalRefsAllowed = false
			doc, e := loader.LoadFromFile("../../docs/openapi/" + name + ".yaml")
			require.NoError(t, e)
			require.Equal(t, "3.0.3", doc.OpenAPI)
			require.NotZero(t, doc.Paths.Len())
			require.NoError(t, doc.Validate(context.Background()))
		})
	}
}

func TestRichAuthoringReference(t *testing.T) {
	raw, e := os.ReadFile("../../docs/swagger.json")
	require.NoError(t, e)
	var doc struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				In     string
				Schema map[string]interface{}
			}
		}
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	for _, path := range []string{"/v1/whatsapp/rich-templates", "/v1/whatsapp/rich-templates/drafts"} {
		found := false
		for _, param := range doc.Paths[path]["post"].Parameters {
			if param.In == "body" && param.Schema["$ref"] != nil {
				found = true
			}
		}
		require.True(t, found, "missing rich request schema %s", path)
	}
}
