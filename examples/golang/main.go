// FreeRangeNotify API — Go Integration Example
//
// Prerequisites:
//   - Go 1.21+
//   - A FreeRangeNotify account with an application and API key
//
// Usage:
//
//	export FRN_API_KEY="frn_your_api_key_here"
//	go run main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Configuration
// ──────────────────────────────────────────────────────────────────────────────

const baseURL = "https://freerangenotify.monkeys.support/v1"

func apiKey() string {
	key := os.Getenv("FRN_API_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "Error: FRN_API_KEY environment variable is not set.")
		fmt.Fprintln(os.Stderr, "  export FRN_API_KEY=\"frn_your_api_key_here\"")
		os.Exit(1)
	}
	return key
}

// ──────────────────────────────────────────────────────────────────────────────
// HTTP helpers
// ──────────────────────────────────────────────────────────────────────────────

var client = &http.Client{Timeout: 30 * time.Second}

// doJSON sends a JSON request and prints the response.
func doJSON(method, url string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-Key", apiKey())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// prettyPrint formats JSON for display.
func prettyPrint(label string, data []byte) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		fmt.Printf("=== %s ===\n%s\n\n", label, string(data))
		return
	}
	fmt.Printf("=== %s ===\n%s\n\n", label, buf.String())
}

// ──────────────────────────────────────────────────────────────────────────────
// 1. Send a Notification
// ──────────────────────────────────────────────────────────────────────────────

func sendNotification() {
	fmt.Println("──── Send Notification ────")

	payload := map[string]interface{}{
		"user_id":     "user-uuid-or-external-id",
		"channel":     "email",
		"priority":    "normal",
		"template_id": "welcome-email",
		"data": map[string]interface{}{
			"name":    "Jane Doe",
			"company": "Acme Corp",
		},
	}

	resp, err := doJSON("POST", baseURL+"/notifications", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Send notification failed: %v\n", err)
		return
	}
	prettyPrint("Send Notification Response", resp)
}

// ──────────────────────────────────────────────────────────────────────────────
// 2. Send Bulk Notifications
// ──────────────────────────────────────────────────────────────────────────────

func sendBulkNotifications() {
	fmt.Println("──── Send Bulk Notifications ────")

	payload := map[string]interface{}{
		"user_ids":    []string{"user-1-uuid", "user-2-uuid", "user-3-uuid"},
		"channel":     "push",
		"priority":    "high",
		"template_id": "flash-sale",
		"data": map[string]interface{}{
			"discount": "25%",
			"expires":  "2026-06-10T00:00:00Z",
		},
	}

	resp, err := doJSON("POST", baseURL+"/notifications/bulk", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Bulk send failed: %v\n", err)
		return
	}
	prettyPrint("Bulk Send Response", resp)
}

// ──────────────────────────────────────────────────────────────────────────────
// 3. List Notifications
// ──────────────────────────────────────────────────────────────────────────────

func listNotifications() {
	fmt.Println("──── List Notifications ────")

	resp, err := doJSON("GET", baseURL+"/notifications?page=1&page_size=10", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "List notifications failed: %v\n", err)
		return
	}
	prettyPrint("List Notifications Response", resp)
}

// ──────────────────────────────────────────────────────────────────────────────
// 4. Send OTP
// ──────────────────────────────────────────────────────────────────────────────

func sendOTP() {
	fmt.Println("──── Send OTP ────")

	payload := map[string]interface{}{
		"channel":   "sms",
		"recipient": "+14155551234",
	}

	resp, err := doJSON("POST", baseURL+"/otp/send", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Send OTP failed: %v\n", err)
		return
	}
	prettyPrint("Send OTP Response", resp)
}

// ──────────────────────────────────────────────────────────────────────────────
// 5. Verify OTP
// ──────────────────────────────────────────────────────────────────────────────

func verifyOTP(requestID, code string) {
	fmt.Println("──── Verify OTP ────")

	payload := map[string]interface{}{
		"request_id": requestID,
		"code":       code,
	}

	resp, err := doJSON("POST", baseURL+"/otp/verify", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Verify OTP failed: %v\n", err)
		return
	}
	prettyPrint("Verify OTP Response", resp)
}

// ──────────────────────────────────────────────────────────────────────────────
// 6. Upload a File (Invoice)
// ──────────────────────────────────────────────────────────────────────────────

func uploadFile(filePath string) {
	fmt.Println("──── Upload File ────")

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Open file failed: %v\n", err)
		return
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Create form file failed: %v\n", err)
		return
	}
	if _, err := io.Copy(part, file); err != nil {
		fmt.Fprintf(os.Stderr, "Copy file data failed: %v\n", err)
		return
	}
	writer.Close()

	req, err := http.NewRequest("POST", baseURL+"/files", &buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Create request failed: %v\n", err)
		return
	}
	req.Header.Set("X-API-Key", apiKey())
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Upload request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	prettyPrint("Upload File Response", respBody)
}

// ──────────────────────────────────────────────────────────────────────────────
// 7. Quick Send (simplified endpoint)
// ──────────────────────────────────────────────────────────────────────────────

func quickSend() {
	fmt.Println("──── Quick Send ────")

	payload := map[string]interface{}{
		"to":       "jane@example.com",
		"channel":  "email",
		"subject":  "Your Invoice is Ready",
		"body":     "Hi Jane, your invoice #1234 is attached.",
		"priority": "normal",
	}

	resp, err := doJSON("POST", baseURL+"/quick-send", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Quick send failed: %v\n", err)
		return
	}
	prettyPrint("Quick Send Response", resp)
}

// ──────────────────────────────────────────────────────────────────────────────
// Main
// ──────────────────────────────────────────────────────────────────────────────

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║   FreeRangeNotify — Go Integration Examples             ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 1. Send a single notification
	sendNotification()

	// 2. Send bulk notifications
	sendBulkNotifications()

	// 3. List recent notifications
	listNotifications()

	// 4. Send an OTP via SMS
	sendOTP()

	// 5. Verify an OTP (replace with real values from step 4)
	verifyOTP("req_placeholder_id", "123456")

	// 6. Upload a file (uncomment and provide a real path)
	// uploadFile("/path/to/invoice.pdf")

	// 7. Quick send
	quickSend()
}
