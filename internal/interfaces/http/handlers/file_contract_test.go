package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	domainfile "github.com/the-monkeys/freerangenotify/internal/domain/file"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/filestore"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
)

type contractFileRepo struct {
	domainfile.Repository
	objects map[string]*domainfile.FileObject
}

func (r *contractFileRepo) Create(_ context.Context, f *domainfile.FileObject) error {
	r.objects[f.FileID] = f
	return nil
}
func (r *contractFileRepo) GetByID(_ context.Context, app, id string) (*domainfile.FileObject, error) {
	f := r.objects[id]
	if f == nil || f.AppID != app {
		return nil, domainfile.ErrFileNotFound
	}
	return f, nil
}

func contractFiles(t *testing.T, cap int64) (*services.FileService, *filestore.Signer) {
	store, err := filestore.NewLocalStore(t.TempDir(), nil)
	require.NoError(t, err)
	svc := services.NewFileService(store, &contractFileRepo{objects: map[string]*domainfile.FileObject{}}, services.FileServiceConfig{MaxBytes: cap}, nil)
	signer, err := filestore.NewSigner(time.Minute, "fixture-key-only")
	require.NoError(t, err)
	return svc, signer
}
func TestFileGeneratedURLRoundTrip(t *testing.T) {
	svc, signer := contractFiles(t, 50<<20)
	obj, err := svc.Upload(context.Background(), services.UploadInput{AppID: "app-a", Name: "test.txt", MIMEType: "text/plain", DeclaredSize: 5, Reader: strings.NewReader("hello")})
	require.NoError(t, err)
	h := NewFileHandler(svc, signer, "https://notify.example/prefix", nil)
	app := fiber.New()
	app.Get("/prefix/v1/files/download/:id", h.PublicDownload)
	app.Get("/mint/:id", func(c *fiber.Ctx) error { c.Locals("app_id", "app-a"); return h.DownloadURL(c) })
	res, err := app.Test(httptest.NewRequest("GET", "/mint/"+obj.FileID, nil))
	require.NoError(t, err)
	var minted struct {
		URL string `json:"url"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&minted))
	res.Body.Close()
	require.Equal(t, filestore.BuildSignedDownloadURL("https://notify.example/prefix", "app-a", obj.FileID, parseUnix(t, urlQuery(t, minted.URL, "exp")), urlQuery(t, minted.URL, "sig")), minted.URL)
	res, err = app.Test(httptest.NewRequest("GET", minted.URL, nil))
	require.NoError(t, err)
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	require.Equal(t, 200, res.StatusCode)
	require.Equal(t, "hello", string(body))
	for _, change := range []struct {
		key, value string
		status     int
	}{{"app_id", "", 400}, {"app_id", "other", 404}, {"sig", "bad", 404}, {"exp", "1", 410}} {
		u, err := url.Parse(minted.URL)
		require.NoError(t, err)
		q := u.Query()
		q.Set(change.key, change.value)
		u.RawQuery = q.Encode()
		res, err = app.Test(httptest.NewRequest("GET", u.String(), nil))
		require.NoError(t, err)
		res.Body.Close()
		require.Equal(t, change.status, res.StatusCode)
	}
}
func urlQuery(t *testing.T, link, key string) string {
	u, err := url.Parse(link)
	require.NoError(t, err)
	return u.Query().Get(key)
}

func TestUploadBodyFileServiceBoundaries(t *testing.T) {
	for _, cap := range []int64{50 << 20, 1 << 20} {
		t.Run(fmt.Sprint(cap), func(t *testing.T) {
			svc, signer := contractFiles(t, cap)
			h := NewFileHandler(svc, signer, "", nil)
			app := fiber.New(fiber.Config{BodyLimit: middleware.UploadTransportLimit(cap)})
			app.Use(func(c *fiber.Ctx) error { c.Locals("app_id", "app-a"); return c.Next() })
			app.Use(middleware.NonUploadBodyLimit(16 << 20))
			app.Post("/v1/files", h.Upload)
			app.Post("/json", func(c *fiber.Ctx) error { return c.SendStatus(204) })
			for _, size := range []int{17 << 20, int(cap) + 1} {
				var body bytes.Buffer
				writer := multipart.NewWriter(&body)
				header := textproto.MIMEHeader{}
				header.Set("Content-Disposition", `form-data; name="file"; filename="large.txt"`)
				header.Set("Content-Type", "text/plain")
				part, err := writer.CreatePart(header)
				require.NoError(t, err)
				_, err = io.Copy(part, io.LimitReader(zeroReader{}, int64(size)))
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				req := httptest.NewRequest("POST", "/v1/files", &body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				// Race instrumentation makes large multipart copies slower; this checks size admission, not latency.
				res, err := app.Test(req, 120000)
				if req.ContentLength > int64(middleware.UploadTransportLimit(cap)) {
					require.Error(t, err)
					continue
				}
				require.NoError(t, err)
				res.Body.Close()
				expected := 201
				if int64(size) > cap {
					expected = 413
				}
				require.Equal(t, expected, res.StatusCode)
			}
			req := httptest.NewRequest("POST", "/json", strings.NewReader(strings.Repeat("x", 15<<20)))
			req.Header.Set("Content-Type", "application/json")
			res, err := app.Test(req, 10000)
			require.NoError(t, err)
			res.Body.Close()
			require.Equal(t, 204, res.StatusCode)
		})
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func parseUnix(t *testing.T, s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	require.NoError(t, err)
	return v
}
