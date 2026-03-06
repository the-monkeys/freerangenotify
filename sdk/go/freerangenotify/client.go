// Package freerangenotify provides a Go client for the FreeRangeNotify notification service.
//
// Usage:
//
//	client := freerangenotify.New("your-api-key", freerangenotify.WithBaseURL("http://localhost:8080/v1"))
//	result, err := client.Send(ctx, freerangenotify.SendParams{
//	    To:       "user@example.com",
//	    Template: "welcome_email",
//	    Data:     map[string]interface{}{"name": "Alice"},
//	})
package freerangenotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client communicates with the FreeRangeNotify API.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// Option configures the Client.
type Option func(*Client)

// WithBaseURL sets a custom API base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithHTTPClient provides a custom http.Client (e.g. for timeouts or transport).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.http = hc }
}

// New creates a FreeRangeNotify client with the given API key.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: "http://localhost:8080/v1",
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SendParams holds the parameters for sending a notification via Quick-Send.
type SendParams struct {
	To          string                 `json:"to"`
	Template    string                 `json:"template,omitempty"`
	Subject     string                 `json:"subject,omitempty"`
	Body        string                 `json:"body,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Channel     string                 `json:"channel,omitempty"`
	Priority    string                 `json:"priority,omitempty"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
}

// SendResult is the response from a send or quick-send operation.
type SendResult struct {
	NotificationID string `json:"notification_id"`
	Status         string `json:"status"`
	UserID         string `json:"user_id"`
	Channel        string `json:"channel"`
}

// BroadcastParams holds the parameters for broadcasting a notification.
type BroadcastParams struct {
	Template string                 `json:"template_id"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Channel  string                 `json:"channel,omitempty"`
	Priority string                 `json:"priority,omitempty"`
}

// BroadcastResult is the response from a broadcast operation.
type BroadcastResult struct {
	TotalSent     int          `json:"total_sent"`
	Notifications []SendResult `json:"notifications"`
}

// CreateUserParams holds the parameters for creating a user.
type CreateUserParams struct {
	Email      string `json:"email,omitempty"`
	Phone      string `json:"phone,omitempty"`
	Timezone   string `json:"timezone,omitempty"`
	Language   string `json:"language,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
}

// UpdateUserParams holds the parameters for updating a user.
type UpdateUserParams struct {
	ExternalID string `json:"external_id,omitempty"`
	Email      string `json:"email,omitempty"`
	Phone      string `json:"phone,omitempty"`
	Timezone   string `json:"timezone,omitempty"`
	Language   string `json:"language,omitempty"`
	WebhookURL string `json:"webhook_url,omitempty"`
}

// User is a FreeRangeNotify user profile.
type User struct {
	UserID     string `json:"user_id"`
	ExternalID string `json:"external_id"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Timezone   string `json:"timezone"`
	Language   string `json:"language"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// Send delivers a notification to a single recipient via Quick-Send.
func (c *Client) Send(ctx context.Context, params SendParams) (*SendResult, error) {
	var result SendResult
	if err := c.do(ctx, http.MethodPost, "/quick-send", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Broadcast sends a notification to all users in the application.
func (c *Client) Broadcast(ctx context.Context, params BroadcastParams) (*BroadcastResult, error) {
	var result BroadcastResult
	if err := c.do(ctx, http.MethodPost, "/notifications/broadcast", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateUser registers a new user for notification targeting.
func (c *Client) CreateUser(ctx context.Context, params CreateUserParams) (*User, error) {
	var user User
	if err := c.do(ctx, http.MethodPost, "/users/", params, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUser updates an existing user (e.g. to change external_id after a username change).
func (c *Client) UpdateUser(ctx context.Context, userID string, params UpdateUserParams) (*User, error) {
	var user User
	if err := c.do(ctx, http.MethodPut, "/users/"+userID, params, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// ── Internal ────────────────────────────────────────────────

// APIError represents an error response from the FreeRangeNotify API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("freerangenotify: API error %d: %s", e.StatusCode, e.Body)
}

func (c *Client) do(ctx context.Context, method, path string, payload interface{}, out interface{}) error {
	var body io.Reader
	if payload != nil && method != http.MethodGet {
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
