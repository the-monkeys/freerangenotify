package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

func TestBuildUserQuery_AppIDOnly(t *testing.T) {
	repo := &UserRepository{}
	filter := user.UserFilter{AppID: "app-1"}
	q := repo.buildUserQuery(filter)

	query := q["query"].(map[string]interface{})
	boolQ := query["bool"].(map[string]interface{})
	must := boolQ["must"].([]map[string]interface{})

	assert.Len(t, must, 1)
	term := must[0]["term"].(map[string]interface{})
	assert.Equal(t, "app-1", term["app_id"])
}

func TestBuildUserQuery_LinkedUserIDs(t *testing.T) {
	repo := &UserRepository{}
	filter := user.UserFilter{
		AppID:         "target-app",
		LinkedUserIDs: []string{"user-1", "user-2", "user-3"},
	}
	q := repo.buildUserQuery(filter)

	query := q["query"].(map[string]interface{})
	boolQ := query["bool"].(map[string]interface{})
	must := boolQ["must"].([]map[string]interface{})

	// First (and only) must clause should be the bool.should for app_id OR linked user_ids
	assert.Len(t, must, 1)

	innerBool := must[0]["bool"].(map[string]interface{})
	should := innerBool["should"].([]map[string]interface{})
	assert.Len(t, should, 2)

	// First should clause: term filter for the app's own users
	appTerm := should[0]["term"].(map[string]interface{})
	assert.Equal(t, "target-app", appTerm["app_id"])

	// Second should clause: terms filter for linked user IDs
	linkedTerms := should[1]["terms"].(map[string]interface{})
	ids := linkedTerms["user_id"].([]string)
	assert.ElementsMatch(t, []string{"user-1", "user-2", "user-3"}, ids)

	// minimum_should_match must be 1
	assert.Equal(t, 1, innerBool["minimum_should_match"])
}

func TestBuildUserQuery_LinkedUserIDs_WithOtherFilters(t *testing.T) {
	repo := &UserRepository{}
	filter := user.UserFilter{
		AppID:         "target-app",
		LinkedUserIDs: []string{"user-1"},
		Email:         "test@example.com",
		Timezone:      "UTC",
	}
	q := repo.buildUserQuery(filter)

	query := q["query"].(map[string]interface{})
	boolQ := query["bool"].(map[string]interface{})
	must := boolQ["must"].([]map[string]interface{})

	// Should have 3 must clauses: the bool.should + email + timezone
	assert.Len(t, must, 3)

	// First is the bool.should for linked IDs
	innerBool := must[0]["bool"].(map[string]interface{})
	assert.NotNil(t, innerBool["should"])

	// Email filter
	emailTerm := must[1]["term"].(map[string]interface{})
	assert.Equal(t, "test@example.com", emailTerm["email"])

	// Timezone filter
	tzTerm := must[2]["term"].(map[string]interface{})
	assert.Equal(t, "UTC", tzTerm["timezone"])
}

func TestBuildUserQuery_EmptyLinkedUserIDs_FallsBackToAppID(t *testing.T) {
	repo := &UserRepository{}
	filter := user.UserFilter{
		AppID:         "my-app",
		LinkedUserIDs: []string{}, // empty
	}
	q := repo.buildUserQuery(filter)

	query := q["query"].(map[string]interface{})
	boolQ := query["bool"].(map[string]interface{})
	must := boolQ["must"].([]map[string]interface{})

	// With empty LinkedUserIDs, should fall back to simple term filter
	assert.Len(t, must, 1)
	term := must[0]["term"].(map[string]interface{})
	assert.Equal(t, "my-app", term["app_id"])
}

func TestBuildUserQuery_Pagination(t *testing.T) {
	repo := &UserRepository{}
	filter := user.UserFilter{
		AppID:  "app-1",
		Limit:  25,
		Offset: 50,
	}
	q := repo.buildUserQuery(filter)

	assert.Equal(t, 50, q["from"])
	assert.Equal(t, 25, q["size"])
}

func TestBuildUserQuery_NoFilters(t *testing.T) {
	repo := &UserRepository{}
	filter := user.UserFilter{}
	q := repo.buildUserQuery(filter)

	// With no filters, should be match_all
	query := q["query"].(map[string]interface{})
	_, hasMatchAll := query["match_all"]
	assert.True(t, hasMatchAll, "empty filter should produce match_all query")
}
