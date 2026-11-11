package handlers

// SSETokenResponseDoc describes the existing SSE token envelope.
type SSETokenResponseDoc struct {
	SSEToken  string `json:"sse_token"`
	UserID    string `json:"user_id"`
	ExpiresIn int    `json:"expires_in"`
}
