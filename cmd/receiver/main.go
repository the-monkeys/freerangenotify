package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		secret = "my-secret-key" // Default matching our test config
	}

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read Body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		// Verify Signature
		signature := r.Header.Get("X-Webhook-Signature")
		if signature != "" {
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expectedSignature := hex.EncodeToString(mac.Sum(nil))

			if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
				log.Printf("Invalid signature: expected %s, got %s", expectedSignature, signature)
				http.Error(w, "Invalid Signature", http.StatusUnauthorized)
				return
			}
			log.Printf("Signature verified successfully")
		} else {
			log.Printf("Warning: No signature header received")
		}

		// Parse Notification
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Printf("Error parsing JSON: %v", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Security: Validate Targeting
		// Simulate a list of users currently 'logged in' to this receiver system
		currentActiveUsers := map[string]bool{
			"ui-user-123": true, // Authorized UI User
			"ui-user-456": true, // Another Authorized User
		}

		receivedUserID, _ := payload["user_id"].(string)
		if !currentActiveUsers[receivedUserID] {
			log.Printf("SECURITY WARNING: Received notification for non-active user: %s", receivedUserID)
			log.Printf("  Blocking delivery to prevent data leakage.")
			http.Error(w, "User Not Authenticated on this Receiver", http.StatusForbidden)
			return
		}

		log.Printf("Received Webhook Notification for Active User: %s", receivedUserID)
		log.Printf("  ID: %v", payload["id"])
		log.Printf("  Title: %v", payload["content"].(map[string]interface{})["title"])
		log.Printf("  Body: %v", payload["content"].(map[string]interface{})["body"])

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Webhook received and delivered to user %s", receivedUserID)
	})

	log.Printf("Receiver app listening on :%s", port)
	log.Printf("Webhook URL: http://localhost:%s/webhook", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
