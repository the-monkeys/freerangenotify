# WhatsApp Rich & Interactive Messaging — Implementation Plan

**Scope**: Carousel templates, Multi-Product Messages, Single/Catalog Product Messages, CTA-URL with click attribution, Quick-Reply / List interactivity with inbound routing — across **Meta Cloud API** and **Twilio Content API** in parity.

**Status of repo today** (verified):

- Meta path (`internal/infrastructure/providers/meta_whatsapp_provider.go`): supports `whatsapp_template`, `whatsapp_interactive` (button/list/cta_url), media, location, contacts, reactions. **No carousel, no product messages, no flows.**
- Twilio path (`internal/infrastructure/providers/whatsapp_provider.go`): supports `content_sid` + `content_variables` pass-through. **No template authoring, no carousel/catalog helpers, no approval-status sync.**
- Template authoring: `whatsapp_template_handler.go` proxies `POST /v1/whatsapp/templates` straight to Meta with `body` as `map[string]interface{}` — no schema validation, no Twilio counterpart, no carousel-specific helpers.
- Webhooks: Meta inbound webhook handler exists (`meta_webhook_handler.go`); no Twilio Content-status webhook listener; click attribution for CTA-URL is not implemented.
- Domain: `notification.Content.Data` is the bag for `whatsapp_*` blobs — typed structs do not exist.

The plan brings both providers to feature parity for the Snapdeal-style swipeable image carousel with a "Shop now" deep link and full delivery + click telemetry.

---

## 1. Goals

1. Send Meta **Carousel Templates** (up to 10 cards, image header + body + 1–2 buttons per card) via Cloud API.
2. Send Twilio **`twilio/carousel`** content templates with the same logical payload.
3. Send **Multi-Product Messages (MPM)** and **Single Product Messages (SPM)** sourced from a Meta Commerce Catalog, with optional Twilio `twilio/catalog` parity.
4. Provide **typed authoring APIs** (`POST /v1/whatsapp/rich-templates`) that:
   - Validate cards/buttons/variables against WhatsApp limits before submission.
   - Submit to Meta `message_templates` and/or Twilio `Content` simultaneously (whichever provider is configured for the app).
   - Persist template metadata in Elasticsearch with **per-provider SIDs** + approval status.
5. Provide **runtime rendering** that picks the active provider and substitutes variables (product name, image URL, price, deep-link URL).
6. Deliver **click attribution** via signed redirect URLs (`/v1/r/{sig}`) for CTA-URL buttons so analytics close the loop on "Buy now → product page".
7. Wire **inbound webhooks** for quick-reply/list-reply/button payloads back into the FRN inbox + workflows.
8. UI authoring screen for carousels and product messages.

Non-goals: Flows (deferred to Phase 2 — separate plan), commerce catalog management (we read existing catalogs only), in-app emulation of WhatsApp UI.

---

## 2. WhatsApp Platform Capability Matrix

| Feature                       | Meta Cloud API                         | Twilio Content API                      | Notes |
| ----------------------------- | -------------------------------------- | --------------------------------------- | ----- |
| Carousel template             | `template` w/ `components.cards[]`     | `twilio/carousel`                       | Up to 10 cards, image/video header, 1–2 buttons/card |
| Single Product Message (SPM)  | `interactive.type=product`             | `twilio/catalog` (single mode)          | Requires catalog_id + product_retailer_id |
| Multi-Product Message (MPM)   | `interactive.type=product_list`        | `twilio/catalog` (multi mode)           | Up to 30 items in ≤10 sections |
| Catalog Message               | `interactive.type=catalog_message`     | `twilio/catalog`                        | Opens full catalog |
| CTA URL button                | `interactive.type=cta_url` (live msg)  | `twilio/call-to-action`                 | Already supported in Meta path; no Twilio helper yet |
| Quick Reply buttons           | `interactive.type=button`              | `twilio/quick-reply`                    | Already in Meta path |
| List picker                   | `interactive.type=list`                | `twilio/list-picker`                    | Already in Meta path |
| Coupon-code button (template) | `template` w/ COPY_CODE button         | `twilio/card`                           | New |
| Authentication OTP template   | `template` AUTHENTICATION category     | `twilio/authentication-code`            | Out of scope here |
| Flows                         | `interactive.type=flow`                | `twilio/flows`                          | Phase 2 |

