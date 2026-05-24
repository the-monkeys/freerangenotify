package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
)

// memRichRepo is an in-memory RichTemplateRepository used by service tests.
// Keeps assertions about persistence (was Create called? was Update called?)
// in the same package, instead of wiring the real Elasticsearch backend.
type memRichRepo struct {
	mu    sync.Mutex
	byID  map[string]*whatsapp.RichTemplate
}

func newMemRichRepo() *memRichRepo {
	return &memRichRepo{byID: map[string]*whatsapp.RichTemplate{}}
}

func (r *memRichRepo) Create(_ context.Context, tpl *whatsapp.RichTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[tpl.ID] = tpl
	return nil
}
func (r *memRichRepo) GetByID(_ context.Context, id string) (*whatsapp.RichTemplate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tpl, ok := r.byID[id]
	if !ok {
		return nil, &notFoundError{id: id}
	}
	return tpl, nil
}
func (r *memRichRepo) GetByName(_ context.Context, appID, name string) (*whatsapp.RichTemplate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.byID {
		if t.AppID == appID && t.Name == name {
			return t, nil
		}
	}
	return nil, nil
}
func (r *memRichRepo) List(_ context.Context, filter whatsapp.RichTemplateFilter) ([]*whatsapp.RichTemplate, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*whatsapp.RichTemplate, 0, len(r.byID))
	for _, t := range r.byID {
		if filter.AppID != "" && t.AppID != filter.AppID {
			continue
		}
		out = append(out, t)
	}
	return out, int64(len(out)), nil
}
func (r *memRichRepo) Update(_ context.Context, tpl *whatsapp.RichTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[tpl.ID] = tpl
	return nil
}
func (r *memRichRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, id)
	return nil
}

type notFoundError struct{ id string }

func (e *notFoundError) Error() string { return "not found: " + e.id }

// richStubAppRepo returns a single Meta-configured app for service tests. Other
// Repository methods are unused on the rich-template service hot paths.
// Named "rich…" to avoid colliding with stubAppRepo in credit_service_legacy_test.go.
type richStubAppRepo struct{ app *application.Application }

func (r *richStubAppRepo) Create(context.Context, *application.Application) error { return nil }
func (r *richStubAppRepo) GetByID(_ context.Context, id string) (*application.Application, error) {
	if r.app != nil && r.app.AppID == id {
		return r.app, nil
	}
	return nil, &notFoundError{id: id}
}
func (r *richStubAppRepo) GetByAPIKey(context.Context, string) (*application.Application, error) {
	return nil, nil
}
func (r *richStubAppRepo) Update(context.Context, *application.Application) error { return nil }
func (r *richStubAppRepo) List(context.Context, application.ApplicationFilter) ([]*application.Application, error) {
	if r.app == nil {
		return nil, nil
	}
	return []*application.Application{r.app}, nil
}
func (r *richStubAppRepo) Delete(context.Context, string) error                          { return nil }
func (r *richStubAppRepo) RegenerateAPIKey(context.Context, string) (string, error)      { return "", nil }

// newServiceWithMockMeta wires up the service with an httptest server that
// captures the inbound POST body so each test can assert what JSON we sent
// to Meta. metaResponse is returned verbatim from POST + GET.
func newServiceWithMockMeta(t *testing.T, metaResponse string, metaStatus int) (
	svc WhatsAppRichTemplateService,
	repo *memRichRepo,
	captured *capturedRequest,
	cleanup func(),
) {
	t.Helper()
	captured = &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured.mu.Lock()
		captured.requests = append(captured.requests, capturedReq{Method: r.Method, Path: r.URL.Path, Body: string(body), Auth: r.Header.Get("Authorization")})
		captured.mu.Unlock()
		// Pass-through stubs for the Resumable Upload pipeline so carousel
		// fixtures don't need real media. Same shape as the production
		// Meta endpoints return.
		switch {
		case strings.Contains(r.URL.Path, "/uploads"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"upload:TEST_SESSION_123"}`))
			return
		case strings.Contains(r.URL.Path, "upload:"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"h":"4::test_handle_xyz"}`))
			return
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("FAKEIMAGEBYTES"))
			return
		}
		w.WriteHeader(metaStatus)
		_, _ = w.Write([]byte(metaResponse))
	}))

	// Wire a service that points its httpClient at the test server by
	// rewriting all outbound URLs at the transport layer. Cleaner than
	// templating graph.facebook.com inside the service.
	transport := &rewriteTransport{base: http.DefaultTransport, target: srv.URL}
	repo = newMemRichRepo()
	appRepo := &richStubAppRepo{app: &application.Application{
		AppID: "app-1",
		Settings: application.Settings{
			WhatsApp: &application.WhatsAppAppConfig{
				Provider:          "meta",
				MetaWABAID:        "WABA_X",
				MetaAccessToken:   "token-xyz",
				MetaPhoneNumberID: "PH_1",
			},
		},
	}}
	httpClient := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	svc = NewWhatsAppRichTemplateService(repo, appRepo, httpClient, "v23.0", "test-app-id", "test-app-secret", zap.NewNop())
	return svc, repo, captured, srv.Close
}

