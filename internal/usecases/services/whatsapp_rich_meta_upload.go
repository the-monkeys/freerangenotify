package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
)

// uploadCarouselHeaderMedia uploads every distinct carousel-header media URL
// on `tpl` to Meta's Resumable Upload API and returns a url→handle map. The
// handle is what Meta's message_templates endpoint requires under
// `example.header_handle` — passing a raw URL there returns error
// 100/2388215 ("Invalid parameter").
//
// We upload at Create time (synchronously) because Meta requires the handle
// to exist *before* the template submission completes. Handles are cached
// per-URL inside the call so a 10-card carousel that reuses the same image
// twice only uploads it once.
//
// The flow is two HTTP calls per URL:
//
//  1. POST https://graph.facebook.com/{version}/{app_id}/uploads
//     ?file_length=N&file_type=image/jpeg
//     &access_token={app_id}|{app_secret}
//     → {"id":"upload:..."}
//
//  2. POST https://graph.facebook.com/{version}/{upload_session_id}
//     headers: Authorization: OAuth {user_access_token}, file_offset: 0
//     body:    raw bytes
//     → {"h":"4::aW1hZ2..."}
//
// metaAccessToken is the WABA's access token (system-user token), used for
// the bytes-upload step. metaAppID + metaAppSecret form the app access token
// used for the init step.
func (s *whatsappRichTemplateService) uploadCarouselHeaderMedia(
	ctx context.Context,
	tpl *whatsapp.RichTemplate,
	metaAccessToken string,
) (map[string]string, error) {
	// Walk the template and collect every URL that will need a handle.
	type urlSpec struct {
		url         string
		contentType string
	}
	specs := make([]urlSpec, 0, len(tpl.Cards)+1)
	seen := make(map[string]bool)
	push := func(u, ct string) {
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		specs = append(specs, urlSpec{url: u, contentType: ct})
	}

	if tpl.Header != nil {
		push(tpl.Header.ImageURL, guessImageContentType(tpl.Header.ImageURL))
		push(tpl.Header.VideoURL, guessVideoContentType(tpl.Header.VideoURL))
		push(tpl.Header.DocumentURL, "application/pdf")
	}
	for _, c := range tpl.Cards {
		push(c.HeaderImageURL, guessImageContentType(c.HeaderImageURL))
		push(c.HeaderVideoURL, guessVideoContentType(c.HeaderVideoURL))
	}

	if len(specs) == 0 {
		return nil, nil
	}

	if s.metaAppID == "" || s.metaAppSecret == "" {
		return nil, fmt.Errorf("meta app credentials not configured: cannot upload media handles required by Meta's carousel template API. " +
			"Set FREERANGE_PROVIDERS_META_WHATSAPP_META_APP_ID and FREERANGE_PROVIDERS_META_WHATSAPP_META_APP_SECRET in your env")
	}

	out := make(map[string]string, len(specs))
	for _, sp := range specs {
		handle, err := s.uploadOneMediaURL(ctx, sp.url, sp.contentType, metaAccessToken)
		if err != nil {
			return nil, fmt.Errorf("upload %s: %w", sp.url, err)
		}
		out[sp.url] = handle
	}
	return out, nil
}

