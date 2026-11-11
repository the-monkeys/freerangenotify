package testutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBizRequiredFixtureRejectsUnownedEndpoint(t *testing.T) {
	p := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(p, []byte(`{"runID":"fake","project":"production","services":{"elasticsearch":{"id":"other","url":"http://127.0.0.1:9200"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FRN_BIZ_FIXTURE_MANIFEST", p)
	if _, err := ValidateBizFixture(); !errors.Is(err, ErrUnownedFixture) {
		t.Fatalf("unowned fixture accepted: %v", err)
	}
}
