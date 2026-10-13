package freerangenotify

import (
	"encoding/json"
	"testing"
)

func TestBulkCreateDecodesStructuredAndLegacyErrors(t *testing.T) {
	for _, body := range []string{`{"created":1,"updated":2,"skipped":3,"total":7,"errors":[{"index":6,"email":"a@example.com","message":"write rejected"}]}`, `{"created":1,"errors":["write rejected"]}`} {
		var out BulkCreateUsersResult
		if err := json.Unmarshal([]byte(body), &out); err != nil {
			t.Fatal(err)
		}
		if len(out.Errors) != 1 || out.Errors[0] != "write rejected" {
			t.Fatalf("lost errors: %#v", out)
		}
	}
}
