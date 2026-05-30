package filestore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainfile "github.com/the-monkeys/freerangenotify/internal/domain/file"
)

func newLocal(t *testing.T) *LocalStore {
	t.Helper()
	dir := t.TempDir()
	s, err := NewLocalStore(dir, nil)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	return s
}

func TestNewLocalStore_RejectsEmptyRoot(t *testing.T) {
	if _, err := NewLocalStore("", nil); err == nil {
		t.Fatal("expected error for empty root")
	}
}

func TestNewLocalStore_RejectsFileAsRoot(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalStore(f, nil); err == nil {
		t.Fatal("expected error for file root")
	}
}

func TestLocalStore_PutGetDelete_Roundtrip(t *testing.T) {
	s := newLocal(t)
	ctx := context.Background()
	payload := []byte("hello-world")

	if err := s.Put(ctx, "app1", "file_abc", bytes.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatalf("Put: %v", err)
	}

	rc, err := s.Get(ctx, "app1", "file_abc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, payload)
	}

	ok, err := s.Exists(ctx, "app1", "file_abc")
	if err != nil || !ok {
		t.Errorf("Exists ok=%v err=%v", ok, err)
	}

	if err := s.Delete(ctx, "app1", "file_abc"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "app1", "file_abc"); !errors.Is(err, domainfile.ErrFileNotFound) {
		t.Errorf("Get after Delete: want ErrFileNotFound, got %v", err)
	}
}

func TestLocalStore_Tenancy_Isolated(t *testing.T) {
	s := newLocal(t)
	ctx := context.Background()
	if err := s.Put(ctx, "app1", "file_x", strings.NewReader("a"), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "app2", "file_x"); !errors.Is(err, domainfile.ErrFileNotFound) {
		t.Errorf("cross-tenant Get should be ErrFileNotFound; got %v", err)
	}
	if err := s.Delete(ctx, "app2", "file_x"); !errors.Is(err, domainfile.ErrFileNotFound) {
		t.Errorf("cross-tenant Delete should be ErrFileNotFound; got %v", err)
	}
	ok, _ := s.Exists(ctx, "app2", "file_x")
	if ok {
		t.Error("Exists must be false for other tenant")
	}
}

func TestLocalStore_RejectsPathTraversal(t *testing.T) {
	s := newLocal(t)
	ctx := context.Background()

	badIDs := []string{
		"../etc/passwd",
		"..",
		".",
		"foo/bar",
		"foo\\bar",
		"",
		" ",
		strings.Repeat("a", 129),
		"weird*char",
	}
	for _, id := range badIDs {
		t.Run("put_"+id, func(t *testing.T) {
			err := s.Put(ctx, "app1", id, strings.NewReader("x"), 1)
			if err == nil {
				t.Errorf("Put with fileID %q should fail", id)
			}
		})
	}
	for _, id := range badIDs {
		t.Run("put_app_"+id, func(t *testing.T) {
			err := s.Put(ctx, id, "file_ok", strings.NewReader("x"), 1)
			if err == nil {
				t.Errorf("Put with appID %q should fail", id)
			}
		})
	}
}

func TestLocalStore_SizeMismatchAborts(t *testing.T) {
	s := newLocal(t)
	ctx := context.Background()
	err := s.Put(ctx, "app1", "file_size", strings.NewReader("hi"), 99)
	if err == nil {
		t.Fatal("expected size-mismatch error")
	}
	ok, _ := s.Exists(ctx, "app1", "file_size")
	if ok {
		t.Error("partial file must be cleaned up on size mismatch")
	}
}

func TestLocalStore_Put_SizeZeroAllowed(t *testing.T) {
	// size=0 means "don't enforce" — write whatever the reader provides.
	s := newLocal(t)
	ctx := context.Background()
	if err := s.Put(ctx, "app1", "file_any", strings.NewReader("payload"), 0); err != nil {
		t.Fatalf("Put with size=0 should succeed; got %v", err)
	}
}

func TestLocalStore_ContextCancellation(t *testing.T) {
	s := newLocal(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.Put(ctx, "app1", "file_ctx", strings.NewReader("x"), 1)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestLocalStore_OnlyTenantDirIsCreated(t *testing.T) {
	s := newLocal(t)
	ctx := context.Background()
	if err := s.Put(ctx, "app1", "file_a", strings.NewReader("x"), 1); err != nil {
		t.Fatal(err)
	}
	root := s.Root()
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 || entries[0].Name() != "app1" {
		t.Errorf("root should contain only app1 dir; got %v", entries)
	}
}
