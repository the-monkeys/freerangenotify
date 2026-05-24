package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
)

// WhatsAppRichTemplateService is the authoring/management surface for the
// rich-template store. Sits in front of the repository, the validator, and
// the upstream provider authoring APIs (Meta Cloud API today; Twilio Content
// API in Phase 2).
type WhatsAppRichTemplateService interface {
	Create(ctx context.Context, tpl *whatsapp.RichTemplate) (*whatsapp.RichTemplate, error)
	Get(ctx context.Context, appID, id string) (*whatsapp.RichTemplate, error)
	GetByName(ctx context.Context, appID, name string) (*whatsapp.RichTemplate, error)
	List(ctx context.Context, filter whatsapp.RichTemplateFilter) ([]*whatsapp.RichTemplate, int64, error)
	Delete(ctx context.Context, appID, id string) error
	SyncApproval(ctx context.Context, appID, id string) (*whatsapp.RichTemplate, error)
	Preview(ctx context.Context, appID, id string, variables map[string]string) (map[string]interface{}, error)
	// ApplyMetaStatus is invoked by the Meta webhook handler when a
	// message_template_status_update event fires. It locates the template by
	// the Meta-side template_name + WABA ID and updates its MetaBinding.
	ApplyMetaStatus(ctx context.Context, wabaID, templateName, status, reason string) error
	// ApplyTwilioStatus is invoked by the Twilio content-status webhook
	// handler. Locates the template by Twilio ContentSid and updates its
	// TwilioBinding.
	ApplyTwilioStatus(ctx context.Context, contentSid, status, reason string) error
	// ResolveSendPayload turns an FRN-internal SendPayload (template_id +
	// variables + per-card runtime data) into the wire-shape
	// `whatsapp_rich` map the Meta provider expects at send-time. URL
	// buttons with track_clicks=true are wrapped with the signed redirect
	// URL so taps land on /v1/r/:sig and emit a clicked analytics event.
	ResolveSendPayload(ctx context.Context, appID, notificationID string, payload whatsapp.SendPayload) (map[string]interface{}, error)
	// ResolveTwilioSendPayload turns an FRN-internal SendPayload into the
	// wire-shape the Twilio WhatsApp provider expects: a `content_sid` and
	// a flat positional `content_variables` map. Carousel per-card overrides
	// are emitted using Twilio's dotted-key convention (`<card+1>.<pos>`).
	// Returns ErrTemplateNotConfigured when the template has no Twilio binding.
	ResolveTwilioSendPayload(ctx context.Context, appID, notificationID string, payload whatsapp.SendPayload) (map[string]interface{}, error)
	// EnableClickTracking wires the click-attribution signer + public URL
	// after the service is constructed (since the signer is built later
	// in container.go). Calling with signer==nil disables wrapping.
	EnableClickTracking(signer *whatsapp.ClickSigner, publicURL string)
}

// httpDoer is the small subset of *http.Client we depend on so tests can
// substitute an httptest.Server-backed client without ceremony.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type whatsappRichTemplateService struct {
	repo          whatsapp.RichTemplateRepository
	appRepo       application.Repository
	httpClient    httpDoer
	apiVersion    string
	metaAppID     string // FRN's Facebook App ID, required for Resumable Upload init
	metaAppSecret string // FRN's Facebook App Secret, required for Resumable Upload init
	logger        *zap.Logger
	now           func() time.Time
	clickSigner   *whatsapp.ClickSigner // optional; nil disables click-attribution wrapping
	publicURL     string                // base URL for /v1/r/{sig}; required when clickSigner is set
}

// EnableClickTracking sets the click-attribution signer + public URL. The
// signer and URL are wired in container.go after both this service and the
// signer are constructed; we keep the dependency optional so unit tests
// can run without the signer.
func (s *whatsappRichTemplateService) EnableClickTracking(signer *whatsapp.ClickSigner, publicURL string) {
	s.clickSigner = signer
	s.publicURL = publicURL
}

