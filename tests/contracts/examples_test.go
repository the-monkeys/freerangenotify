package contracts

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func loadCases(t *testing.T) []APICase {
	t.Helper()
	raw, err := os.ReadFile("../../examples/docs/api-cases.json")
	require.NoError(t, err)
	var cases []APICase
	require.NoError(t, json.Unmarshal(raw, &cases))
	return cases
}
func substitute(s string, vars map[string]string) string {
	for key, value := range vars {
		s = strings.ReplaceAll(s, "${"+key+"}", value)
	}
	return s
}
func atPath(t *testing.T, value interface{}, path string) interface{} {
	for _, part := range strings.Split(path, ".") {
		m, ok := value.(map[string]interface{})
		require.True(t, ok, "path %s", path)
		value = m[part]
	}
	return value
}
func assertSubset(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if m, ok := expected.(map[string]interface{}); ok {
		a, ok := actual.(map[string]interface{})
		require.True(t, ok)
		for key, value := range m {
			require.Contains(t, a, key)
			assertSubset(t, value, a[key])
		}
		return
	}
	if list, ok := expected.([]interface{}); ok {
		a, ok := actual.([]interface{})
		require.True(t, ok)
		require.Len(t, a, len(list))
		for i, value := range list {
			assertSubset(t, value, a[i])
		}
		return
	}
	require.Equal(t, expected, actual)
}
func exampleBlock(t *testing.T, guide, id, kind string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../../ui/src/docs", guide+".md"))
	require.NoError(t, err)
	pattern := regexp.MustCompile("(?s)<!-- api-example:" + regexp.QuoteMeta(id) + ":" + kind + " -->\\s*```[^\\n]*\\n(.*?)\\n```")
	match := pattern.FindStringSubmatch(strings.ReplaceAll(string(raw), "\r\n", "\n"))
	require.Len(t, match, 2, "missing %s:%s in %s", id, kind, guide)
	return match[1]
}
func runExamples(t *testing.T, cases []APICase) {
	app := newExampleRouter(t)
	defer app.Shutdown()
	vars := map[string]string{"FILE_ID": "file_saved_report"}
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			request := substitute(string(c.Request), vars)
			req := httptest.NewRequest(c.Method, substitute("/v1"+c.Path, vars), strings.NewReader(request))
			req.Header.Set("Content-Type", "application/json")
			switch c.Auth {
			case "jwt":
				req.Header.Set("Authorization", "Bearer fixture-dashboard-jwt")
			case "api-key":
				req.Header.Set("Authorization", "Bearer "+vars["API_KEY"])
			case "x-api-key":
				req.Header.Set("X-API-Key", vars["API_KEY"])
			}
			for k, v := range c.Headers {
				req.Header.Set(k, substitute(v, vars))
			}
			res, err := app.Test(req, 10000)
			require.NoError(t, err)
			raw, err := io.ReadAll(res.Body)
			res.Body.Close()
			require.NoError(t, err)
			require.Equal(t, c.Status, res.StatusCode, string(raw))
			var actual interface{}
			require.NoError(t, json.Unmarshal(raw, &actual))
			for key, path := range c.Capture {
				value, ok := atPath(t, actual, path).(string)
				require.True(t, ok)
				require.NotEmpty(t, value)
				if strings.HasSuffix(key, "_ID") {
					if strings.HasPrefix(value, "otp_") {
						require.Regexp(t, `^otp_[a-f0-9]{32}$`, value)
					} else {
						require.NoError(t, uuid.Validate(value))
					}
				}
				vars[key] = value
			}
			var expected interface{}
			require.NoError(t, json.Unmarshal([]byte(substitute(string(c.Response), vars)), &expected))
			assertSubset(t, expected, actual)
			for kind, payload := range map[string]json.RawMessage{"request": c.Request, "response": c.Response} {
				if len(payload) == 0 {
					continue
				}
				var published, fixture interface{}
				require.NoError(t, json.Unmarshal([]byte(exampleBlock(t, c.Guide, c.ID, kind)), &published))
				require.NoError(t, json.Unmarshal(payload, &fixture))
				require.Equal(t, fixture, published)
			}
			curl := exampleBlock(t, c.Guide, c.ID, "curl")
			require.Contains(t, curl, "-X "+c.Method)
			require.Contains(t, curl, `"$BASE_URL`+c.Path+`"`)
			auth := "Authorization: Bearer $API_KEY"
			if c.Auth == "jwt" {
				auth = "Authorization: Bearer $DASHBOARD_JWT"
			}
			if c.Auth == "x-api-key" {
				auth = "X-API-Key: $API_KEY"
			}
			require.Contains(t, curl, auth)
			if len(c.Request) > 0 {
				body := strings.Split(curl, "<<JSON\n")
				require.Len(t, body, 2)
				body[1] = strings.TrimSuffix(body[1], "\nJSON")
				var a, b interface{}
				require.NoError(t, json.Unmarshal([]byte(body[1]), &a))
				require.NoError(t, json.Unmarshal(c.Request, &b))
				require.Equal(t, b, a)
			}
		})
	}
}
func TestDocumentationOnboardingContracts(t *testing.T) {
	cases := []APICase{}
	for _, c := range loadCases(t) {
		if c.Guide == "getting-started" || c.Guide == "users" {
			cases = append(cases, c)
		}
	}
	runExamples(t, cases)
}

func TestDocumentationLifecycleContracts(t *testing.T) {
	cases := []APICase{}
	for _, c := range loadCases(t) {
		if c.Guide != "otp" && c.Guide != "webhooks" && c.Guide != "file-attachments" && c.Guide != "channels" {
			cases = append(cases, c)
		}
	}
	runExamples(t, cases)
}