**Hard constraints (enforced by validator, §5.3):**

- Carousel: 2–10 cards; all cards must share the same media type (all image **or** all video); each card body ≤ 160 chars; 1–2 buttons per card; buttons of the same type across all cards.
- Variables: `{{1}}`, `{{2}}` … positional. Carousel cards have card-scoped variables.
- All marketing categories require **template approval** before send. CTA-URL inside a non-template `interactive` message is the only zero-approval interactive surface (still requires session window or template wrapper).
- Twilio carousel cards are limited to image headers (no video) at time of writing — validator must downgrade or reject when targeting Twilio.

---

## 3. Domain Model

### 3.1 New typed structs — `internal/domain/whatsapp/rich.go`

```go
package whatsapp

type RichTemplateKind string

const (
    KindCarousel       RichTemplateKind = "carousel"
    KindSingleProduct  RichTemplateKind = "single_product"
    KindMultiProduct   RichTemplateKind = "multi_product"
    KindCatalog        RichTemplateKind = "catalog"
    KindCTAURL         RichTemplateKind = "cta_url"
    KindQuickReply     RichTemplateKind = "quick_reply"
    KindList           RichTemplateKind = "list"
    KindCouponCode     RichTemplateKind = "coupon_code"
)

type RichTemplate struct {
    ID            string             `json:"id"`            // FRN-internal ID
    AppID         string             `json:"app_id"`
    TenantID      string             `json:"tenant_id"`
    Name          string             `json:"name"`          // snake_case, unique per app
    Kind          RichTemplateKind   `json:"kind"`
    Language      string             `json:"language"`      // e.g. en_US
    Category      string             `json:"category"`      // MARKETING | UTILITY | AUTHENTICATION

    // Authoring payload (provider-agnostic)
    Body          *TextComponent     `json:"body,omitempty"`
    Header        *HeaderComponent   `json:"header,omitempty"`
    Footer        *TextComponent     `json:"footer,omitempty"`
    Cards         []CarouselCard     `json:"cards,omitempty"`        // for carousel
    Buttons       []Button           `json:"buttons,omitempty"`      // for non-carousel
    Catalog       *CatalogRef        `json:"catalog,omitempty"`      // for product/catalog
    Products      []ProductSection   `json:"products,omitempty"`     // for MPM

    // Provider linkage (filled in after submission)
    Providers     ProviderBindings   `json:"providers"`
    ApprovalState ApprovalState      `json:"approval_state"`

    CreatedAt, UpdatedAt time.Time
}

type CarouselCard struct {
    HeaderImageURL string   `json:"header_image_url,omitempty"`
    HeaderVideoURL string   `json:"header_video_url,omitempty"`
    Body           string   `json:"body"`           // ≤160 chars
    Variables      []string `json:"variables"`      // names matching {{1}}..{{n}}
    Buttons        []Button `json:"buttons"`        // 1..2
}

type Button struct {
    Type        ButtonType `json:"type"`        // QUICK_REPLY | URL | PHONE_NUMBER | COPY_CODE
    Text        string     `json:"text"`        // display label
    URL         string     `json:"url,omitempty"`         // for URL; supports {{1}} suffix
    Payload     string     `json:"payload,omitempty"`     // for QUICK_REPLY (returned in inbound webhook)
    PhoneNumber string     `json:"phone_number,omitempty"`
    Example     string     `json:"example,omitempty"`     // required by Meta for URL with variables
    TrackClicks bool       `json:"track_clicks,omitempty"` // wraps URL in /v1/r/{sig}
}

type ProviderBindings struct {
    Meta   *MetaBinding   `json:"meta,omitempty"`
    Twilio *TwilioBinding `json:"twilio,omitempty"`
}

type MetaBinding struct {
    TemplateName string `json:"template_name"`
    TemplateID   string `json:"template_id"`
    Status       string `json:"status"` // APPROVED | PENDING | REJECTED | DISABLED
    Reason       string `json:"reason,omitempty"`
}

type TwilioBinding struct {
    ContentSid   string `json:"content_sid"`
    ApprovalSid  string `json:"approval_sid,omitempty"`
    Status       string `json:"status"` // approved | pending | rejected | unsubmitted
    Reason       string `json:"reason,omitempty"`
}
```