// NewWhatsAppRichTemplateService wires up the service. apiVersion defaults
// to v23.0 when empty; tests typically inject httptest.Server.Client() and
// can override `now` for deterministic CreatedAt assertions.
func NewWhatsAppRichTemplateService(
	repo whatsapp.RichTemplateRepository,
	appRepo application.Repository,
	httpClient httpDoer,
	apiVersion string,
	metaAppID, metaAppSecret string,
	logger *zap.Logger,
) WhatsAppRichTemplateService {
	if apiVersion == "" {
		apiVersion = "v23.0"
	}
	if httpClient == nil {
		// Media upload requires bigger window than the default 15s; image
		// downloads + Meta byte-upload can take several seconds on a slow
		// link. 60s gives carousels with multiple media headers headroom.
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &whatsappRichTemplateService{
		repo:          repo,
		appRepo:       appRepo,
		httpClient:    httpClient,
		apiVersion:    apiVersion,
		metaAppID:     metaAppID,
		metaAppSecret: metaAppSecret,
		logger:        logger,
		now:           func() time.Time { return time.Now().UTC() },
	}
}

// Create is the main authoring path: validate → stamp identity → submit to
// every configured provider → persist with the resulting bindings.
//
// Per the plan §11 rollout matrix, Meta submission failures during Create
// are returned as errors so the UI can show the user why their template was
// rejected before the document is ever indexed. Twilio submission is
// best-effort once Phase 2 ships — that part will move to a deferred path.
func (s *whatsappRichTemplateService) Create(ctx context.Context, tpl *whatsapp.RichTemplate) (*whatsapp.RichTemplate, error) {
	if tpl == nil {
		return nil, fmt.Errorf("template is nil")
	}
	if errs := whatsapp.Validate(tpl); !errs.IsEmpty() {
		return nil, errs
	}
	if tpl.AppID == "" {
		return nil, fmt.Errorf("app_id is required")
	}

	// Reject duplicate (app_id, name) at this layer so the user sees a clear
	// 409 instead of a confusing index conflict deeper in ES.
	existing, err := s.repo.GetByName(ctx, tpl.AppID, tpl.Name)
	if err != nil {
		return nil, fmt.Errorf("lookup existing template: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("rich template %q already exists for app %s", tpl.Name, tpl.AppID)
	}

	app, err := s.appRepo.GetByID(ctx, tpl.AppID)
	if err != nil {
		return nil, fmt.Errorf("load app %s: %w", tpl.AppID, err)
	}
	wa := app.Settings.WhatsApp
	metaConfigured := wa != nil && wa.Provider == "meta" && wa.MetaAccessToken != "" && wa.MetaWABAID != ""
	// Twilio Content API is submitted whenever Twilio creds are present on
	// the app, regardless of the runtime Provider field — so an app can run
	// Meta in prod and keep Twilio Content SIDs warm as a failover.
	twilioConfigured := wa != nil && wa.AccountSID != "" && wa.AuthToken != ""

	tpl.ID = "frn_tpl_" + uuid.NewString()
	now := s.now()
	tpl.CreatedAt = now
	tpl.UpdatedAt = now
	tpl.ApprovalState = whatsapp.ApprovalDraft
	if tpl.Providers.Meta == nil {
		tpl.Providers.Meta = &whatsapp.MetaBinding{}
	}

	// Lists are pure runtime interactive messages (no Meta template
	// equivalent), so persist them as Draft without a submission round-trip.
	// Products/MPM/Catalog are gated separately above by the validator.
	skipMetaSubmission := tpl.Kind == whatsapp.KindList

	if metaConfigured && !skipMetaSubmission {
		// Meta carousel + media-header templates require an `header_handle`
		// produced by the Resumable Upload API. Passing the raw URL returns
		// error 100/2388215 ("Invalid parameter"). Upload first, then
		// submit a payload copy with URLs swapped for handles.
		submitTpl := tpl
		if needsMediaHandles(tpl) {
			handles, uErr := s.uploadCarouselHeaderMedia(ctx, tpl, wa.MetaAccessToken)
			if uErr != nil {
				return nil, fmt.Errorf("meta media upload failed: %w", uErr)
			}
			submitTpl = applyMediaHandles(tpl, handles)
		}
		metaBinding, err := s.submitToMeta(ctx, submitTpl, wa)
		if err != nil {
			return nil, fmt.Errorf("meta submission failed: %w", err)
		}
		tpl.Providers.Meta = metaBinding
	} else if skipMetaSubmission {
		// Runtime-only kinds don't need approval. Mark them approved so the
		// UI surfaces them as ready-to-send.
		tpl.ApprovalState = whatsapp.ApprovalApproved
		s.logger.Info("Rich template created (runtime-only kind; no Meta submission)",
			zap.String("kind", string(tpl.Kind)),
			zap.String("name", tpl.Name))
	} else {
		// No Meta configured — keep ApprovalDraft so the user knows the
		// template was stored but not yet shipped anywhere.
		s.logger.Info("Rich template created without Meta submission (no Meta config on app)",
			zap.String("app_id", tpl.AppID),
			zap.String("name", tpl.Name))
	}

	// Best-effort Twilio submission: a failure here is logged but does not
	// fail the whole Create. The Meta binding (when present) is the
	// authoritative one for users running Meta as their active provider; the
	// Twilio binding is a parity bonus.
	if twilioConfigured && !skipMetaSubmission {
		twBinding, err := s.submitToTwilio(ctx, tpl, wa)
		if err != nil {
			s.logger.Warn("Twilio Content API submission failed (template still persisted with Meta binding)",
				zap.String("template", tpl.Name),
				zap.Error(err))
		} else {
			tpl.Providers.Twilio = twBinding
		}
	}

	tpl.ApprovalState = aggregateApprovalState(tpl.Providers)
	if err := s.repo.Create(ctx, tpl); err != nil {
		return nil, fmt.Errorf("persist template: %w", err)
	}
	return tpl, nil
}

// Get fetches a template by ID and enforces the app scope so cross-tenant
// reads are rejected at the service boundary.
func (s *whatsappRichTemplateService) Get(ctx context.Context, appID, id string) (*whatsapp.RichTemplate, error) {
	tpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tpl.AppID != appID {
		return nil, fmt.Errorf("rich template %s does not belong to app %s", id, appID)
	}
	return tpl, nil
}

func (s *whatsappRichTemplateService) GetByName(ctx context.Context, appID, name string) (*whatsapp.RichTemplate, error) {
	tpl, err := s.repo.GetByName(ctx, appID, name)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, fmt.Errorf("rich template %q not found", name)
	}
	return tpl, nil
}

func (s *whatsappRichTemplateService) List(ctx context.Context, filter whatsapp.RichTemplateFilter) ([]*whatsapp.RichTemplate, int64, error) {
	return s.repo.List(ctx, filter)
}

// Delete removes the FRN record but does not currently delete the upstream
// Meta template — that requires confirming the user really wants the
// approved template removed from their WABA, which the UI handles via a
// separate confirmation flow. We log the orphan so it can be reconciled.
func (s *whatsappRichTemplateService) Delete(ctx context.Context, appID, id string) error {
	tpl, err := s.Get(ctx, appID, id)
	if err != nil {
		return err
	}
	if tpl.Providers.Meta != nil && tpl.Providers.Meta.TemplateName != "" {
		s.logger.Warn("Deleting rich template; Meta template not removed from WABA",
			zap.String("id", id),
			zap.String("meta_template_name", tpl.Providers.Meta.TemplateName))
	}
	return s.repo.Delete(ctx, id)
}

// SyncApproval pulls the latest Meta template status from the Graph API and
// updates the binding + aggregate ApprovalState. The polling path; the
// webhook path is ApplyMetaStatus.
func (s *whatsappRichTemplateService) SyncApproval(ctx context.Context, appID, id string) (*whatsapp.RichTemplate, error) {
	tpl, err := s.Get(ctx, appID, id)
	if err != nil {
		return nil, err
	}
	if tpl.Providers.Meta == nil || tpl.Providers.Meta.TemplateName == "" {
		return tpl, nil // never submitted; nothing to sync
	}
	app, err := s.appRepo.GetByID(ctx, appID)
	if err != nil {
		return nil, err
	}
	wa := app.Settings.WhatsApp
	if wa == nil || wa.Provider != "meta" || wa.MetaAccessToken == "" || wa.MetaWABAID == "" {
		return tpl, nil
	}

	status, reason, err := s.fetchMetaStatus(ctx, tpl.Providers.Meta.TemplateName, wa)
	if err != nil {
		return nil, err
	}
	tpl.Providers.Meta.Status = status
	tpl.Providers.Meta.Reason = reason
	tpl.Providers.Meta.UpdatedAt = s.now()
	tpl.ApprovalState = aggregateApprovalState(tpl.Providers)
	tpl.UpdatedAt = s.now()
	if err := s.repo.Update(ctx, tpl); err != nil {
		return nil, err
	}
	return tpl, nil
}

// Preview returns the Meta-side authoring JSON the service would submit for
// a given template. Useful for UI side-by-side previews and debugging
// without round-tripping to Graph API. Variable substitution is applied to
// example values so the preview reflects what the recipient would see.
func (s *whatsappRichTemplateService) Preview(ctx context.Context, appID, id string, variables map[string]string) (map[string]interface{}, error) {
	tpl, err := s.Get(ctx, appID, id)
	if err != nil {
		return nil, err
	}
	payload := metaAuthoringPayload(tpl)
	if len(variables) > 0 {
		payload["_preview_variables"] = variables // surface for the UI side-by-side renderer
	}
	return payload, nil
}

// ApplyMetaStatus is invoked from the meta webhook handler so async approval
// transitions land in the index without polling. Locates the template by
// (WABA → app_id → name).
func (s *whatsappRichTemplateService) ApplyMetaStatus(ctx context.Context, wabaID, templateName, status, reason string) error {
	apps, err := s.appRepo.List(ctx, application.ApplicationFilter{Limit: 500})
	if err != nil {
		return err
	}
	var appID string
	for _, app := range apps {
		wa := app.Settings.WhatsApp
		if wa != nil && wa.Provider == "meta" && wa.MetaWABAID == wabaID {
			appID = app.AppID
			break
		}
	}
	if appID == "" {
		s.logger.Warn("Template status webhook for unknown WABA",
			zap.String("waba_id", wabaID),
			zap.String("template_name", templateName))
		return nil
	}

	// Find by Meta-side template_name (the name we submitted, which mirrors
	// tpl.Name today but may diverge in future for collision avoidance).
	tpl, err := s.repo.GetByName(ctx, appID, templateName)
	if err != nil {
		return err
	}
	if tpl == nil {
		// Template was created outside FRN (raw Graph API) — nothing to do.
		return nil
	}
	if tpl.Providers.Meta == nil {
		tpl.Providers.Meta = &whatsapp.MetaBinding{TemplateName: templateName}
	}
	tpl.Providers.Meta.Status = strings.ToUpper(status)
	tpl.Providers.Meta.Reason = reason
	tpl.Providers.Meta.UpdatedAt = s.now()
	tpl.ApprovalState = aggregateApprovalState(tpl.Providers)
	tpl.UpdatedAt = s.now()
	return s.repo.Update(ctx, tpl)
}

// ApplyTwilioStatus is the webhook-driven counterpart to ApplyMetaStatus.
// Looks up by Twilio ContentSid (which we stored at submission time) and
// updates the binding + aggregate state.
func (s *whatsappRichTemplateService) ApplyTwilioStatus(ctx context.Context, contentSid, status, reason string) error {
	// We don't have a "GetByTwilioSid" repo method, so do a small scan with
	// a high cap. Twilio webhook volume is low (one per approval transition
	// per template); the simpler implementation is fine here.
	const scanCap = 1000
	apps, err := s.appRepo.List(ctx, application.ApplicationFilter{Limit: 500})
	if err != nil {
		return err
	}
	for _, app := range apps {
		tpls, _, err := s.repo.List(ctx, whatsapp.RichTemplateFilter{AppID: app.AppID, Limit: scanCap})
		if err != nil {
			s.logger.Warn("Twilio status webhook: list templates failed", zap.String("app_id", app.AppID), zap.Error(err))
			continue
		}
		for _, tpl := range tpls {
			if tpl.Providers.Twilio == nil || tpl.Providers.Twilio.ContentSid != contentSid {
				continue
			}
			tpl.Providers.Twilio.Status = strings.ToLower(status)
			tpl.Providers.Twilio.Reason = reason
			tpl.Providers.Twilio.UpdatedAt = s.now()
			tpl.ApprovalState = aggregateApprovalState(tpl.Providers)
			tpl.UpdatedAt = s.now()
			return s.repo.Update(ctx, tpl)
		}
	}
	s.logger.Warn("Twilio status webhook for unknown ContentSid", zap.String("content_sid", contentSid))
	return nil
}

// ResolveSendPayload loads the template referenced by SendPayload.TemplateID
// and produces the wire-format `whatsapp_rich` map the Meta provider expects
// at send-time. Per-button click-attribution wrapping happens here so the
// provider stays purely a formatter.
//
// For carousel templates, per-card runtime data (header media overrides,
// per-card variables, per-card button values) is interleaved into the wire
// format. Missing per-card data falls back to the authored template values.
func (s *whatsappRichTemplateService) ResolveSendPayload(ctx context.Context, appID, notificationID string, payload whatsapp.SendPayload) (map[string]interface{}, error) {
	if payload.TemplateID == "" {
		return nil, fmt.Errorf("send payload missing template_id")
	}
	tpl, err := s.Get(ctx, appID, payload.TemplateID)
	if err != nil {
		return nil, err
	}
	if tpl.Providers.Meta == nil || tpl.Providers.Meta.TemplateName == "" {
		return nil, fmt.Errorf("template %s has no Meta binding (not submitted)", payload.TemplateID)
	}

	rich := map[string]interface{}{
		"kind":          string(tpl.Kind),
		"template_name": tpl.Providers.Meta.TemplateName,
		"language":      tpl.Language,
	}

	// Top-level BODY variables in template order (1..n).
	if vars := orderedVariableValues(tpl.Body, payload.Variables); len(vars) > 0 {
		rich["body_variables"] = vars
	}

	switch tpl.Kind {
	case whatsapp.KindCarousel:
		rich["cards"] = s.resolveCarouselCards(tpl, payload, notificationID)
	case whatsapp.KindCouponCode:
		if tpl.CouponCode != "" {
			rich["coupon_code"] = tpl.CouponCode
		}
	}
	return rich, nil
}

// resolveCarouselCards merges authored cards with per-card runtime overrides
// and produces the wire-format `cards` array. Each card's button values are
// signed via the click signer when track_clicks=true.
func (s *whatsappRichTemplateService) resolveCarouselCards(tpl *whatsapp.RichTemplate, payload whatsapp.SendPayload, notificationID string) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(tpl.Cards))
	for i, card := range tpl.Cards {
		var override whatsapp.SendPayloadCard
		if i < len(payload.Cards) {
			override = payload.Cards[i]
		}
		wireCard := map[string]interface{}{}
		// Header media: runtime override > authored URL.
		switch {
		case override.HeaderImageURL != "":
			wireCard["header_image_url"] = override.HeaderImageURL
		case override.HeaderVideoURL != "":
			wireCard["header_video_url"] = override.HeaderVideoURL
		case card.HeaderImageURL != "":
			wireCard["header_image_url"] = card.HeaderImageURL
		case card.HeaderVideoURL != "":
			wireCard["header_video_url"] = card.HeaderVideoURL
		}
		// Body variables: runtime override > template-default.
		vars := orderedVariableValues(card.Body, override.Variables)
		if len(vars) > 0 {
			wireCard["body_variables"] = vars
		}
		// Buttons in order. URL buttons with track_clicks → signed redirect.
		buttons := make([]map[string]interface{}, 0, len(card.Buttons))
		for j, b := range card.Buttons {
			value := ""
			if j < len(override.ButtonValues) {
				value = override.ButtonValues[j]
			}
			buttons = append(buttons, s.resolveButton(b, j, value, notificationID, tpl.AppID))
		}
		wireCard["buttons"] = buttons
		out = append(out, wireCard)
	}
	return out
}

