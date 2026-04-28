package freerangenotify

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// UsersClient handles user operations.
type UsersClient struct {
	client *Client
}

// Create registers a new user for notification targeting.
func (u *UsersClient) Create(ctx context.Context, params CreateUserParams) (*User, error) {
	var result User
	if err := u.client.do(ctx, "POST", "/users/", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BulkCreate registers multiple users in a single request.
// Set SkipExisting to silently skip duplicates, or Upsert to update existing users.
func (u *UsersClient) BulkCreate(ctx context.Context, params BulkCreateUsersParams) (*BulkCreateUsersResult, error) {
	var result BulkCreateUsersResult
	if err := u.client.do(ctx, "POST", "/users/bulk", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Get retrieves a user by their internal UUID or external_id.
func (u *UsersClient) Get(ctx context.Context, identifier string) (*User, error) {
	var result User
	if err := u.client.do(ctx, "GET", "/users/"+identifier, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetByExternalID retrieves a user by their external_id.
func (u *UsersClient) GetByExternalID(ctx context.Context, externalID string) (*User, error) {
	var result User
	if err := u.client.do(ctx, "GET", "/users/by-external-id/"+externalID, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Update modifies an existing user's profile. Accepts internal UUID or external_id.
func (u *UsersClient) Update(ctx context.Context, identifier string, params UpdateUserParams) (*User, error) {
	var result User
	if err := u.client.do(ctx, "PUT", "/users/"+identifier, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateByExternalID modifies a user's profile directly by external_id without
// resolving to an internal UUID first.
func (u *UsersClient) UpdateByExternalID(ctx context.Context, externalID string, params UpdateUserParams) (*User, error) {
	var result User
	if err := u.client.do(ctx, "PUT", "/users/by-external-id/"+externalID, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete removes a user. Accepts internal UUID or external_id.
func (u *UsersClient) Delete(ctx context.Context, identifier string) error {
	return u.client.do(ctx, "DELETE", "/users/"+identifier, nil, nil)
}

// List returns a paginated list of users.
func (u *UsersClient) List(ctx context.Context, page, pageSize int) (*UserListResponse, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if pageSize > 0 {
		q.Set("page_size", strconv.Itoa(pageSize))
	}

	var result UserListResponse
	if err := u.client.doWithQuery(ctx, "GET", "/users/", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Devices ──

// AddDevice registers a push notification device token for a user.
func (u *UsersClient) AddDevice(ctx context.Context, userID string, params AddDeviceParams) (*Device, error) {
	var result Device
	if err := u.client.do(ctx, "POST", fmt.Sprintf("/users/%s/devices", userID), params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDevices retrieves all registered devices for a user.
func (u *UsersClient) GetDevices(ctx context.Context, userID string) ([]Device, error) {
	var result struct {
		Devices []Device `json:"devices"`
	}
	if err := u.client.do(ctx, "GET", fmt.Sprintf("/users/%s/devices", userID), nil, &result); err != nil {
		return nil, err
	}
	return result.Devices, nil
}

// RemoveDevice removes a device registration.
func (u *UsersClient) RemoveDevice(ctx context.Context, userID, deviceID string) error {
	return u.client.do(ctx, "DELETE", fmt.Sprintf("/users/%s/devices/%s", userID, deviceID), nil, nil)
}

// ── Preferences ──

// GetPreferences retrieves a user's notification preferences.
func (u *UsersClient) GetPreferences(ctx context.Context, userID string) (*Preferences, error) {
	var result struct {
		Preferences *Preferences `json:"preferences"`
	}
	if err := u.client.do(ctx, "GET", fmt.Sprintf("/users/%s/preferences", userID), nil, &result); err != nil {
		return nil, err
	}
	return result.Preferences, nil
}

// UpdatePreferences updates a user's notification preferences.
func (u *UsersClient) UpdatePreferences(ctx context.Context, userID string, prefs Preferences) (*Preferences, error) {
	var result struct {
		Preferences *Preferences `json:"preferences"`
	}
	if err := u.client.do(ctx, "PUT", fmt.Sprintf("/users/%s/preferences", userID), prefs, &result); err != nil {
		return nil, err
	}
	return result.Preferences, nil
}

// ── Subscriber Hash ──

// GetSubscriberHash retrieves an HMAC subscriber hash for SSE authentication.
func (u *UsersClient) GetSubscriberHash(ctx context.Context, userID string) (string, error) {
	var result struct {
		SubscriberHash string `json:"subscriber_hash"`
	}
	if err := u.client.do(ctx, "GET", fmt.Sprintf("/users/%s/subscriber-hash", userID), nil, &result); err != nil {
		return "", err
	}
	return result.SubscriberHash, nil
}