### 3.2 Index (Elasticsearch) — `whatsapp_rich_templates_v1`

Aliased to `whatsapp_rich_templates`. Reuses the index-version pattern in `elasticsearch/index-65/`. Keyed by `id`, with a unique constraint per `(app_id, name)` enforced by a SHA1 routing key.

### 3.3 Send-time payload extension

`notification.Content.Data` gains one new key, structurally validated:

```jsonc
{
  "whatsapp_rich": {
    "template_id": "frn_tpl_01HP...",     // resolved server-side to Meta name OR Twilio ContentSid
    "variables": { "1": "Trendy Polo", "2": "₹229.00", "3": "https://snap.deal/p/123" },
    "cards": [                             // optional per-card overrides for carousel
      { "variables": { "1": "Trendy Polo", "2": "₹229.00" }, "header_image_url": "https://cdn..." },
      { "variables": { "1": "Athleisure Polo", "2": "₹260.00" }, "header_image_url": "https://cdn..." }
    ]
  }
}
```

The provider router resolves `template_id → provider-specific binding` based on the app's WhatsApp config (`provider == "meta" | "twilio"`).

The legacy `content_sid` / `whatsapp_interactive` keys remain supported for back-compat; new code paths funnel through `whatsapp_rich`.

---

## 4. Provider Implementation

### 4.1 Meta Cloud API — `meta_whatsapp_provider.go`

Add types:

```go
type metaCard struct {
    Components []metaComponent `json:"components"`
}

type metaCarouselComponent struct {
    Type  string     `json:"type"` // "carousel"
    Cards []metaCard `json:"cards"`
}
```

Update `metaTemplate` to allow a typed `Components` already in place; rendering function `buildCarouselTemplate(rich, vars)` produces:

```jsonc
{
  "messaging_product": "whatsapp",
  "to": "...",
  "type": "template",
  "template": {
    "name": "trendy_styles_carousel",
    "language": { "code": "en_US" },
    "components": [
      { "type": "BODY", "parameters": [{ "type": "text", "text": "User" }] },
      {
        "type": "carousel",
        "cards": [
          { "card_index": 0, "components": [
              { "type": "HEADER", "parameters": [{ "type": "image", "image": { "link": "https://..." }}] },
              { "type": "BODY",   "parameters": [{ "type": "text", "text": "Trendy Polo" }, { "type": "text", "text": "₹229.00" }] },
              { "type": "BUTTON", "sub_type": "URL", "index": "0",
                "parameters": [{ "type": "text", "text": "p/12345" }] }
          ]},
          { "card_index": 1, "components": [...] }
        ]
      }
    ]
  }
}
```

Add product/catalog renderers:

```go
func (p *MetaWhatsAppProvider) buildSingleProduct(...) metaMessage  // interactive.type = "product"
func (p *MetaWhatsAppProvider) buildMultiProduct(...) metaMessage   // interactive.type = "product_list"
func (p *MetaWhatsAppProvider) buildCatalogMessage(...) metaMessage // interactive.type = "catalog_message"
```

Dispatch order in `buildMessage`:

1. `whatsapp_rich` (new, typed) — preferred.
2. Existing `whatsapp_template`, `whatsapp_interactive`, media keys (back-compat).
3. Plain text/media fallback.

### 4.2 Twilio Content API — `whatsapp_provider.go`

Today the provider only forwards `content_sid`. Add **render-time SID resolution**:

```go
if rich, ok := notif.Content.Data["whatsapp_rich"].(map[string]interface{}); ok {
    binding, vars, err := p.resolveTwilioBinding(ctx, rich, notif.AppID)
    if err != nil { return NewErrorResult(err, ErrorTypeConfiguration), nil }
    data.Set("ContentSid", binding.ContentSid)
    if len(vars) > 0 {
        b, _ := json.Marshal(vars)
        data.Set("ContentVariables", string(b))
    }
}
```

