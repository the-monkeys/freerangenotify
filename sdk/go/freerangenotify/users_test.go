package freerangenotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsersClient_UpdateByExternalID_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/v1/users/by-external-id/ext-42", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var body UpdateUserParams
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "New Name", body.FullName)
		assert.Equal(t, "new@example.com", body.Email)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(User{
			UserID:     "uid-1",
			ExternalID: "ext-42",
			FullName:   "New Name",
			Email:      "new@example.com",
		})
	}))
	defer ts.Close()

	client := New("test-key", WithBaseURL(ts.URL+"/v1"))
	got, err := client.Users.UpdateByExternalID(context.Background(), "ext-42", UpdateUserParams{
		FullName: "New Name",
		Email:    "new@example.com",
	})

	require.NoError(t, err)
	assert.Equal(t, "uid-1", got.UserID)
	assert.Equal(t, "ext-42", got.ExternalID)
	assert.Equal(t, "New Name", got.FullName)
	assert.Equal(t, "new@example.com", got.Email)
}

func TestUsersClient_UpdateByExternalID_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/users/by-external-id/nonexistent", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"success":false,"error":{"code":"NOT_FOUND","message":"User not found: nonexistent"}}`))
	}))
	defer ts.Close()

	client := New("test-key", WithBaseURL(ts.URL+"/v1"))
	got, err := client.Users.UpdateByExternalID(context.Background(), "nonexistent", UpdateUserParams{
		FullName: "Should fail",
	})

	assert.Nil(t, got)
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok, "error should be *APIError")
	assert.True(t, apiErr.IsNotFound())
}

func TestUsersClient_UpdateByExternalID_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer ts.Close()

	client := New("test-key", WithBaseURL(ts.URL+"/v1"))
	got, err := client.Users.UpdateByExternalID(context.Background(), "ext-42", UpdateUserParams{})

	assert.Nil(t, got)
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 500, apiErr.StatusCode)
}

func TestUsersClient_UpdateByExternalID_PartialParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		// Only phone should be present (omitempty on other zero-value fields).
		_, hasFullName := body["full_name"]
		assert.False(t, hasFullName, "full_name should be omitted when empty")
		assert.Equal(t, "+1234567890", body["phone"])

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(User{UserID: "uid-1", Phone: "+1234567890"})
	}))
	defer ts.Close()

	client := New("test-key", WithBaseURL(ts.URL+"/v1"))
	got, err := client.Users.UpdateByExternalID(context.Background(), "ext-42", UpdateUserParams{
		Phone: "+1234567890",
	})

	require.NoError(t, err)
	assert.Equal(t, "+1234567890", got.Phone)
}
