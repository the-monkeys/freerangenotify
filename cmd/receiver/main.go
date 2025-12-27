package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Minimal Go receiver example: listens on /webhook, logs payload, optional presence check-in.
func main() {
	port := getenv("PORT", "8090")
	userID := getenv("USER_ID", "")
	apiKey := getenv("API_KEY", "")
	hubURL := getenv("HUB_URL", "http://localhost:8080")

	if apiKey != "" && userID != "" {
		dynamicURL := fmt.Sprintf("http://host.docker.internal:%s/webhook/0256d6f2a5524a83ba9a37025c40aa0b34f10258b43543a4994cb2d994392b0c3a871672fbb3496db5fffcbd00e3b7df", port)
		if err := checkIn(hubURL, apiKey, userID, dynamicURL); err != nil {
			log.Printf("check-in failed: %v", err)
		}
	} else {
		log.Println("check-in skipped (running in anonymous/passive mode)")
	}

	http.HandleFunc("/webhook/0256d6f2a5524a83ba9a37025c40aa0b34f10258b43543a4994cb2d994392b0c3a871672fbb3496db5fffcbd00e3b7df", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		log.Println("=== notification received ===")
		log.Printf("Headers: %+v", r.Header)
		log.Printf("Body: %s", string(body))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("Listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func checkIn(hubURL, apiKey, userID, dynamicURL string) error {
	payload := map[string]string{
		"user_id":     userID,
		"dynamic_url": dynamicURL,
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", hubURL+"/v1/presence/check-in", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("check-in failed: %s", string(body))
	}

	log.Printf("check-in SUCCESSFUL for user %s", userID)
	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