// uploadOneMediaURL runs the two-step resumable upload for a single URL.
// Returns the `h` handle that should be passed to message_templates as
// example.header_handle[0].
func (s *whatsappRichTemplateService) uploadOneMediaURL(
	ctx context.Context,
	srcURL, contentType, userAccessToken string,
) (string, error) {
	// Step A: fetch the bytes into memory. Media is bounded by Meta's
	// per-asset limits (image ≤ 5 MB, video ≤ 16 MB, document ≤ 100 MB),
	// so the simple in-memory approach is fine here.
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return "", fmt.Errorf("build get request: %w", err)
	}
	getResp, err := s.httpClient.Do(getReq)
	if err != nil {
		return "", fmt.Errorf("download media: %w", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode < 200 || getResp.StatusCode >= 300 {
		return "", fmt.Errorf("download media returned status %d", getResp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(getResp.Body)
	if err != nil {
		return "", fmt.Errorf("read media bytes: %w", err)
	}
	if ct := getResp.Header.Get("Content-Type"); ct != "" && contentType == "" {
		contentType = ct
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	// Step B: initialise the upload session. The access_token here is the
	// APP token (app_id|app_secret), NOT the user/WABA access token.
	initURL := fmt.Sprintf(
		"https://graph.facebook.com/%s/%s/uploads?file_length=%d&file_type=%s&access_token=%s%%7C%s",
		s.apiVersion,
		s.metaAppID,
		len(bodyBytes),
		contentType,
		s.metaAppID,
		s.metaAppSecret,
	)
	initReq, err := http.NewRequestWithContext(ctx, http.MethodPost, initURL, nil)
	if err != nil {
		return "", fmt.Errorf("build init request: %w", err)
	}
	initResp, err := s.httpClient.Do(initReq)
	if err != nil {
		return "", fmt.Errorf("init upload session: %w", err)
	}
	defer initResp.Body.Close()
	initBody, _ := io.ReadAll(initResp.Body)
	if initResp.StatusCode < 200 || initResp.StatusCode >= 300 {
		return "", parseMetaError(initResp.StatusCode, initBody)
	}
	var initParsed struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(initBody, &initParsed); err != nil {
		return "", fmt.Errorf("decode init response: %w (body=%s)", err, string(initBody))
	}
	if initParsed.ID == "" {
		return "", fmt.Errorf("init returned empty session id (body=%s)", string(initBody))
	}

	// Step C: upload the bytes. Authorization header here is the USER
	// access token (OAuth scheme, not Bearer). file_offset starts at 0
	// because we're doing the whole thing in one request.
	uploadURL := fmt.Sprintf("https://graph.facebook.com/%s/%s", s.apiVersion, initParsed.ID)
	upReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("build upload request: %w", err)
	}
	upReq.Header.Set("Authorization", "OAuth "+userAccessToken)
	upReq.Header.Set("file_offset", "0")
	upReq.Header.Set("Content-Type", contentType)
	upReq.ContentLength = int64(len(bodyBytes))

	upResp, err := s.httpClient.Do(upReq)
	if err != nil {
		return "", fmt.Errorf("upload bytes: %w", err)
	}
	defer upResp.Body.Close()
	upBody, _ := io.ReadAll(upResp.Body)
	if upResp.StatusCode < 200 || upResp.StatusCode >= 300 {
		return "", parseMetaError(upResp.StatusCode, upBody)
	}
	var upParsed struct {
		H string `json:"h"`
	}
	if err := json.Unmarshal(upBody, &upParsed); err != nil {
		return "", fmt.Errorf("decode upload response: %w (body=%s)", err, string(upBody))
	}
	if upParsed.H == "" {
		return "", fmt.Errorf("upload returned empty handle (body=%s)", string(upBody))
	}
	return upParsed.H, nil
}

// applyMediaHandles returns a deep-ish copy of `tpl` with every header URL
// rewritten to its uploaded handle. The original `tpl` is not mutated so
// the caller can still persist the user-supplied URLs (handy for the UI to
// re-render previews and to re-upload if a re-submission is needed).
func applyMediaHandles(tpl *whatsapp.RichTemplate, handles map[string]string) *whatsapp.RichTemplate {
	if len(handles) == 0 {
		return tpl
	}
	clone := *tpl
	if tpl.Header != nil {
		hh := *tpl.Header
		if h, ok := handles[hh.ImageURL]; ok {
			hh.ImageURL = h
		}
		if h, ok := handles[hh.VideoURL]; ok {
			hh.VideoURL = h
		}
		if h, ok := handles[hh.DocumentURL]; ok {
			hh.DocumentURL = h
		}
		clone.Header = &hh
	}
	if len(tpl.Cards) > 0 {
		clone.Cards = make([]whatsapp.CarouselCard, len(tpl.Cards))
		for i, c := range tpl.Cards {
			cc := c
			if h, ok := handles[cc.HeaderImageURL]; ok {
				cc.HeaderImageURL = h
			}
			if h, ok := handles[cc.HeaderVideoURL]; ok {
				cc.HeaderVideoURL = h
			}
			clone.Cards[i] = cc
		}
	}
	return &clone
}

// guessImageContentType makes a best-effort guess at the MIME type from the
// URL extension. Meta's upload init endpoint requires a file_type, so we
// pick a sensible default rather than failing.
func guessImageContentType(u string) string {
	switch lowerExt(u) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	}
	return "image/jpeg"
}

func guessVideoContentType(u string) string {
	switch lowerExt(u) {
	case ".3gp":
		return "video/3gpp"
	case ".mov":
		return "video/quicktime"
	}
	return "video/mp4"
}

// needsMediaHandles reports whether the template carries any header media
// URL that Meta will require to be uploaded as a handle before submission.
// Coupon-code and carousel kinds with image/video headers all qualify;
// text-only templates do not.
func needsMediaHandles(tpl *whatsapp.RichTemplate) bool {
	if tpl == nil {
		return false
	}
	if tpl.Header != nil {
		if tpl.Header.ImageURL != "" || tpl.Header.VideoURL != "" || tpl.Header.DocumentURL != "" {
			return true
		}
	}
	for _, c := range tpl.Cards {
		if c.HeaderImageURL != "" || c.HeaderVideoURL != "" {
			return true
		}
	}
	return false
}

func lowerExt(u string) string {
	if u == "" {
		return ""
	}
	if i := strings.LastIndex(u, "?"); i >= 0 {
		u = u[:i]
	}
	if i := strings.LastIndex(u, "#"); i >= 0 {
		u = u[:i]
	}
	if i := strings.LastIndex(u, "."); i >= 0 {
		return strings.ToLower(u[i:])
	}
	return ""
}
