package handlers

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/filestore"
	apierrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (h *ApplicationHandler) SetCodeSampleBaseURL(raw string) error {
	if err := filestore.ValidatePublicURL(raw); err != nil {
		return err
	}
	if raw == "" {
		h.codeSampleBaseURL = "YOUR_BASE_URL/v1"
		return nil
	}
	h.codeSampleBaseURL = strings.TrimSuffix(strings.TrimRight(raw, "/"), "/v1") + "/v1"
	return nil
}

type integrationSnippet struct {
	Title string `json:"title"`
	Code  string `json:"code"`
}
type integrationChannelSamples struct {
	Sender   integrationSnippet  `json:"sender"`
	Receiver *integrationSnippet `json:"receiver,omitempty"`
	Auth     *integrationSnippet `json:"auth,omitempty"`
}

// GetCodeSamples returns backend examples with placeholder credentials only.
// @Summary Get integration code samples
// @Description Requires an account JWT and application ownership or team access. Returned credentials are placeholders.
// @Tags Applications
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps/{id}/code-samples [get]
func (h *ApplicationHandler) GetCodeSamples(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		return apierrors.Unauthorized("User not authenticated")
	}
	appID := c.Params("id")
	if appID == "" {
		return apierrors.BadRequest("app_id is required")
	}
	if _, _, err := h.authorizeAppAccess(c, appID, userID); err != nil {
		return err
	}
	base := h.codeSampleBaseURL
	if base == "" {
		base = "YOUR_BASE_URL/v1"
	}
	samples := map[string]map[string]integrationChannelSamples{}
	for _, lang := range []string{"go", "python", "javascript", "java", "cpp", "rust", "ruby"} {
		channels := map[string]integrationChannelSamples{}
		for _, channel := range []string{"sse", "email", "sms", "whatsapp", "webhook"} {
			payload, _ := json.Marshal(map[string]interface{}{"user_id": "YOUR_USER_ID", "template_id": "YOUR_TEMPLATE_ID", "channel": channel, "data": map[string]string{"name": "Alice"}})
			item := integrationChannelSamples{Sender: integrationSnippet{Title: "Backend: queue " + channel + " notification", Code: integrationHTTP(lang, base+"/notifications", string(payload))}}
			if channel == "sse" {
				item.Auth = &integrationSnippet{Title: "Backend: issue a short-lived SSE token", Code: integrationHTTP(lang, base+"/sse/tokens", `{"user_id":"YOUR_USER_ID"}`)}
				if lang == "javascript" {
					item.Receiver = &integrationSnippet{Title: "Browser: use only your backend-issued token", Code: "// Get sseToken from your authenticated backend. Never embed an app key.\nconst url = new URL(" + jsonLiteral(base+"/sse") + ");\nurl.searchParams.set('sse_token', sseToken);\nconst stream = new EventSource(url);\nstream.addEventListener('notification', (event) => {\n  console.log(JSON.parse(event.data));\n});\n// Close on logout; obtain a fresh token after expiry.\n// stream.close();"}
				}
			}
			channels[channel] = item
		}
		samples[lang] = channels
	}
	return c.JSON(fiber.Map{"success": true, "data": samples})
}
func jsonLiteral(s string) string { raw, _ := json.Marshal(s); return string(raw) }

