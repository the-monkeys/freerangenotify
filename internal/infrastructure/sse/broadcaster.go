package sse

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
)

// SSEMessage is the internal Redis pub/sub envelope. It carries routing
// information (UserID) and the notification payload.
type SSEMessage struct {
	Type         string                     `json:"type"`
	UserID       string                     `json:"user_id,omitempty"`
	Notification *notification.Notification `json:"notification,omitempty"`
	Data         interface{}                `json:"data,omitempty"`
}

// ClientPayload is the clean structure delivered to browser clients.
// It contains only what the frontend needs — no internal fields like
// app_id, retry_count, or error_message.
type ClientPayload struct {
	NotificationID string                 `json:"notification_id"`
	Title          string                 `json:"title"`
	Body           string                 `json:"body"`
	Channel        string                 `json:"channel,omitempty"`
	Category       string                 `json:"category,omitempty"`
	Status         string                 `json:"status"`
	Data           map[string]interface{} `json:"data,omitempty"`
	CreatedAt      string                 `json:"created_at"`
}

// Client represents an SSE client connection.
type Client struct {
	UserID string
	Chan   chan string
}

// Broadcaster manages SSE connections and broadcasts messages via Redis pub/sub.
type Broadcaster struct {
	repo     notification.Repository
	clients  map[string][]*Client
	mu       sync.RWMutex
	logger   *zap.Logger
	redis    *redis.Client
	stopChan chan struct{}
}

