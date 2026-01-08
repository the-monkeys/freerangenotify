package main

import (
	"io"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/webhook/verify", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		log.Printf("RECEIVED_WEBHOOK: %s", string(body))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Println("Starting Webhook Receiver on :8092...")
	if err := http.ListenAndServe(":8092", nil); err != nil {
		log.Fatal(err)
	}
}