`resolveTwilioBinding` calls a new `RichTemplateRepository.GetByID` and maps `RichTemplate.Providers.Twilio.ContentSid` + builds the positional `{{1..n}}` map by flattening card variables (Twilio carousels use `<card_index>.<position>` notation, e.g. `"1.1": "Trendy Polo"`).

### 4.3 New service — `internal/usecases/services/whatsapp_rich_template_service.go`

```go
type WhatsAppRichTemplateService interface {
    Create(ctx, tpl *RichTemplate) (*RichTemplate, error)   // submits to Meta + Twilio in parallel
    Get(ctx, appID, id string) (*RichTemplate, error)
    List(ctx, appID string, filter ListFilter) ([]RichTemplate, error)
    Update(ctx, id string, patch UpdatePatch) (*RichTemplate, error)  // resubmits new version
    Delete(ctx, id string) error
    SyncApproval(ctx, id string) error   // pulls latest status from both providers
}
```

Submission pipeline:

1. Validate (§5.3).
2. Translate authoring DTO → Meta `message_templates` JSON; POST to Graph API.
3. Translate authoring DTO → Twilio Content JSON; `POST https://content.twilio.com/v1/Content`, then `POST /v1/Content/{Sid}/ApprovalRequests/whatsapp` (category, content_type=carousel/etc).
4. Persist `RichTemplate` with both bindings; return early if either provider is not configured for the app.
5. On error, persist the partial template with `ApprovalState=PartiallySubmitted` so the user can retry.

### 4.4 Approval status sync

- Meta: subscribed via existing `meta_webhook_handler` (`message_template_status_update` field) → calls `RichTemplateService.applyMetaStatus`.
- Twilio: poll-based + optional **Twilio Content webhook** (new endpoint `POST /v1/webhooks/twilio/content-status`) authenticated by Twilio signature. Background sync job runs every 5 min for templates in `PENDING` ≥ 1 h.

---

## 5. API Surface

### 5.1 Authoring

```
POST   /v1/whatsapp/rich-templates           # create (idempotent on (app_id, name))
GET    /v1/whatsapp/rich-templates
GET    /v1/whatsapp/rich-templates/:id
PATCH  /v1/whatsapp/rich-templates/:id
DELETE /v1/whatsapp/rich-templates/:id
POST   /v1/whatsapp/rich-templates/:id/sync  # force approval-status refresh
POST   /v1/whatsapp/rich-templates/:id/preview { user_id, variables } # dry-run render against both providers
```

All routed through the protected (`app-key + tenant`) group in `internal/interfaces/http/routes/routes.go`. Existing `whatsapp_template_handler.go` deprecated for new clients but retained for raw Meta passthrough (mark `Deprecated: true` in OpenAPI).

### 5.2 Send

Existing `POST /v1/notifications` keeps its shape; clients send:

```jsonc
{
  "app_id": "app-123",
  "user_id": "u-internal-uuid",
  "channel": "whatsapp",
  "data": {
    "whatsapp_rich": {
      "template_id": "frn_tpl_01HP...",
      "cards": [
        { "variables": { "1": "Trendy Polo", "2": "₹229.00" }, "header_image_url": "https://cdn/.../1.jpg" },
        { "variables": { "1": "Athleisure Polo", "2": "₹260.00" }, "header_image_url": "https://cdn/.../2.jpg" }
      ]
    }
  }
}
```

`SendRequest.Validate` in `internal/domain/notification/models.go` (already special-cases `content_sid`) is extended to also accept `whatsapp_rich.template_id` as a substitute for `TemplateID`.

### 5.3 Validation rules — `internal/domain/whatsapp/validate.go`

Reject before hitting Meta/Twilio (cheaper feedback loop):