// NewBroadcaster creates a new SSE broadcaster.
func NewBroadcaster(repo notification.Repository, logger *zap.Logger) *Broadcaster {
	return &Broadcaster{
		repo:     repo,
		clients:  make(map[string][]*Client),
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// SetRedis sets the Redis client and starts listening for pub/sub messages.
func (b *Broadcaster) SetRedis(redisClient *redis.Client) {
	b.redis = redisClient
	go b.listenForMessages()
}

// listenForMessages subscribes to the Redis "sse:notifications" channel
// with automatic reconnection.
func (b *Broadcaster) listenForMessages() {
	for {
		select {
		case <-b.stopChan:
			return
		default:
			if b.redis == nil {
				b.logger.Error("Redis client not available for SSE pub/sub")
				time.Sleep(5 * time.Second)
				continue
			}

			pubsub := b.redis.Subscribe(context.Background(), "sse:notifications")
			ch := pubsub.Channel()

			b.logger.Info("SSE broadcaster subscribed to Redis pub/sub")

		messageLoop:
			for {
				select {
				case msg, ok := <-ch:
					if !ok {
						b.logger.Warn("Redis pub/sub channel closed, reconnecting")
						break messageLoop
					}

					var sseMsg SSEMessage
					if err := json.Unmarshal([]byte(msg.Payload), &sseMsg); err != nil {
						b.logger.Error("Failed to unmarshal SSE message", zap.Error(err))
						continue
					}

					// Build a clean client payload from the notification.
					clientData := b.buildClientPayload(&sseMsg)

					if sseMsg.UserID != "" {
						b.logger.Debug("Broadcasting to user",
							zap.String("user_id", sseMsg.UserID))
						b.Broadcast(sseMsg.UserID, clientData)
					} else {
						b.logger.Debug("Broadcasting to all users")
						b.BroadcastToAll(clientData)
					}

				case <-b.stopChan:
					pubsub.Close()
					return
				}
			}

			pubsub.Close()
			b.logger.Info("Reconnecting SSE pub/sub in 5 seconds...")
			time.Sleep(5 * time.Second)
		}
	}
}

// buildClientPayload converts an internal SSEMessage into a clean JSON string
// suitable for browser consumption.
func (b *Broadcaster) buildClientPayload(sseMsg *SSEMessage) string {
	if sseMsg.Notification != nil {
		n := sseMsg.Notification
		payload := ClientPayload{
			NotificationID: n.NotificationID,
			Title:          n.Content.Title,
			Body:           n.Content.Body,
			Channel:        string(n.Channel),
			Category:       n.Category,
			Status:         string(n.Status),
			Data:           n.Content.Data,
			CreatedAt:      n.CreatedAt.Format(time.RFC3339),
		}
		data, err := json.Marshal(payload)
		if err != nil {
			b.logger.Error("Failed to marshal client payload", zap.Error(err))
			return "{}"
		}
		return string(data)
	}

	// Fallback: forward raw data field
	data, err := json.Marshal(sseMsg.Data)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// PublishMessage publishes an SSE message to Redis for broadcasting.
func (b *Broadcaster) PublishMessage(msg *SSEMessage) error {
	if b.redis == nil {
		return fmt.Errorf("Redis client not set")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return b.redis.Publish(context.Background(), "sse:notifications", data).Err()
}

// AddClient registers a new SSE client for a user.
func (b *Broadcaster) AddClient(userID string) *Client {
	b.mu.Lock()
	defer b.mu.Unlock()

	client := &Client{
		UserID: userID,
		Chan:   make(chan string, 10),
	}

	b.clients[userID] = append(b.clients[userID], client)
	b.logger.Info("SSE client added", zap.String("user_id", userID))
	return client
}

// RemoveClient unregisters an SSE client.
func (b *Broadcaster) RemoveClient(userID string, client *Client) {
	b.mu.Lock()
	defer b.mu.Unlock()

	clients := b.clients[userID]
	for i, c := range clients {
		if c == client {
			close(c.Chan)
			b.clients[userID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}

	if len(b.clients[userID]) == 0 {
		delete(b.clients, userID)
	}

	b.logger.Info("SSE client removed", zap.String("user_id", userID))
}

// Broadcast sends a message to all clients of a specific user.
func (b *Broadcaster) Broadcast(userID string, message string) {
	b.mu.RLock()
	clients := b.clients[userID]
	b.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.Chan <- message:
		default:
			b.logger.Warn("SSE client channel full, dropping message",
				zap.String("user_id", userID))
		}
	}
}

// BroadcastToAll sends a message to every connected client.
func (b *Broadcaster) BroadcastToAll(message string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for userID, clients := range b.clients {
		for _, client := range clients {
			select {
			case client.Chan <- message:
			default:
				b.logger.Warn("SSE client channel full, dropping message",
					zap.String("user_id", userID))
			}
		}
	}
}

// HandleSSE manages the long-lived SSE connection for a single user.
func (b *Broadcaster) HandleSSE(c *fiber.Ctx, userID string) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Headers", "Cache-Control")

	// Disable write deadline for long-lived stream
	if c.Context().Conn() != nil {
		if err := c.Context().Conn().SetWriteDeadline(time.Time{}); err != nil {
			b.logger.Warn("Failed to disable write deadline", zap.Error(err))
		}
	}

	ctxDone := c.Context().Done()
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		client := b.AddClient(userID)

		defer func() {
			if r := recover(); r != nil {
				b.logger.Error("Panic in SSE stream", zap.Any("panic", r))
			}
			b.RemoveClient(userID, client)
		}()

		b.logger.Info("SSE stream started", zap.String("user_id", userID))

		// Named "connected" event — clients use addEventListener("connected", ...)
		fmt.Fprintf(w, "event: connected\ndata: {\"user_id\":%q}\n\n", userID)
		if err := w.Flush(); err != nil {
			b.logger.Error("Failed to send connected event", zap.Error(err))
			return
		}

		// Heartbeat to keep proxies/load-balancers from closing the connection
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case message, ok := <-client.Chan:
				if !ok {
					return
				}

				// Named "notification" event
				if _, err := fmt.Fprintf(w, "event: notification\ndata: %s\n\n", message); err != nil {
					b.logger.Error("Failed to write SSE message", zap.String("user_id", userID), zap.Error(err))
					b.handleDeliveryFailure(c.Context(), message)
					return
				}

				if err := w.Flush(); err != nil {
					b.logger.Error("Failed to flush SSE message", zap.String("user_id", userID), zap.Error(err))
					b.handleDeliveryFailure(c.Context(), message)
					return
				}

			case <-ticker.C:
				// SSE comment line — not visible to EventSource API but keeps TCP alive
				fmt.Fprintf(w, ": keepalive\n\n")
				if err := w.Flush(); err != nil {
					b.logger.Info("SSE keepalive failed, client likely disconnected",
						zap.String("user_id", userID))
					return
				}

			case <-ctxDone:
				b.logger.Info("SSE client disconnected", zap.String("user_id", userID))
				return
			}
		}
	})

	return nil
}

// handleDeliveryFailure resets a notification to Queued so it can be retried.
func (b *Broadcaster) handleDeliveryFailure(ctx context.Context, message string) {
	// Try to parse the message as ClientPayload to get the notification ID
	var payload ClientPayload
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		b.logger.Warn("Cannot parse failed message for recovery", zap.Error(err))
		return
	}

	if payload.NotificationID != "" {
		b.logger.Info("Resetting failed notification to queued",
			zap.String("notification_id", payload.NotificationID))

		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := b.repo.UpdateStatus(bgCtx, payload.NotificationID, notification.StatusQueued); err != nil {
			b.logger.Error("Failed to reset notification status", zap.Error(err))
		}
	}
}

// Close stops the broadcaster and releases resources.
func (b *Broadcaster) Close() {
	close(b.stopChan)
}
