// Package freerangenotify provides a Go client for the FreeRangeNotify notification service.
//
// It uses a sub-client pattern for resource-oriented access:
//
//	client := freerangenotify.New("frn_xxx", freerangenotify.WithBaseURL("http://localhost:8080/v1"))
//
//	// Resource-oriented sub-clients
//	client.Notifications.Send(ctx, params)
//	client.Users.Create(ctx, params)
//	client.Templates.List(ctx, opts)
//	client.Workflows.Trigger(ctx, params)
//	client.Topics.AddSubscribers(ctx, topicID, userIDs)
//	client.Presence.CheckIn(ctx, params)
//
//	// Backward-compatible convenience methods
//	client.Send(ctx, params)       // delegates to client.Notifications.QuickSend
//	client.Broadcast(ctx, params)  // delegates to client.Notifications.Broadcast
package freerangenotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client communicates with the FreeRangeNotify API.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client

	// Sub-clients (resource-oriented)
	Notifications *NotificationsClient
	Users         *UsersClient
	Templates     *TemplatesClient
	Workflows     *WorkflowsClient
	Topics        *TopicsClient
	Presence      *PresenceClient
}

// Option configures the Client.
type Option func(*Client)

// WithBaseURL sets a custom API base URL.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient provides a custom http.Client (e.g. for transports or TLS).
func WithHTTPClient(hc *http.Client) Option { return func(c *Client) { c.http = hc } }

// WithEnvironment sets an informational environment label.
// The actual environment is determined server-side by the API key used.
// Use a per-environment API key (e.g., frn_dev_xxx, frn_stg_xxx, frn_prod_xxx)
// to scope all operations to that environment.
func WithEnvironment(_ string) Option { return func(_ *Client) {} }

// WithTimeout overrides the default HTTP request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}

// New creates a FreeRangeNotify client with the given API key.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: "http://localhost:8080/v1",
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	c.Notifications = &NotificationsClient{client: c}
	c.Users = &UsersClient{client: c}
	c.Templates = &TemplatesClient{client: c}
	c.Workflows = &WorkflowsClient{client: c}
	c.Topics = &TopicsClient{client: c}
	c.Presence = &PresenceClient{client: c}
	return c
}

// ── Backward-compatible convenience methods ──────────────────

// Send delivers a notification via Quick-Send (delegates to Notifications.QuickSend).
func (c *Client) Send(ctx context.Context, params SendParams) (*SendResult, error) {
	return c.Notifications.QuickSend(ctx, params)
}

// Broadcast sends a notification to all users (delegates to Notifications.Broadcast).
func (c *Client) Broadcast(ctx context.Context, params BroadcastParams) (*BroadcastResult, error) {
	return c.Notifications.Broadcast(ctx, params)
}

// CreateUser registers a new user (delegates to Users.Create).
func (c *Client) CreateUser(ctx context.Context, params CreateUserParams) (*User, error) {
	return c.Users.Create(ctx, params)
}

// UpdateUser updates an existing user (delegates to Users.Update).
func (c *Client) UpdateUser(ctx context.Context, userID string, params UpdateUserParams) (*User, error) {
	return c.Users.Update(ctx, userID, params)
}

// ── Internal HTTP Transport ─────────────────────────────────

func (c *Client) do(ctx context.Context, method, path string, payload interface{}, out interface{}) error {
	var body io.Reader
	if payload != nil && method != http.MethodGet && method != http.MethodDelete {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("freerangenotify: marshal request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("freerangenotify: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("freerangenotify: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("freerangenotify: decode response: %w", err)
		}
	}
	return nil
}

// doWithQuery is like do but appends query parameters to the path.
func (c *Client) doWithQuery(ctx context.Context, method, path string, query url.Values, out interface{}) error {
	if len(query) > 0 {
		path = path + "?" + query.Encode()
	}
	return c.do(ctx, method, path, nil, out)
}