- `Kind=carousel`: 2 ≤ len(Cards) ≤ 10; uniform header type; each card.body ≤ 160; 1 ≤ buttons/card ≤ 2; mixed button types across cards prohibited; required `Example` for URL buttons containing `{{n}}`.
- `Kind=multi_product`: ≤ 10 sections, ≤ 30 total items; each `product_retailer_id` non-empty.
- `Variables`: contiguous indices starting at 1; no missing positions; each value ≤ 1024 chars; reject newlines unless explicitly allowed.
- Twilio-specific downgrade: if `header_video_url` set on any card and Twilio binding is requested, fail with `WHATSAPP_TWILIO_VIDEO_HEADER_UNSUPPORTED`.

---

## 6. Click Attribution & Deep Links

For URL buttons with `track_clicks=true`:

1. At send time, the renderer wraps the resolved URL: `https://{frn_host}/v1/r/{tenant}/{sig}` where `sig` is HMAC-SHA256 over `(notification_id, button_index, target_url, exp)`.
2. New handler `internal/interfaces/http/handlers/redirect_handler.go` GET `/v1/r/:sig`: validates HMAC, records click event (`notification.click`) into Elasticsearch + emits SSE, then 302 to the target URL.
3. Click events feed the existing analytics pipeline (`internal/domain/analytics`).

This is the only way to attribute "Buy now" clicks since Meta does **not** report URL-button clicks back to the webhook — only quick-reply payloads.

---

## 7. Inbound Routing (Quick-Reply / List / Button)

Already partly handled in `meta_webhook_handler.go` for Meta. Extend:

- Add Twilio inbound webhook handler at `POST /v1/webhooks/twilio/whatsapp` (validates `X-Twilio-Signature`).
- Both handlers normalize into a single `whatsapp.InboundMessage` shape with a `ButtonPayload` field. The existing `whatsapp_service_impl.go` route table (RouteWebhookForward, etc.) consumes it.
- Quick-reply payloads carry `Button.Payload`, which we surface in the inbox UI and as a workflow trigger (`workflow.event = whatsapp.button_clicked`).

---

## 8. UI

`ui/` (React/Vite) gains:

- **Rich Template Builder** page (`/whatsapp/rich-templates/new`): card editor with image upload, body/buttons, variable preview pane, side-by-side Meta vs Twilio render preview, submit button.
- **Approval Status Board**: lists templates with per-provider status pills (Meta, Twilio) and re-sync action.
- **Quick Send → Carousel**: a launcher that pulls user audience + selects an approved rich template.

All endpoints are reachable via the existing `/v1` proxy through `notification-service`. No new gateway plumbing.

---

## 9. Testing Strategy

| Layer            | Test                                                                                          |
| ---------------- | --------------------------------------------------------------------------------------------- |
| Domain validator | `whatsapp/validate_test.go` — boundary tests for cards, buttons, variables                    |
| Meta renderer    | `meta_whatsapp_provider_test.go` — golden JSON fixtures per `RichTemplateKind`                |
| Twilio renderer  | `whatsapp_provider_test.go` — golden form-encoded payload per kind                            |
| Service          | Mock both APIs (httpmock) for `Create`/`SyncApproval` paths; assert partial-submission state  |
| HTTP             | Integration test `tests/whatsapp_rich_e2e_test.go` covering author → submit (mocked) → send → click → analytics |
| Click attribution| `redirect_handler_test.go` — HMAC signature validation, replay rejection, expiry              |
| UI               | Playwright spec under `e2e/whatsapp-carousel.spec.ts`                                         |

CI gate: `make test-unit` must pass; integration covered under `make test-integration` (already wired in `Makefile`).

---

## 10. Security & Compliance

- HMAC verification on **every** inbound webhook (`X-Hub-Signature-256` for Meta, `X-Twilio-Signature` for Twilio).
- Tenant scoping: every rich-template query filtered by `tenant_id` + `app_id`; reuse the existing RBAC middleware.
- Per-app credentials only; never log access tokens or signatures (extend the existing redaction in `pkg/logging`).
- Click-redirect URLs MUST expire (default 30 days, configurable per app) and use constant-time HMAC comparison.
- Marketing category enforcement: validator rejects `category=MARKETING` when the app's WhatsApp config has `marketing_opt_in_required=true` and the recipient lacks an opt-in record.
- Rate-limit `POST /v1/whatsapp/rich-templates` (5 req/min/tenant) — Meta penalises rapid template churn.