// resolveButton produces the wire-shape for one button. For URL buttons
// with TrackClicks=true and a non-empty runtime value, the value is wrapped
// in a signed redirect URL. The value supplied by the caller is treated as
// the path tail substituted into the authored URL pattern.
func (s *whatsappRichTemplateService) resolveButton(b whatsapp.Button, index int, runtimeValue, notificationID, appID string) map[string]interface{} {
	out := map[string]interface{}{
		"sub_type": string(b.Type),
		"text":     runtimeValue,
	}
	switch b.Type {
	case whatsapp.ButtonURL:
		if b.TrackClicks && s.clickSigner != nil && s.publicURL != "" && runtimeValue != "" {
			target := b.URL
			// Substitute {{1}} (typical URL variable position) so the
			// click event lands on the real destination.
			if strings.Contains(target, "{{1}}") {
				target = strings.ReplaceAll(target, "{{1}}", runtimeValue)
			} else if !strings.HasSuffix(target, runtimeValue) {
				// No variable position: append runtimeValue as a path suffix.
				if !strings.HasSuffix(target, "/") {
					target += "/"
				}
				target += runtimeValue
			}
			signed, err := s.clickSigner.Sign(whatsapp.ClickPayload{
				NotificationID: notificationID,
				AppID:          appID,
				ButtonIndex:    index,
				TargetURL:      target,
			})
			if err == nil {
				// Pass the sig as the runtime value — the authored template
				// URL must be `{public_url}/v1/r/{{1}}` for this to work.
				out["text"] = signed
				out["tracked_target"] = target // for analytics correlation
			}
		}
	case whatsapp.ButtonQuickReply:
		out["payload"] = b.Payload
		delete(out, "text")
	case whatsapp.ButtonCopyCode:
		out["coupon_code"] = b.CouponCode
		delete(out, "text")
	}
	return out
}

