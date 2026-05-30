package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	domainfile "github.com/the-monkeys/freerangenotify/internal/domain/file"
)

// --- in-memory test doubles ---------------------------------------------

type memStore struct {
	mu      sync.Mutex
	objects map[string][]byte // key = appID + "/" + fileID
	putErr  error
}

func (m *memStore) key(a, f string) string { return a + "/" + f }

func (m *memStore) Put(_ context.Context, a, f string, r io.Reader, size int64) error {
	if m.putErr != nil {
		return m.putErr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if size > 0 && int64(len(b)) != size {
		return fmt.Errorf("size mismatch")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.objects == nil {
		m.objects = map[string][]byte{}
	}
	m.objects[m.key(a, f)] = b
	return nil
}

func (m *memStore) Get(_ context.Context, a, f string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[m.key(a, f)]
	if !ok {
		return nil, domainfile.ErrFileNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *memStore) Delete(_ context.Context, a, f string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(a, f)
	if _, ok := m.objects[k]; !ok {
		return domainfile.ErrFileNotFound
	}
	delete(m.objects, k)
	return nil
}

func (m *memStore) Exists(_ context.Context, a, f string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[m.key(a, f)]
	return ok, nil
}

type memRepo struct {
	mu     sync.Mutex
	byID   map[string]*domainfile.FileObject
	create func(*domainfile.FileObject) error // optional override
}

func (m *memRepo) Create(_ context.Context, f *domainfile.FileObject) error {
	if m.create != nil {
		return m.create(f)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byID == nil {
		m.byID = map[string]*domainfile.FileObject{}
	}
	if _, exists := m.byID[f.FileID]; exists {
		return errors.New("already exists")
	}
	clone := *f
	m.byID[f.FileID] = &clone
	return nil
}

func (m *memRepo) GetByID(_ context.Context, app, id string) (*domainfile.FileObject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.byID[id]
	if !ok || f.AppID != app {
		return nil, domainfile.ErrFileNotFound
	}
	clone := *f
	return &clone, nil
}

func (m *memRepo) Delete(_ context.Context, app, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.byID[id]
	if !ok || f.AppID != app {
		return domainfile.ErrFileNotFound
	}
	delete(m.byID, id)
	return nil
}

func (m *memRepo) List(_ context.Context, app string, limit, offset int) ([]*domainfile.FileObject, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*domainfile.FileObject, 0)
	for _, f := range m.byID {
		if f.AppID == app {
			c := *f
			out = append(out, &c)
		}
	}
	return out, int64(len(out)), nil
}

func newSvc(t *testing.T, cfg FileServiceConfig) (*FileService, *memStore, *memRepo) {
	t.Helper()
	store := &memStore{}
	repo := &memRepo{}
	s := NewFileService(store, repo, cfg, nil)
	s.now = func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	// Deterministic id generator for assertions.
	var n int
	s.idGen = func() string { n++; return fmt.Sprintf("file_test_%d", n) }
	return s, store, repo
}

// --- tests --------------------------------------------------------------

func TestFileService_Upload_RoundTrip(t *testing.T) {
	s, store, repo := newSvc(t, FileServiceConfig{})
	ctx := context.Background()
	payload := []byte("hello world")

	obj, err := s.Upload(ctx, UploadInput{
		AppID:        "app1",
		Name:         "invoice.pdf",
		MIMEType:     "application/pdf",
		DeclaredSize: int64(len(payload)),
		Reader:       bytes.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if obj.FileID != "file_test_1" {
		t.Errorf("FileID = %q", obj.FileID)
	}
	if obj.SHA256 != "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9" {
		t.Errorf("SHA256 = %q", obj.SHA256)
	}
	if obj.ExpiresAt.IsZero() {
		t.Error("default retention should set ExpiresAt")
	}
	if obj.MIMEType != "application/pdf" {
		t.Errorf("mime = %q", obj.MIMEType)
	}

	// Bytes landed in store and metadata in repo.
	if got := store.objects[store.key("app1", obj.FileID)]; !bytes.Equal(got, payload) {
		t.Errorf("store bytes = %q", got)
	}
	if _, ok := repo.byID[obj.FileID]; !ok {
		t.Error("repo missing file record")
	}
}

func TestFileService_Upload_RejectsUnsupportedMIME(t *testing.T) {
	s, _, _ := newSvc(t, FileServiceConfig{})
	_, err := s.Upload(context.Background(), UploadInput{
		AppID: "app1", Name: "x.exe", MIMEType: "application/x-msdownload",
		DeclaredSize: 1, Reader: strings.NewReader("x"),
	})
	if err != domainfile.ErrUnsupportedMIMEType {
		t.Errorf("want ErrUnsupportedMIMEType, got %v", err)
	}
}

func TestFileService_Upload_WildcardAllowlist(t *testing.T) {
	s, _, _ := newSvc(t, FileServiceConfig{AllowedMIMETypes: []string{"image/*"}})
	_, err := s.Upload(context.Background(), UploadInput{
		AppID: "app1", Name: "x.heic", MIMEType: "image/heic",
		DeclaredSize: 3, Reader: strings.NewReader("xyz"),
	})
	if err != nil {
		t.Errorf("image/* wildcard should accept image/heic; got %v", err)
	}
}

func TestFileService_Upload_RejectsOversize(t *testing.T) {
	s, _, _ := newSvc(t, FileServiceConfig{MaxBytes: 10})
	_, err := s.Upload(context.Background(), UploadInput{
		AppID: "app1", Name: "x.pdf", MIMEType: "application/pdf",
		DeclaredSize: 11, Reader: strings.NewReader("xxxxxxxxxxx"),
	})
	if err != domainfile.ErrFileTooLarge {
		t.Errorf("want ErrFileTooLarge, got %v", err)
	}
}

func TestFileService_Upload_RejectsInvalidInputs(t *testing.T) {
	s, _, _ := newSvc(t, FileServiceConfig{})
	cases := []struct {
		name string
		in   UploadInput
		want error
	}{
		{"empty app", UploadInput{Name: "n", MIMEType: "application/pdf", DeclaredSize: 1, Reader: strings.NewReader("x")}, domainfile.ErrInvalidAppID},
		{"empty name", UploadInput{AppID: "app1", MIMEType: "application/pdf", DeclaredSize: 1, Reader: strings.NewReader("x")}, domainfile.ErrInvalidFileName},
		{"empty mime", UploadInput{AppID: "app1", Name: "n", DeclaredSize: 1, Reader: strings.NewReader("x")}, domainfile.ErrInvalidMIMEType},
		{"zero size", UploadInput{AppID: "app1", Name: "n", MIMEType: "application/pdf", DeclaredSize: 0, Reader: strings.NewReader("x")}, domainfile.ErrInvalidSize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.Upload(context.Background(), tc.in); err != tc.want {
				t.Errorf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestFileService_Upload_CleansUpOnRepoFailure(t *testing.T) {
	s, store, repo := newSvc(t, FileServiceConfig{})
	repo.create = func(_ *domainfile.FileObject) error { return errors.New("es down") }

	_, err := s.Upload(context.Background(), UploadInput{
		AppID: "app1", Name: "n.pdf", MIMEType: "application/pdf",
		DeclaredSize: 3, Reader: strings.NewReader("xyz"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(store.objects) != 0 {
		t.Errorf("store must be empty after repo failure; got %v", store.objects)
	}
}

func TestFileService_Get_Tenancy(t *testing.T) {
	s, _, _ := newSvc(t, FileServiceConfig{})
	ctx := context.Background()
	obj, _ := s.Upload(ctx, UploadInput{
		AppID: "app1", Name: "n.pdf", MIMEType: "application/pdf",
		DeclaredSize: 3, Reader: strings.NewReader("xyz"),
	})
	if _, err := s.Get(ctx, "app2", obj.FileID); err != domainfile.ErrFileNotFound {
		t.Errorf("cross-tenant Get: want ErrFileNotFound, got %v", err)
	}
	if _, err := s.Get(ctx, "app1", obj.FileID); err != nil {
		t.Errorf("same-tenant Get failed: %v", err)
	}
}

func TestFileService_OpenContent_RejectsExpired(t *testing.T) {
	s, _, repo := newSvc(t, FileServiceConfig{})
	ctx := context.Background()
	obj, _ := s.Upload(ctx, UploadInput{
		AppID: "app1", Name: "n.pdf", MIMEType: "application/pdf",
		DeclaredSize: 3, Reader: strings.NewReader("xyz"),
	})
	// Backdate expiry in the repo.
	repo.byID[obj.FileID].ExpiresAt = time.Unix(1, 0)
	_, _, err := s.OpenContent(ctx, "app1", obj.FileID)
	if err != domainfile.ErrFileExpired {
		t.Errorf("want ErrFileExpired, got %v", err)
	}
}

func TestFileService_Delete_RemovesBoth(t *testing.T) {
	s, store, repo := newSvc(t, FileServiceConfig{})
	ctx := context.Background()
	obj, _ := s.Upload(ctx, UploadInput{
		AppID: "app1", Name: "n.pdf", MIMEType: "application/pdf",
		DeclaredSize: 3, Reader: strings.NewReader("xyz"),
	})
	if err := s.Delete(ctx, "app1", obj.FileID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := store.objects[store.key("app1", obj.FileID)]; ok {
		t.Error("store should be empty")
	}
	if _, ok := repo.byID[obj.FileID]; ok {
		t.Error("repo should be empty")
	}
}

func TestFileService_Delete_RejectsCrossTenant(t *testing.T) {
	s, _, _ := newSvc(t, FileServiceConfig{})
	ctx := context.Background()
	obj, _ := s.Upload(ctx, UploadInput{
		AppID: "app1", Name: "n.pdf", MIMEType: "application/pdf",
		DeclaredSize: 3, Reader: strings.NewReader("xyz"),
	})
	if err := s.Delete(ctx, "app2", obj.FileID); err != domainfile.ErrFileNotFound {
		t.Errorf("cross-tenant Delete: want ErrFileNotFound, got %v", err)
	}
}

func TestFileService_Retention_NegativeMeansNeverExpire(t *testing.T) {
	s, _, _ := newSvc(t, FileServiceConfig{Retention: -1})
	ctx := context.Background()
	obj, _ := s.Upload(ctx, UploadInput{
		AppID: "app1", Name: "n.pdf", MIMEType: "application/pdf",
		DeclaredSize: 3, Reader: strings.NewReader("xyz"),
	})
	if !obj.ExpiresAt.IsZero() {
		t.Errorf("retention=-1 means never expire; got ExpiresAt=%v", obj.ExpiresAt)
	}
}

// AV scanner hook fires for small payloads and rejection deletes bytes.
type stubScanner struct{ err error; called bool }

func (s *stubScanner) Scan(_ context.Context, _, _ string, _ []byte) error {
	s.called = true
	return s.err
}

func TestFileService_Upload_VirusScanRejection(t *testing.T) {
	scanner := &stubScanner{err: errors.New("malware")}
	s, store, repo := newSvc(t, FileServiceConfig{VirusScanner: scanner})
	_, err := s.Upload(context.Background(), UploadInput{
		AppID: "app1", Name: "n.pdf", MIMEType: "application/pdf",
		DeclaredSize: 3, Reader: strings.NewReader("xyz"),
	})
	if err == nil || !scanner.called {
		t.Fatalf("scanner should fire and reject; err=%v called=%v", err, scanner.called)
	}
	if len(store.objects) != 0 {
		t.Error("bytes must be cleaned up after AV rejection")
	}
	if len(repo.byID) != 0 {
		t.Error("metadata must not be persisted after AV rejection")
	}
}
