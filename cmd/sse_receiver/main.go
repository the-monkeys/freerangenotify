package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type SSEMessage struct {
	Type         string                 `json:"type"`
	UserID       string                 `json:"user_id,omitempty"`
	Notification map[string]interface{} `json:"notification,omitempty"`
	Data         interface{}            `json:"data,omitempty"`
}

func main() {
	userID := getenv("USER_ID", "7998188a-1649-40f9-b2e1-de26a130c367")
	hubURL := getenv("HUB_URL", "http://localhost:8080")

	log.Printf("SSE Receiver starting for user: %s", userID)
	log.Printf("Connecting to: %s/v1/sse?user_id=%s", hubURL, userID)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 0, // No timeout for SSE connection
	}

	// Create request
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/sse?user_id=%s", hubURL, userID), nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Make the request
	log.Println("Making HTTP request...")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to connect to SSE endpoint: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("Response received. Status: %s", resp.Status)
	log.Printf("Headers: %v", resp.Header)

	if resp.StatusCode != 200 {
		log.Fatalf("SSE connection failed with status: %d", resp.StatusCode)
	}

	log.Println("SSE connection established. Listening for notifications...")

	// Handle graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		log.Println("Shutting down SSE receiver...")
		resp.Body.Close()
	}()

	// Read SSE stream
	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			log.Println("Waiting to read line...")
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					log.Println("SSE connection closed by server")
					return
				}
				log.Printf("Error reading SSE stream: %v", err)
				time.Sleep(5 * time.Second) // Wait before reconnecting
				continue
			}

			log.Printf("Received raw line: %q", line)

			line = strings.TrimSpace(line)
			if line == "" {
				log.Println("Empty line (keep-alive/separator)")
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "{\"type\":\"connected\"}" {
					log.Println("SSE connection confirmed")
					continue
				}

				// Parse the notification
				var sseMsg SSEMessage
				if err := json.Unmarshal([]byte(data), &sseMsg); err != nil {
					log.Printf("Failed to parse SSE message: %v", err)
					log.Printf("Raw data: %s", data)
					continue
				}

				log.Println("=== SSE NOTIFICATION RECEIVED ===")
				log.Printf("Type: %s", sseMsg.Type)
				log.Printf("UserID: %s", sseMsg.UserID)

				if sseMsg.Notification != nil {
					log.Printf("Notification: %+v", sseMsg.Notification)
				}

				if sseMsg.Data != nil {
					log.Printf("Data: %+v", sseMsg.Data)
				}

				log.Println("==================================")
			}
		}
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