// rewriteTransport sends any outbound request to target (the httptest server)
// while preserving the path so the service code can keep its graph.facebook.com
// URLs unchanged.
type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace scheme+host with the test target, keep path/query.
	u := *req.URL
	u.Scheme = "http"
	u.Host = strings.TrimPrefix(t.target, "http://")
	req.URL = &u
	return t.base.RoundTrip(req)
}

// newServiceWithMockProviders wires the service with TWO httptest servers:
// one acting as Meta Graph API, one as Twilio Content API. Routing happens
// at the transport layer by Host (graph.facebook.com vs content.twilio.com).
// Used by Phase 2 tests that exercise the dual-provider Create path.
func newServiceWithMockProviders(t *testing.T) (
	svc WhatsAppRichTemplateService,
	repo *memRichRepo,
	metaCaptured, twilioCaptured *capturedRequest,
	cleanup func(),
) {
	t.Helper()
	metaCaptured = &capturedRequest{}
	twilioCaptured = &capturedRequest{}

	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		metaCaptured.mu.Lock()
		metaCaptured.requests = append(metaCaptured.requests, capturedReq{Method: r.Method, Path: r.URL.Path, Body: string(body), Auth: r.Header.Get("Authorization")})
		metaCaptured.mu.Unlock()
		switch {
		// Resumable Upload step A — init session.
		case strings.Contains(r.URL.Path, "/uploads"):
			_, _ = w.Write([]byte(`{"id":"upload:TEST_SESSION_123"}`))
		// Resumable Upload step B — upload bytes. The path begins with the
		// session id returned above.
		case strings.Contains(r.URL.Path, "upload:"):
			_, _ = w.Write([]byte(`{"h":"4::test_handle_xyz"}`))
		// Stand-in for the actual image bytes the service downloads
		// before uploading. The body content is irrelevant for these
		// tests; we only need a 200.
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("FAKEIMAGEBYTES"))
		default:
			_, _ = w.Write([]byte(`{"id":"meta-tpl-001","status":"PENDING"}`))
		}
	}))

	twilioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		twilioCaptured.mu.Lock()
		twilioCaptured.requests = append(twilioCaptured.requests, capturedReq{Method: r.Method, Path: r.URL.Path, Body: string(body), Auth: r.Header.Get("Authorization")})
		twilioCaptured.mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/Content"):
			_, _ = w.Write([]byte(`{"sid":"HXcontent123","friendly_name":"x"}`))
		case strings.Contains(r.URL.Path, "/ApprovalRequests/whatsapp"):
			_, _ = w.Write([]byte(`{"sid":"HXapproval456","status":"pending"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// Route requests by Host: graph.facebook.com → metaSrv, content.twilio.com → twilioSrv.
	transport := &hostRouterTransport{
		metaTarget:   metaSrv.URL,
		twilioTarget: twilioSrv.URL,
		base:         http.DefaultTransport,
	}

	repo = newMemRichRepo()
	appRepo := &richStubAppRepo{app: &application.Application{
		AppID: "app-1",
		Settings: application.Settings{
			WhatsApp: &application.WhatsAppAppConfig{
				Provider:          "meta",
				MetaWABAID:        "WABA_X",
				MetaAccessToken:   "token-xyz",
				MetaPhoneNumberID: "PH_1",
				// Twilio creds also present so dual-submission triggers.
				AccountSID: "AC_test",
				AuthToken:  "twauth",
				FromNumber: "+1555",
			},
		},
	}}
	httpClient := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	svc = NewWhatsAppRichTemplateService(repo, appRepo, httpClient, "v23.0", "test-app-id", "test-app-secret", zap.NewNop())
	return svc, repo, metaCaptured, twilioCaptured, func() {
		metaSrv.Close()
		twilioSrv.Close()
	}
}

// hostRouterTransport routes outbound HTTP based on the request Host header
// so a single *http.Client can talk to both the mocked Meta endpoint and
// the mocked Twilio endpoint within one test.
type hostRouterTransport struct {
	metaTarget   string
	twilioTarget string
	base         http.RoundTripper
}

func (t *hostRouterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	u.Scheme = "http"
	switch {
	case strings.Contains(req.URL.Host, "content.twilio.com"):
		u.Host = strings.TrimPrefix(t.twilioTarget, "http://")
	default:
		// Everything else (graph.facebook.com plus any test cdn:// host
		// the upload pre-fetch tries to download) goes to the meta mock.
		u.Host = strings.TrimPrefix(t.metaTarget, "http://")
	}
	req.URL = &u
	return t.base.RoundTrip(req)
}

type capturedReq struct {
	Method string
	Path   string
	Body   string
	Auth   string
}

type capturedRequest struct {
	mu       sync.Mutex
	requests []capturedReq
}

func (c *capturedRequest) all() []capturedReq {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]capturedReq, len(c.requests))
	copy(out, c.requests)
	return out
}

// TestCreate_CarouselSubmitsToMeta asserts that creating a carousel rich
// template (a) submits the documented authoring JSON to Meta, (b) persists
// the returned template_id under Providers.Meta, and (c) sets the aggregate
// approval state to Pending while Meta reviews.
func TestCreate_CarouselSubmitsToMeta(t *testing.T) {
	resp := `{"id":"meta-tpl-001","status":"PENDING","category":"MARKETING"}`
	svc, repo, captured, cleanup := newServiceWithMockMeta(t, resp, http.StatusOK)
	defer cleanup()

	tpl := &whatsapp.RichTemplate{
		Name:     "snapdeal_styles",
		AppID:    "app-1",
		Language: "en_US",
		Category: "MARKETING",
		Kind:     whatsapp.KindCarousel,
		Body:     "Hi {{1}}, check these out:",
		Cards: []whatsapp.CarouselCard{
			{
				HeaderImageURL: "https://cdn/p1.jpg",
				Body:           "Polo {{1}} {{2}}",
				Buttons:        []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "View", URL: "https://shop/p/1"}},
			},
			{
				HeaderImageURL: "https://cdn/p2.jpg",
				Body:           "Tee {{1}}",
				Buttons:        []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "View", URL: "https://shop/p/2"}},
			},
		},
	}

	got, err := svc.Create(context.Background(), tpl)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Identity stamped + persisted.
	if got.ID == "" || !strings.HasPrefix(got.ID, "frn_tpl_") {
		t.Errorf("expected frn_tpl_ prefixed id, got %q", got.ID)
	}
	if _, err := repo.GetByID(context.Background(), got.ID); err != nil {
		t.Errorf("template not persisted: %v", err)
	}
	if got.Providers.Meta == nil || got.Providers.Meta.TemplateID != "meta-tpl-001" {
		t.Errorf("MetaBinding not populated: %+v", got.Providers.Meta)
	}
	if got.ApprovalState != whatsapp.ApprovalPending {
		t.Errorf("ApprovalState: got %q, want pending", got.ApprovalState)
	}

	// Wire-shape: the create flow now also performs Resumable Upload for
	// each distinct carousel header image (Meta requires handles, not
	// raw URLs). Expected pipeline per card: GET image bytes, POST /uploads
	// to init session, POST upload:... to send bytes, then ONE POST
	// /message_templates carrying the resulting handles.
	reqs := captured.all()
	if len(reqs) < 2 {
		t.Fatalf("expected at least 2 outbound requests (upload + submit), got %d", len(reqs))
	}
	// Find the template submission — it's the only POST hitting
	// /message_templates.
	var submitReq *capturedReq
	for i := range reqs {
		if strings.HasSuffix(reqs[i].Path, "/WABA_X/message_templates") {
			submitReq = &reqs[i]
			break
		}
	}
	if submitReq == nil {
		t.Fatalf("no template submission found among requests: %+v", reqs)
	}
	if submitReq.Method != "POST" {
		t.Errorf("expected POST, got %s", submitReq.Method)
	}
	if submitReq.Auth != "Bearer token-xyz" {
		t.Errorf("auth header: got %q", submitReq.Auth)
	}
	// Spot-check the JSON: carousel + per-card body example present, and
	// the header_handle should now contain the uploaded handle, NOT the
	// raw cdn URL.
	for _, needle := range []string{
		`"type":"CAROUSEL"`,
		`"category":"MARKETING"`,
		`"language":"en_US"`,
		`"header_handle":["4::test_handle_xyz"]`, // upload handle, not raw URL
		`"body_text":[["sample","sample"]]`,      // 2 vars in card 0 body
	} {
		if !strings.Contains(submitReq.Body, needle) {
			t.Errorf("Meta payload missing %q\nbody=%s", needle, submitReq.Body)
		}
	}
	// The raw cdn URL must NOT appear in the submission body — that was
	// the exact bug that caused error 100/2388215 in production.
	if strings.Contains(submitReq.Body, "https://cdn/p1.jpg") {
		t.Errorf("submission body still contains raw cdn URL; upload swap did not run\nbody=%s", submitReq.Body)
	}
}

// TestCreate_ValidationFailsBeforeSubmit asserts that invalid templates are
// rejected by the validator with no Meta round-trip.
func TestCreate_ValidationFailsBeforeSubmit(t *testing.T) {
	svc, _, captured, cleanup := newServiceWithMockMeta(t, `{}`, http.StatusOK)
	defer cleanup()

	// Carousel with only 1 card violates CAROUSEL_CARD_COUNT.
	tpl := &whatsapp.RichTemplate{
		Name: "bad", AppID: "app-1", Language: "en_US", Category: "MARKETING", Kind: whatsapp.KindCarousel,
		Cards: []whatsapp.CarouselCard{{HeaderImageURL: "https://x", Body: "x", Buttons: []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "x", URL: "https://x"}}}},
	}
	_, err := svc.Create(context.Background(), tpl)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	if len(captured.all()) != 0 {
		t.Errorf("validator failed to short-circuit; Meta was called")
	}
}

// TestCreate_RejectsDuplicateName ensures (app_id, name) uniqueness is
// enforced at the service boundary so the UI gets a clear conflict.
func TestCreate_RejectsDuplicateName(t *testing.T) {
	svc, _, _, cleanup := newServiceWithMockMeta(t, `{"id":"x","status":"PENDING"}`, http.StatusOK)
	defer cleanup()

	tpl := func() *whatsapp.RichTemplate {
		return &whatsapp.RichTemplate{
			Name: "duplicate", AppID: "app-1", Language: "en_US", Category: "UTILITY",
			Kind: whatsapp.KindCTAURL, Body: "x",
			Buttons: []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "Go", URL: "https://x"}},
		}
	}

	if _, err := svc.Create(context.Background(), tpl()); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(context.Background(), tpl())
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected duplicate-name error, got %v", err)
	}
}

// TestApplyMetaStatus_UpdatesBindingAndAggregate verifies the webhook path:
// a status update mutates the per-provider status and recomputes the
// aggregate ApprovalState.
func TestApplyMetaStatus_UpdatesBindingAndAggregate(t *testing.T) {
	svc, repo, _, cleanup := newServiceWithMockMeta(t, `{"id":"meta-x","status":"PENDING"}`, http.StatusOK)
	defer cleanup()

	tpl, err := svc.Create(context.Background(), &whatsapp.RichTemplate{
		Name: "promo", AppID: "app-1", Language: "en_US", Category: "MARKETING",
		Kind: whatsapp.KindCTAURL, Body: "tap",
		Buttons: []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "Go", URL: "https://x"}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tpl.ApprovalState != whatsapp.ApprovalPending {
		t.Fatalf("expected pending after create, got %s", tpl.ApprovalState)
	}

	if err := svc.ApplyMetaStatus(context.Background(), "WABA_X", "promo", "APPROVED", ""); err != nil {
		t.Fatalf("ApplyMetaStatus: %v", err)
	}
	got, _ := repo.GetByID(context.Background(), tpl.ID)
	if got.Providers.Meta.Status != "APPROVED" {
		t.Errorf("Meta status: got %q", got.Providers.Meta.Status)
	}
	if got.ApprovalState != whatsapp.ApprovalApproved {
		t.Errorf("aggregate: got %q, want approved", got.ApprovalState)
	}
}

// TestCreate_AlsoSubmitsToTwilio asserts that when an app has BOTH Meta
// and Twilio credentials, Create submits to both providers and persists
// both bindings. Twilio submission is best-effort: the test wires the
// happy path here; the warn-on-failure path is exercised separately.
func TestCreate_AlsoSubmitsToTwilio(t *testing.T) {
	svc, _, metaCap, twilioCap, cleanup := newServiceWithMockProviders(t)
	defer cleanup()

	tpl, err := svc.Create(context.Background(), &whatsapp.RichTemplate{
		Name: "dual_promo", AppID: "app-1", Language: "en_US", Category: "MARKETING",
		Kind: whatsapp.KindCTAURL, Body: "Tap below",
		Buttons: []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "Visit", URL: "https://x"}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tpl.Providers.Meta == nil || tpl.Providers.Meta.TemplateID != "meta-tpl-001" {
		t.Errorf("Meta binding missing: %+v", tpl.Providers.Meta)
	}
	if tpl.Providers.Twilio == nil || tpl.Providers.Twilio.ContentSid != "HXcontent123" {
		t.Errorf("Twilio binding missing: %+v", tpl.Providers.Twilio)
	}
	if tpl.Providers.Twilio.ApprovalSid != "HXapproval456" {
		t.Errorf("Twilio approval_sid missing: %+v", tpl.Providers.Twilio)
	}
	if len(metaCap.all()) != 1 {
		t.Errorf("expected 1 Meta call, got %d", len(metaCap.all()))
	}
	if len(twilioCap.all()) != 2 {
		t.Errorf("expected 2 Twilio calls (create+approval), got %d", len(twilioCap.all()))
	}
	createCall := twilioCap.all()[0]
	for _, needle := range []string{`"twilio/call-to-action"`, `"body":"Tap below"`, `"url":"https://x"`} {
		if !strings.Contains(createCall.Body, needle) {
			t.Errorf("Twilio body missing %q\nbody=%s", needle, createCall.Body)
		}
	}
}

// TestApplyTwilioStatus_UpdatesBindingAndAggregate verifies the Twilio
// webhook path: receiving an "approved" status flips the aggregate from
// pending to approved.
func TestApplyTwilioStatus_UpdatesBindingAndAggregate(t *testing.T) {
	svc, repo, _, _, cleanup := newServiceWithMockProviders(t)
	defer cleanup()

	tpl, err := svc.Create(context.Background(), &whatsapp.RichTemplate{
		Name: "twilio_promo", AppID: "app-1", Language: "en_US", Category: "MARKETING",
		Kind: whatsapp.KindCTAURL, Body: "x",
		Buttons: []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "Go", URL: "https://x"}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.ApplyTwilioStatus(context.Background(), "HXcontent123", "approved", ""); err != nil {
		t.Fatalf("ApplyTwilioStatus: %v", err)
	}
	got, _ := repo.GetByID(context.Background(), tpl.ID)
	if got.Providers.Twilio.Status != "approved" {
		t.Errorf("Twilio status: got %q", got.Providers.Twilio.Status)
	}
	if got.ApprovalState != whatsapp.ApprovalApproved {
		t.Errorf("aggregate: got %q, want approved", got.ApprovalState)
	}
}

// TestResolveSendPayload_CarouselSignsTrackedURLs proves that a carousel
// template with track_clicks=true buttons produces signed redirect values
// at runtime — i.e., the worker can pass the resolved payload straight to
// the Meta provider.
func TestResolveSendPayload_CarouselSignsTrackedURLs(t *testing.T) {
	svc, _, _, _, cleanup := newServiceWithMockProviders(t)
	defer cleanup()

	signer, err := whatsapp.NewClickSigner("test-key")
	if err != nil {
		t.Fatalf("NewClickSigner: %v", err)
	}
	svc.EnableClickTracking(signer, "https://api.test")

	tpl, err := svc.Create(context.Background(), &whatsapp.RichTemplate{
		Name: "tracked_carousel", AppID: "app-1", Language: "en_US", Category: "MARKETING",
		Kind: whatsapp.KindCarousel,
		Body: "Hi {{1}}",
		Cards: []whatsapp.CarouselCard{
			{HeaderImageURL: "https://cdn/1.jpg", Body: "Polo {{1}}",
				Buttons: []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "View", URL: "https://shop/p/{{1}}", Example: "sku-1", TrackClicks: true}}},
			{HeaderImageURL: "https://cdn/2.jpg", Body: "Tee {{1}}",
				Buttons: []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "View", URL: "https://shop/p/{{1}}", Example: "sku-2", TrackClicks: true}}},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	resolved, err := svc.ResolveSendPayload(context.Background(), "app-1", "notif-7", whatsapp.SendPayload{
		TemplateID: tpl.ID,
		Variables:  map[string]string{"1": "Asha"},
		Cards: []whatsapp.SendPayloadCard{
			{Variables: map[string]string{"1": "Polo Trendy"}, ButtonValues: []string{"sku-1"}},
			{Variables: map[string]string{"1": "Tee Cool"}, ButtonValues: []string{"sku-2"}},
		},
	})
	if err != nil {
		t.Fatalf("ResolveSendPayload: %v", err)
	}

	// Sanity-check the shape.
	if resolved["template_name"] != tpl.Providers.Meta.TemplateName {
		t.Errorf("template_name mismatch: %v", resolved["template_name"])
	}
	cards, ok := resolved["cards"].([]map[string]interface{})
	if !ok || len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %v", resolved["cards"])
	}
	for i, c := range cards {
		buttons, _ := c["buttons"].([]map[string]interface{})
		if len(buttons) != 1 {
			t.Fatalf("card %d: expected 1 button, got %v", i, buttons)
		}
		text, _ := buttons[0]["text"].(string)
		// Signed values contain a `.` separating payload and HMAC.
		if !strings.Contains(text, ".") {
			t.Errorf("card %d button text is not signed: %q", i, text)
		}
		// Verify the signature is valid by round-tripping through the signer.
		payload, err := signer.Verify(text)
		if err != nil {
			t.Errorf("card %d signed value did not verify: %v", i, err)
			continue
		}
		if payload.NotificationID != "notif-7" || payload.AppID != "app-1" {
			t.Errorf("card %d payload identity: %+v", i, payload)
		}
		expectedTarget := "https://shop/p/sku-" + string(rune('1'+i))
		if payload.TargetURL != expectedTarget {
			t.Errorf("card %d target_url: got %q want %q", i, payload.TargetURL, expectedTarget)
		}
	}
}

// TestPreview_ReturnsAuthoringJSON sanity-checks the preview path so the UI
// can render a side-by-side without hitting Meta.
func TestPreview_ReturnsAuthoringJSON(t *testing.T) {
	svc, _, _, cleanup := newServiceWithMockMeta(t, `{"id":"x","status":"PENDING"}`, http.StatusOK)
	defer cleanup()

	tpl, err := svc.Create(context.Background(), &whatsapp.RichTemplate{
		Name: "coupon_drop", AppID: "app-1", Language: "en_US", Category: "MARKETING",
		Kind: whatsapp.KindCouponCode, Body: "Use code {{1}}", CouponCode: "DEAL50",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	preview, err := svc.Preview(context.Background(), "app-1", tpl.ID, map[string]string{"1": "DEAL50"})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	b, _ := json.Marshal(preview)
	wire := string(b)
	for _, needle := range []string{
		`"name":"coupon_drop"`,
		`"category":"MARKETING"`,
		`"type":"BUTTONS"`,
		`"_preview_variables"`,
	} {
		if !strings.Contains(wire, needle) {
			t.Errorf("preview missing %q\npreview=%s", needle, wire)
		}
	}
}
