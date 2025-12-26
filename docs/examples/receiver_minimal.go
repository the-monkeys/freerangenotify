package main

import (
	"io"
	"log"
	"net/http"
	"os"
)

// Minimal webhook receiver example for FreeRangeNotify.
// - Listens on :PORT (default 8090)
// - Accepts POST /webhook with JSON body
// - Just logs the payload and returns 200
func main() {
	port := getEnv("PORT", "8090")

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
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

		log.Println("=== Notification received ===")
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

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
