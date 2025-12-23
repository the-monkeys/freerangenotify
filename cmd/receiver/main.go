package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	port := flag.String("port", "8090", "Port to listen on")
	userID := flag.String("userid", "", "Authorized User ID for this instance")
	secret := flag.String("secret", "my-secret-key", "Webhook signing secret")
	hubURL := flag.String("hub", "http://localhost:8080", "FreeRangeNotify Hub URL")
	apiKey := flag.String("apikey", "", "API Key for Hub authentication")
	flag.Parse()

	if *userID == "" {
		log.Println("Warning: No --userid provided. Receiver will accept notifications for any user (Security Disabled).")
	}

	// 1. Perform Check-in on startup
	if *apiKey != "" {
		dynamicURL := fmt.Sprintf("http://host.docker.internal:%s/webhook", *port)
		log.Printf("Checking-in to Hub at %s for user %s...", *hubURL, *userID)
		checkIn(*hubURL, *apiKey, *userID, dynamicURL)
	} else {
		log.Println("Warning: No --apikey provided. Skipping check-in. Hub won't know we are active.")
	}

	// 2. Setup Webhook Handler
	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

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
			mac := hmac.New(sha256.New, []byte(*secret))
			mac.Write(body)
			expectedSignature := hex.EncodeToString(mac.Sum(nil))

			if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
				log.Printf("Invalid signature: expected %s, got %s", expectedSignature, signature)
				http.Error(w, "Invalid Signature", http.StatusUnauthorized)
				return
			}
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Printf("Error parsing JSON: %v", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		receivedUserID, _ := payload["user_id"].(string)

		// Security: Validate targeting
		if *userID != "" && receivedUserID != *userID {
			log.Printf("SECURITY WARNING: Blocked notification for unauthorized user: %s (Authorized: %s)", receivedUserID, *userID)
			http.Error(w, "Unauthorized user", http.StatusForbidden)
			return
		}

		log.Printf("SUCCESS: Received notification for user %s", receivedUserID)
		log.Printf("  ID: %v | Title: %v", payload["id"], payload["content"].(map[string]interface{})["title"])

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Delivered")
	})

	log.Printf("Receiver listening on :%s", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

func checkIn(hubURL, apiKey, userID, dynamicURL string) {
	payload := map[string]string{
		"user_id":     userID,
		"dynamic_url": dynamicURL,
	}
	data, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", hubURL+"/v1/presence/check-in", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Check-in request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Check-in failed with status %d: %s", resp.StatusCode, string(body))
	} else {
		log.Printf("Check-in SUCCESSFUL for user %s", userID)
	}
}
