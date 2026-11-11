package docgen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Generate produces all public reference documents from pinned Swag annotations.
// Check mode writes only to an owned temporary directory and fails on drift.
func Generate(root string, check bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	if !bytes.Contains(mod, []byte("github.com/swaggo/swag v1.16.6")) {
		return fmt.Errorf("docsgen requires swag v1.16.6 pinned in go.mod")
	}
	temp, err := os.MkdirTemp("", "frn-docsgen-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	cmd := exec.Command("go", "run", "github.com/swaggo/swag/cmd/swag", "init", "-g", "main.go", "-d", "cmd/server,internal,pkg", "-o", temp, "--parseInternal", "--generatedTime=false", "--packageName", "docs")
	cmd.Dir = root
	if output, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("swag: %w: %s", e, output)
	}
	raw, err := os.ReadFile(filepath.Join(temp, "swagger.json"))
	if err != nil {
		return err
	}
	var doc map[string]interface{}
	if err = json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	// Deployment is selected by the API/UI base URL, never a baked-in localhost host.
	delete(doc, "host")
	delete(doc, "schemes")
	paths := doc["paths"].(map[string]interface{})
	// Explicitly excluded operator/legacy surfaces remain checked by route coverage.
	exclusions, err := readExclusions(root)
	if err != nil {
		return err
	}
	disabled := map[string]bool{}
	for _, key := range RouteInventory(false) {
		disabled[key] = true
	}
	for _, key := range AllRouteInventory() {
		parts := strings.SplitN(key, " ", 2)
		path, method := parts[1], strings.ToLower(parts[0])
		if exclusions[key] != "" {
			continue
		}
		ops, _ := paths[path].(map[string]interface{})
		if ops == nil {
			ops = map[string]interface{}{}
			paths[path] = ops
		}
		if ops[method] == nil {
			return fmt.Errorf("missing annotation: %s", key)
		}
		if !disabled[key] {
			op := ops[method].(map[string]interface{})
			op["x-feature-prerequisites"] = "Requires this deployment's optional " + strings.Split(strings.TrimPrefix(path, "/v1/"), "/")[0] + " feature and its configured service/provider."
		}

	}
	for path, value := range paths {
		ops := value.(map[string]interface{})
		for method := range ops {
			if exclusions[strings.ToUpper(method)+" "+path] != "" {
				delete(ops, method)
			}
		}
		if len(ops) == 0 {
			delete(paths, path)
		}
	}
	if err = applyExamples(root, doc); err != nil {
		return err
	}
	raw, err = json.MarshalIndent(doc, "", "    ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	var v2 openapi2.T
	if err = json.Unmarshal(raw, &v2); err != nil {
		return err
	}
	v3, err := openapi2conv.ToV3(&v2)
	if err != nil {
		return fmt.Errorf("convert: %w", err)
	}
	// Validate local refs only; no reference may fetch arbitrary network resources.
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	v3raw, _ := json.Marshal(v3)
	v3, err = loader.LoadFromData(v3raw)
	if err != nil {
		return err
	}
	if err = v3.Validate(context.Background()); err != nil {
		return fmt.Errorf("validate OpenAPI: %w", err)
	}
	swaggerYAML, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	outputs := map[string][]byte{"docs/swagger.json": raw, "ui/public/swagger.json": raw, "docs/swagger.yaml": swaggerYAML}
	// docs.go remains Swag's compatible registration format, with the enriched spec.
	generated, err := os.ReadFile(filepath.Join(temp, "docs.go"))
	if err != nil {
		return err
	}
	start := bytes.Index(generated, []byte("const docTemplate = `"))
	if start < 0 {
		return fmt.Errorf("unexpected Swag template format")
	}
	end := bytes.Index(generated[start+len("const docTemplate = `"):], []byte("`"))
	if start < 0 || end < 0 {
		return fmt.Errorf("unexpected Swag template format")
	}
	a := start + len("const docTemplate = `")
	b := a + end
	generated = append(append(append([]byte{}, generated[:a]...), bytes.ReplaceAll(raw, []byte("`"), []byte("` + \"`\" + `"))...), generated[b:]...)
	generated = bytes.ReplaceAll(generated, []byte(`LeftDelim:        "{{"`), []byte(`LeftDelim:        "<<FRN"`))
	generated = bytes.ReplaceAll(generated, []byte(`RightDelim:       "}}"`), []byte(`RightDelim:       "FRN>>"`))
	outputs["docs/docs.go"] = generated
	for name, filter := range map[string]string{"swagger": "", "otp": "/v1/otp/", "whatsapp-rich": "/v1/whatsapp/"} {
		data, err := json.Marshal(v3)
		if err != nil {
			return err
		}
		var view map[string]interface{}
		json.Unmarshal(data, &view)
		if filter != "" {
			pv := view["paths"].(map[string]interface{})
			for path := range pv {
				if !strings.HasPrefix(path, filter) {
					delete(pv, path)
				}
			}
		}
		y, err := yaml.Marshal(view)
		if err != nil {
			return err
		}
		outputs["docs/openapi/"+name+".yaml"] = y
	}
	for name, data := range outputs {
		dest := filepath.Join(root, filepath.FromSlash(name))
		if check {
			current, e := os.ReadFile(dest)
			if e != nil || !bytes.Equal(current, data) {
				return fmt.Errorf("generated reference drift: %s; run go run ./cmd/docsgen", name)
			}
		} else {
			if e := os.MkdirAll(filepath.Dir(dest), 0755); e != nil {
				return e
			}
			if e := os.WriteFile(dest, data, 0644); e != nil {
				return e
			}
		}
	}
	return nil
}
func readExclusions(root string) (map[string]string, error) {
	raw, e := os.ReadFile(filepath.Join(root, "examples/docs/internal-routes.json"))
	if e != nil {
		return nil, e
	}
	var m map[string]string
	e = json.Unmarshal(raw, &m)
	for key, reason := range m {
		if strings.Contains(key, "*") || strings.TrimSpace(reason) == "" {
			return nil, fmt.Errorf("invalid explicit exclusion %q", key)
		}
	}
	return m, e
}
func applyExamples(root string, doc map[string]interface{}) error {
	raw, e := os.ReadFile(filepath.Join(root, "examples/docs/api-cases.json"))
	if e != nil {
		return e
	}
	var cases []struct {
		Method, Path      string
		Request, Response interface{}
	}
	if e = json.Unmarshal(raw, &cases); e != nil {
		return e
	}
	paths := doc["paths"].(map[string]interface{})
	for _, c := range cases {
		path := "/v1" + c.Path
		for key := range paths {
			if key == path {
				ops := paths[key].(map[string]interface{})
				if op, ok := ops[strings.ToLower(c.Method)].(map[string]interface{}); ok {
					if _, exists := op["x-contract-example"]; !exists {
						op["x-contract-example"] = map[string]interface{}{"request": c.Request, "response": c.Response}
					}
				}
			}
		}
	}
	return nil
}