// orderedVariableValues returns the variable values in the order they
// appear in `body`. Caller's `vars` map keys are stringified positional
// indices ("1", "2", ...). Missing values stay empty so the receiver sees
// the literal placeholder — that is preferable to a runtime error for the
// common "user didn't supply var" case.
func orderedVariableValues(body string, vars map[string]string) []string {
	matches := varRE.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	max := 0
	for _, m := range matches {
		n := 0
		for _, ch := range m[1] {
			n = n*10 + int(ch-'0')
		}
		if n > max {
			max = n
		}
	}
	out := make([]string, max)
	for i := 0; i < max; i++ {
		out[i] = vars[fmt.Sprintf("%d", i+1)]
	}
	return out
}

// --- Twilio Content API ---

// submitToTwilio submits the template to the Twilio Content API and then
// requests WhatsApp approval. Returns the TwilioBinding populated with the
// content_sid and approval_sid.
func (s *whatsappRichTemplateService) submitToTwilio(ctx context.Context, tpl *whatsapp.RichTemplate, wa *application.WhatsAppAppConfig) (*whatsapp.TwilioBinding, error) {
	payload := twilioContentPayload(tpl)
	body, _ := json.Marshal(payload)

	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, twilioContentBaseURL+"/Content", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	createReq.Header.Set("Content-Type", "application/json")
	createReq.SetBasicAuth(wa.AccountSID, wa.AuthToken)

	resp, err := s.httpClient.Do(createReq)
	if err != nil {
		return nil, fmt.Errorf("twilio create content: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("twilio create content returned status %d: %s", resp.StatusCode, string(respBody))
	}
	var created struct {
		SID string `json:"sid"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return nil, fmt.Errorf("decode twilio content create: %w", err)
	}
	if created.SID == "" {
		return nil, fmt.Errorf("twilio create content returned no SID")
	}

	// Submit for WhatsApp approval. category is required and mirrors Meta.
	approvalBody, _ := json.Marshal(map[string]interface{}{
		"name":     tpl.Name,
		"category": strings.ToLower(tpl.Category),
	})
	approvalReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/Content/%s/ApprovalRequests/whatsapp", twilioContentBaseURL, created.SID),
		bytes.NewReader(approvalBody))
	if err != nil {
		return nil, err
	}
	approvalReq.Header.Set("Content-Type", "application/json")
	approvalReq.SetBasicAuth(wa.AccountSID, wa.AuthToken)
	approvalResp, err := s.httpClient.Do(approvalReq)
	if err != nil {
		return nil, fmt.Errorf("twilio approval request: %w", err)
	}
	defer approvalResp.Body.Close()
	approvalRespBody, _ := io.ReadAll(approvalResp.Body)
	if approvalResp.StatusCode < 200 || approvalResp.StatusCode >= 300 {
		// Approval request failed but content was created — still bind the
		// SID so we can retry approval later via SyncApproval.
		s.logger.Warn("Twilio approval request failed; content sid bound for retry",
			zap.String("content_sid", created.SID),
			zap.Int("status", approvalResp.StatusCode),
			zap.String("body", string(approvalRespBody)))
		now := s.now()
		return &whatsapp.TwilioBinding{
			ContentSid:  created.SID,
			Status:      "unsubmitted",
			Reason:      string(approvalRespBody),
			SubmittedAt: now,
			UpdatedAt:   now,
		}, nil
	}
	var approval struct {
		Status string `json:"status"`
		SID    string `json:"sid"`
	}
	_ = json.Unmarshal(approvalRespBody, &approval)
	now := s.now()
	status := strings.ToLower(approval.Status)
	if status == "" {
		status = "pending"
	}
	return &whatsapp.TwilioBinding{
		ContentSid:  created.SID,
		ApprovalSid: approval.SID,
		Status:      status,
		SubmittedAt: now,
		UpdatedAt:   now,
	}, nil
}

// twilioContentBaseURL is exposed as a package variable so tests can
// rewrite it to point at an httptest server. Default is Twilio's production
// Content API endpoint.
var twilioContentBaseURL = "https://content.twilio.com/v1"

// --- Meta authoring API ---

// submitToMeta translates the typed RichTemplate to Meta's
// message_templates authoring JSON and POSTs it to Graph API. Returns the
// populated MetaBinding (template ID, initial status).
func (s *whatsappRichTemplateService) submitToMeta(ctx context.Context, tpl *whatsapp.RichTemplate, wa *application.WhatsAppAppConfig) (*whatsapp.MetaBinding, error) {
	payload := metaAuthoringPayload(tpl)
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/message_templates", s.apiVersion, wa.MetaWABAID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+wa.MetaAccessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meta http request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseMetaError(resp.StatusCode, respBody)
	}

	var parsed struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Category string `json:"category"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode meta response: %w (body=%s)", err, string(respBody))
	}

	now := s.now()
	return &whatsapp.MetaBinding{
		TemplateName: tpl.Name,
		TemplateID:   parsed.ID,
		Status:       strings.ToUpper(parsed.Status),
		SubmittedAt:  now,
		UpdatedAt:    now,
	}, nil
}

// fetchMetaStatus polls Graph API for the current status of an already-
// submitted template. Used by SyncApproval.
func (s *whatsappRichTemplateService) fetchMetaStatus(ctx context.Context, templateName string, wa *application.WhatsAppAppConfig) (string, string, error) {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/message_templates?name=%s&access_token=%s",
		s.apiVersion, wa.MetaWABAID, templateName, wa.MetaAccessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", parseMetaError(resp.StatusCode, body)
	}
	var parsed struct {
		Data []struct {
			Status string `json:"status"`
			Reason string `json:"rejection_reason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", fmt.Errorf("decode meta status list: %w", err)
	}
	if len(parsed.Data) == 0 {
		return "", "", fmt.Errorf("template %q not found on meta", templateName)
	}
	return strings.ToUpper(parsed.Data[0].Status), parsed.Data[0].Reason, nil
}

// parseMetaError pulls the structured error out of a Meta response so the
// caller surfaces a useful message instead of "got 400".
func parseMetaError(status int, body []byte) error {
	var e struct {
		Error struct {
			Message   string `json:"message"`
			Type      string `json:"type"`
			Code      int    `json:"code"`
			Subcode   int    `json:"error_subcode"`
			FBTraceID string `json:"fbtrace_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &e); err != nil || e.Error.Message == "" {
		return fmt.Errorf("meta api returned status %d: %s", status, string(body))
	}
	return fmt.Errorf("meta error %d/%d (%s): %s [trace %s]",
		e.Error.Code, e.Error.Subcode, e.Error.Type, e.Error.Message, e.Error.FBTraceID)
}

// aggregateApprovalState rolls per-provider status into the public
// ApprovalState. Approved-by-any wins; otherwise pending dominates; only
// when both providers reject do we report Rejected; only when no provider
// was even tried do we keep Draft.
func aggregateApprovalState(p whatsapp.ProviderBindings) whatsapp.ApprovalState {
	metaState := providerState(p.Meta)
	twilioState := providerState(p.Twilio)

	if metaState == approvedHint || twilioState == approvedHint {
		return whatsapp.ApprovalApproved
	}
	if metaState == disabledHint || twilioState == disabledHint {
		return whatsapp.ApprovalDisabled
	}
	if metaState == pendingHint || twilioState == pendingHint {
		return whatsapp.ApprovalPending
	}
	if metaState == rejectedHint && twilioState == rejectedHint {
		return whatsapp.ApprovalRejected
	}
	if metaState == rejectedHint && twilioState == unknownHint {
		return whatsapp.ApprovalRejected
	}
	if metaState == unknownHint && twilioState == unknownHint {
		return whatsapp.ApprovalDraft
	}
	return whatsapp.ApprovalPartiallySubmitted
}

type providerStateHint int

const (
	unknownHint providerStateHint = iota
	pendingHint
	approvedHint
	rejectedHint
	disabledHint
)

// providerState reduces the per-provider raw status (which is provider-
// specific casing) to one of the hint constants the aggregator uses.
func providerState(b interface{}) providerStateHint {
	var status string
	switch v := b.(type) {
	case *whatsapp.MetaBinding:
		if v == nil {
			return unknownHint
		}
		status = strings.ToLower(v.Status)
	case *whatsapp.TwilioBinding:
		if v == nil {
			return unknownHint
		}
		status = strings.ToLower(v.Status)
	}
	switch status {
	case "approved":
		return approvedHint
	case "pending":
		return pendingHint
	case "rejected":
		return rejectedHint
	case "disabled":
		return disabledHint
	default:
		return unknownHint
	}
}

// ErrTemplateNotConfigured is returned when an operation requires a Meta
// binding that does not exist yet (e.g. SyncApproval on a draft template).
var ErrTemplateNotConfigured = errors.New("template is not configured for the requested provider")

// ResolveTwilioSendPayload loads the template referenced by
// SendPayload.TemplateID and produces the wire-format the Twilio WhatsApp
// provider expects:
//
//	{
//	  "content_sid": "HX...",
//	  "content_variables": { "1": "Asha", "2": "...", "1.1": "Trendy Polo", ... }
//	}
//
// `content_variables` is a flat positional map. Top-level body variables
// occupy keys "1".."n" verbatim from payload.Variables. For carousel
// templates, per-card overrides supplied in payload.Cards are emitted with
// Twilio's dotted-key convention: card index i (1-based) + "." + position.
//
// Returns ErrTemplateNotConfigured when the template has no Twilio binding.
func (s *whatsappRichTemplateService) ResolveTwilioSendPayload(ctx context.Context, appID, notificationID string, payload whatsapp.SendPayload) (map[string]interface{}, error) {
	if payload.TemplateID == "" {
		return nil, fmt.Errorf("send payload missing template_id")
	}
	tpl, err := s.Get(ctx, appID, payload.TemplateID)
	if err != nil {
		return nil, err
	}
	if tpl.Providers.Twilio == nil || tpl.Providers.Twilio.ContentSid == "" {
		return nil, ErrTemplateNotConfigured
	}

	vars := map[string]string{}
	for k, v := range payload.Variables {
		vars[k] = v
	}

	// Carousel: per-card variables get dotted keys ("1.1", "1.2", "2.1", ...).
	// The signed-redirect wrapping applies to URL buttons with TrackClicks,
	// emitted as additional dotted positional values when the authored
	// template uses {{n}} in a card button's URL.
	if tpl.Kind == whatsapp.KindCarousel {
		for i, card := range payload.Cards {
			cardIdx := i + 1
			for k, v := range card.Variables {
				vars[fmt.Sprintf("%d.%s", cardIdx, k)] = v
			}
			// Per-card button values (in order) — Twilio carousels typically
			// expose the URL suffix variable as the last positional slot of
			// each card. Click-attribution wrapping uses the same signer as
			// the Meta path so analytics are consistent across providers.
			if i < len(tpl.Cards) {
				authoredCard := tpl.Cards[i]
				for j, b := range authoredCard.Buttons {
					var runtimeValue string
					if j < len(card.ButtonValues) {
						runtimeValue = card.ButtonValues[j]
					}
					if runtimeValue == "" {
						continue
					}
					if b.Type == whatsapp.ButtonURL && b.TrackClicks && s.clickSigner != nil && s.publicURL != "" {
						target := b.URL
						if strings.Contains(target, "{{1}}") {
							target = strings.ReplaceAll(target, "{{1}}", runtimeValue)
						} else if !strings.HasSuffix(target, runtimeValue) {
							if !strings.HasSuffix(target, "/") {
								target += "/"
							}
							target += runtimeValue
						}
						signed, sErr := s.clickSigner.Sign(whatsapp.ClickPayload{
							NotificationID: notificationID,
							AppID:          appID,
							ButtonIndex:    j,
							TargetURL:      target,
						})
						if sErr == nil {
							runtimeValue = signed
						}
					}
					// Position the button value after the card body variables.
					// Authoring convention: card body uses {{1}}..{{m}}, then
					// the button URL suffix uses {{m+1}}. We approximate by
					// placing the value at position (len(card.Variables)+j+1).
					pos := len(card.Variables) + j + 1
					vars[fmt.Sprintf("%d.%d", cardIdx, pos)] = runtimeValue
				}
			}
		}
	}

	out := map[string]interface{}{
		"content_sid": tpl.Providers.Twilio.ContentSid,
	}
	if len(vars) > 0 {
		// Twilio expects a JSON-encoded string for ContentVariables on the
		// wire, but the provider accepts a map[string]interface{} and
		// json-marshals it itself. Keep the typed shape here.
		contentVars := make(map[string]interface{}, len(vars))
		for k, v := range vars {
			contentVars[k] = v
		}
		out["content_variables"] = contentVars
	}
	return out, nil
}
