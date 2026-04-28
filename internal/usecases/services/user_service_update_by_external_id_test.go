package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// ─── Mock user repository for service tests ────────────────────────────────

type stubUserRepo struct {
	users     map[string]*user.User        // keyed by user_id
	byExtID   map[string]map[string]string // app_id → external_id → user_id
	updateErr error
}

func newStubUserRepo() *stubUserRepo {
	return &stubUserRepo{
		users:   make(map[string]*user.User),
		byExtID: make(map[string]map[string]string),
	}
}

func (r *stubUserRepo) seed(u *user.User) {
	r.users[u.UserID] = u
	if u.ExternalID != "" {
		if r.byExtID[u.AppID] == nil {
			r.byExtID[u.AppID] = make(map[string]string)
		}
		r.byExtID[u.AppID][u.ExternalID] = u.UserID
	}
}

func (r *stubUserRepo) Create(_ context.Context, u *user.User) error               { return nil }
func (r *stubUserRepo) Delete(_ context.Context, _ string) error                   { return nil }
func (r *stubUserRepo) AddDevice(_ context.Context, _ string, _ user.Device) error { return nil }
func (r *stubUserRepo) RemoveDevice(_ context.Context, _, _ string) error          { return nil }
func (r *stubUserRepo) UpdatePreferences(_ context.Context, _ string, _ user.Preferences) error {
	return nil
}
func (r *stubUserRepo) Count(_ context.Context, _ user.UserFilter) (int64, error) { return 0, nil }
func (r *stubUserRepo) BulkCreate(_ context.Context, _ []*user.User) error        { return nil }
func (r *stubUserRepo) List(_ context.Context, _ user.UserFilter) ([]*user.User, error) {
	return nil, nil
}

func (r *stubUserRepo) GetByID(_ context.Context, id string) (*user.User, error) {
	if u, ok := r.users[id]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *stubUserRepo) GetByExternalID(_ context.Context, appID, externalID string) (*user.User, error) {
	if m, ok := r.byExtID[appID]; ok {
		if uid, ok := m[externalID]; ok {
			return r.users[uid], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (r *stubUserRepo) GetByEmail(_ context.Context, _, _ string) (*user.User, error) {
	return nil, fmt.Errorf("not found")
}

func (r *stubUserRepo) Update(_ context.Context, u *user.User) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.users[u.UserID] = u
	return nil
}

// ─── Tests ─────────────────────────────────────────────────────────────────

func TestUpdateByExternalID_Success(t *testing.T) {
	repo := newStubUserRepo()
	repo.seed(&user.User{
		UserID:     "uid-1",
		AppID:      "app-1",
		ExternalID: "ext-42",
		FullName:   "Original Name",
		Email:      "old@example.com",
	})

	svc := NewUserService(repo, zap.NewNop())

	got, err := svc.UpdateByExternalID(context.Background(), "app-1", "ext-42", func(u *user.User) {
		u.FullName = "Updated Name"
		u.Email = "new@example.com"
	})

	require.NoError(t, err)
	assert.Equal(t, "uid-1", got.UserID)
	assert.Equal(t, "Updated Name", got.FullName)
	assert.Equal(t, "new@example.com", got.Email)
	assert.False(t, got.UpdatedAt.IsZero(), "UpdatedAt should be set")

	// Verify persisted in repo
	persisted := repo.users["uid-1"]
	assert.Equal(t, "Updated Name", persisted.FullName)
}

func TestUpdateByExternalID_NotFound(t *testing.T) {
	repo := newStubUserRepo()
	svc := NewUserService(repo, zap.NewNop())

	got, err := svc.UpdateByExternalID(context.Background(), "app-1", "nonexistent", func(u *user.User) {
		u.FullName = "Should not happen"
	})

	assert.Nil(t, got)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateByExternalID_EmptyAppID(t *testing.T) {
	repo := newStubUserRepo()
	svc := NewUserService(repo, zap.NewNop())

	got, err := svc.UpdateByExternalID(context.Background(), "", "ext-42", func(u *user.User) {})

	assert.Nil(t, got)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestUpdateByExternalID_EmptyExternalID(t *testing.T) {
	repo := newStubUserRepo()
	svc := NewUserService(repo, zap.NewNop())

	got, err := svc.UpdateByExternalID(context.Background(), "app-1", "", func(u *user.User) {})

	assert.Nil(t, got)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestUpdateByExternalID_RepoUpdateError(t *testing.T) {
	repo := newStubUserRepo()
	repo.seed(&user.User{
		UserID:     "uid-1",
		AppID:      "app-1",
		ExternalID: "ext-42",
	})
	repo.updateErr = fmt.Errorf("ES write failure")

	svc := NewUserService(repo, zap.NewNop())

	got, err := svc.UpdateByExternalID(context.Background(), "app-1", "ext-42", func(u *user.User) {
		u.FullName = "New"
	})

	assert.Nil(t, got)
	assert.Error(t, err)
}

func TestUpdateByExternalID_CrossAppIsolation(t *testing.T) {
	repo := newStubUserRepo()
	repo.seed(&user.User{
		UserID:     "uid-1",
		AppID:      "app-1",
		ExternalID: "ext-42",
	})

	svc := NewUserService(repo, zap.NewNop())

	// app-2 should NOT see app-1's user
	got, err := svc.UpdateByExternalID(context.Background(), "app-2", "ext-42", func(u *user.User) {
		u.FullName = "Hacked"
	})

	assert.Nil(t, got)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Original should remain unchanged
	assert.Equal(t, "", repo.users["uid-1"].FullName)
}