func integrationHTTP(lang, url, payload string) string {
	var code string
	switch lang {
	case "go":
		code = `package main

import (
 "bytes"
 "fmt"
 "io"
 "net/http"
 "time"
)

func main() {
 request, err := http.NewRequest("POST", BASE_LITERAL, bytes.NewBufferString(PAYLOAD_LITERAL))
 if err != nil { panic(err) }
 request.Header.Set("Authorization", "Bearer YOUR_API_KEY")
 request.Header.Set("Content-Type", "application/json")
 client := &http.Client{Timeout: 30 * time.Second}
 response, err := client.Do(request)
 if err != nil { panic(err) }
 defer response.Body.Close()
 body, err := io.ReadAll(response.Body)
 if err != nil { panic(err) }
 if response.StatusCode < 200 || response.StatusCode >= 300 { panic(fmt.Sprintf("HTTP %d: %s", response.StatusCode, body)) }
 fmt.Println(string(body))
}`
	case "python":
		code = `import requests

# Backend only. Install requests in your application environment.
response = requests.post(BASE_LITERAL,
    data=PAYLOAD_LITERAL,
    headers={"Authorization": "Bearer YOUR_API_KEY", "Content-Type": "application/json"},
    timeout=30)
response.raise_for_status()
print(response.json())`
	case "javascript":
		code = `// Backend Node.js with built-in fetch; keep the app key out of browsers.
async function main() {
  const response = await fetch(BASE_LITERAL, {
    method: 'POST',
    headers: { Authorization: 'Bearer YOUR_API_KEY', 'Content-Type': 'application/json' },
    body: PAYLOAD_LITERAL,
    signal: AbortSignal.timeout(30000),
  });
  if (!response.ok) throw new Error('HTTP ' + response.status);
  console.log(await response.json());
}
main().catch(console.error);`
	case "java":
		code = `// Java 11+. Backend only.
import java.net.URI;
import java.net.http.*;
import java.time.Duration;

class Main {
  public static void main(String[] args) throws Exception {
    HttpRequest request = HttpRequest.newBuilder(URI.create(BASE_LITERAL))
      .timeout(Duration.ofSeconds(30))
      .header("Authorization", "Bearer YOUR_API_KEY")
      .header("Content-Type", "application/json")
      .POST(HttpRequest.BodyPublishers.ofString(PAYLOAD_LITERAL)).build();
    HttpResponse<String> response = HttpClient.newHttpClient().send(request, HttpResponse.BodyHandlers.ofString());
    if (response.statusCode() < 200 || response.statusCode() >= 300) throw new IllegalStateException("HTTP " + response.statusCode());
    System.out.println(response.body());
  }
}`
	case "cpp":
		code = `// Link with libcurl. Backend only.
#include <curl/curl.h>
#include <iostream>

int main() {
  CURL* client = curl_easy_init();
  if (!client) return 1;
  curl_slist* headers = nullptr;
  headers = curl_slist_append(headers, "Authorization: Bearer YOUR_API_KEY");
  headers = curl_slist_append(headers, "Content-Type: application/json");
  curl_easy_setopt(client, CURLOPT_URL, BASE_LITERAL);
  curl_easy_setopt(client, CURLOPT_HTTPHEADER, headers);
  curl_easy_setopt(client, CURLOPT_POSTFIELDS, PAYLOAD_LITERAL);
  curl_easy_setopt(client, CURLOPT_TIMEOUT, 30L);
  CURLcode result = curl_easy_perform(client);
  long status = 0;
  curl_easy_getinfo(client, CURLINFO_RESPONSE_CODE, &status);
  curl_slist_free_all(headers);
  curl_easy_cleanup(client);
  if (result != CURLE_OK || status < 200 || status >= 300) { std::cerr << "Request failed: " << status; return 1; }
  return 0;
}`
	case "rust":
		code = `// Dependencies: reqwest, tokio (macros + runtime). Backend only.
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
  let response = reqwest::Client::new().post(BASE_LITERAL)
    .header("Authorization", "Bearer YOUR_API_KEY")
    .header("Content-Type", "application/json")
    .timeout(std::time::Duration::from_secs(30))
    .body(PAYLOAD_LITERAL).send().await?.error_for_status()?;
  println!("{}", response.text().await?);
  Ok(())
}`
	case "ruby":
		code = `require 'net/http'
require 'uri'

# Backend only.
uri = URI(BASE_LITERAL)
request = Net::HTTP::Post.new(uri)
request['Authorization'] = 'Bearer YOUR_API_KEY'
request['Content-Type'] = 'application/json'
request.body = PAYLOAD_LITERAL
response = Net::HTTP.start(uri.host, uri.port, use_ssl: uri.scheme == 'https', open_timeout: 30, read_timeout: 30) { |http| http.request(request) }
raise "HTTP #{response.code}" unless response.is_a?(Net::HTTPSuccess)
puts response.body`
	}
	quote := jsonLiteral
	if lang == "go" || lang == "cpp" {
		quote = strconv.Quote
	}
	if lang == "rust" {
		quote = func(value string) string { return rustUnicodeEscape.ReplaceAllString(jsonLiteral(value), `\u{$1}`) }
	}
	// Ruby interpolates #{...} even within ordinary double-quoted literals.
	if lang == "ruby" {
		quote = func(s string) string { return strings.ReplaceAll(jsonLiteral(s), "#{", `\#{`) }
	}
	return strings.NewReplacer("BASE_LITERAL", quote(url), "PAYLOAD_LITERAL", quote(payload)).Replace(code)
}

var rustUnicodeEscape = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)
