package contracts

import "encoding/json"

// APICase ties published JSON and curl examples to actual HTTP handlers.
type APICase struct {
	ID       string            `json:"id"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Auth     string            `json:"auth"`
	Guide    string            `json:"guide"`
	Headers  map[string]string `json:"headers,omitempty"`
	Capture  map[string]string `json:"capture,omitempty"`
	Request  json.RawMessage   `json:"request,omitempty"`
	Status   int               `json:"status"`
	Response json.RawMessage   `json:"response"`
}