---

## 11. Rollout Plan

| Phase  | Deliverable                                                                                 | Flag                                  |
| ------ | ------------------------------------------------------------------------------------------- | ------------------------------------- |
| **0**  | Domain types, validator, ES index, repository                                                | none — additive                       |
| **1**  | Meta renderer for `carousel`, `cta_url`, `coupon_code`; service.Create + Meta submission     | `whatsapp_rich.meta_enabled=true`     |
| **2**  | Twilio renderer + Content API submission + status sync                                       | `whatsapp_rich.twilio_enabled=true`   |
| **3**  | Click-attribution redirect + analytics events                                                 | `whatsapp_rich.click_tracking=true`   |
| **4**  | Multi-Product, Single Product, Catalog                                                        | `whatsapp_rich.commerce_enabled=true` |
| **5**  | UI builder + Playwright coverage                                                              | UI feature flag                       |
| **6**  | Inbound button-click → workflow trigger                                                       | `workflows.whatsapp_buttons=true`     |

Each phase ships behind a feature flag in `internal/config`. Phases 0–2 are mandatory for the Snapdeal-style use case; the rest are additive.

---

## 12. Open Questions

1. Do we standardize on Meta-native carousel templates and treat Twilio as fallback, or maintain strict parity? Recommendation: **parity, but Meta-first** — Twilio submission failure is non-fatal during Create.
2. Where do hosted card images live? Recommendation: tenant-supplied CDN URL only — FRN does not host media.
3. Catalog ID per app vs per template? Recommendation: per-app default with per-template override (commerce teams rarely run more than one catalog).
4. Should we expose the redirect signer key per-app (so tenants can verify clicks themselves)? Recommendation: yes, expose a `GET /v1/apps/:id/redirect-pubkey` after Phase 3.

---

## 13. File-Level Change List

New files:

- `internal/domain/whatsapp/rich.go`
- `internal/domain/whatsapp/validate.go`
- `internal/usecases/services/whatsapp_rich_template_service.go`
- `internal/infrastructure/repository/whatsapp_rich_template_repo.go`
- `internal/interfaces/http/handlers/whatsapp_rich_template_handler.go`
- `internal/interfaces/http/handlers/redirect_handler.go`
- `internal/interfaces/http/handlers/twilio_webhook_handler.go`
- `elasticsearch/index-66/whatsapp_rich_templates.json` (next index version)
- `ui/src/pages/whatsapp/RichTemplateBuilder.tsx`
- `ui/src/pages/whatsapp/RichTemplateList.tsx`
- `e2e/whatsapp-carousel.spec.ts`

Modified files:

- `internal/infrastructure/providers/meta_whatsapp_provider.go` — add carousel/product renderers + `whatsapp_rich` dispatch.
- `internal/infrastructure/providers/whatsapp_provider.go` — add `whatsapp_rich` resolver + Twilio carousel variable flattening.
- `internal/domain/notification/models.go` — accept `whatsapp_rich.template_id` in `Validate`.
- `internal/interfaces/http/routes/routes.go` — register new handlers + Twilio webhook + redirect route.
- `internal/container/container.go` — wire repository, service, handlers.
- `internal/interfaces/http/handlers/meta_webhook_handler.go` — forward template-status events to `RichTemplateService.applyMetaStatus`.
- `docs/openapi/*.yaml` — document new endpoints.
- `documents/API_DOCUMENTATION.md` — append rich-templates section.

---

## 14. Definition of Done

- A tenant with both Meta and Twilio configured can `POST /v1/whatsapp/rich-templates` once and have the carousel rendered identically on both providers (verified by golden fixtures).
- Sending a notification with `whatsapp_rich.template_id` to an opted-in user delivers a swipeable carousel with image + price + "Shop now" deep link.
- Clicking "Shop now" redirects through `/v1/r/{sig}`, records a `notification.click` event, and lands on the product page.
- Approval status changes from Meta or Twilio are reflected in the FRN UI within 5 minutes (webhook) or 5 minutes (poll fallback).
- All tests under `make test` pass; OpenAPI bundle regenerates cleanly.
