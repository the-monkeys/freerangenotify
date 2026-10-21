package freerangenotify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserEnvelopeCompatibility(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		t.Run(fmt.Sprint(wrapped), func(t *testing.T) {
			body := `{"user_id":"fixture-user","email":"alice@example.test"}`
			if wrapped {
				body = `{"success":true,"data":` + body + `}`
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, body)
			}))
			defer server.Close()
			client := New("fixture-key", WithBaseURL(server.URL))
			user, err := client.Users.Create(context.Background(), CreateUserParams{Email: "alice@example.test"})
			if err != nil || user.UserID != "fixture-user" {
				t.Fatalf("user envelope lost ID: %+v %v", user, err)
			}
		})
	}
}
