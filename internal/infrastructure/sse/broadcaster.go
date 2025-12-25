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

// SSEMessage represents a message to broadcast via SSE
type SSEMessage struct {
	Type         string                     `json:"type"`
	UserID       string                     `json:"user_id,omitempty"`
	Notification *notification.Notification `json:"notification,omitempty"`
	Data         interface{}                `json:"data,omitempty"`
}

// Client represents an SSE client connection
type Client struct {
	UserID string
	Chan   chan string
}

// Broadcaster manages SSE connections and broadcasts messages
type Broadcaster struct {
	clients  map[string][]*Client
	mu       sync.RWMutex
	logger   *zap.Logger
	redis    *redis.Client
	pubsub   *redis.PubSub
	stopChan chan struct{}
}

// NewBroadcaster creates a new SSE broadcaster
func NewBroadcaster(logger *zap.Logger) *Broadcaster {
	return &Broadcaster{
		clients:  make(map[string][]*Client),
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// SetRedis sets the Redis client for pub/sub
func (b *Broadcaster) SetRedis(redisClient *redis.Client) {
	b.redis = redisClient
	b.pubsub = redisClient.Subscribe(context.Background(), "sse:notifications")
	go b.listenForMessages()
}

// listenForMessages listens for Redis pub/sub messages with reconnection logic
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

			// Subscribe to the channel
			b.pubsub = b.redis.Subscribe(context.Background(), "sse:notifications")
			ch := b.pubsub.Channel()

			b.logger.Info("SSE broadcaster subscribed to Redis pub/sub")

			// Listen for messages
		messageLoop:
			for {
				select {
				case msg, ok := <-ch:
					if !ok {
						// Channel closed, break to reconnect
						b.logger.Warn("Redis pub/sub channel closed, attempting reconnection")
						break messageLoop
					}

					b.logger.Debug("Received message from Redis",
						zap.String("channel", msg.Channel),
						zap.String("payload", msg.Payload))

					var sseMsg SSEMessage
					if err := json.Unmarshal([]byte(msg.Payload), &sseMsg); err != nil {
						b.logger.Error("Failed to unmarshal SSE message", zap.Error(err))
						continue
					}

					if sseMsg.UserID != "" {
						b.logger.Debug("Broadcasting message to user",
							zap.String("user_id", sseMsg.UserID))
						b.Broadcast(sseMsg.UserID, msg.Payload)
					} else {
						b.logger.Debug("Broadcasting message to all users")
						b.BroadcastToAll(msg.Payload)
					}

				case <-b.stopChan:
					return
				}
			}

			// Close the pubsub and wait before reconnecting
			if b.pubsub != nil {
				b.pubsub.Close()
				b.pubsub = nil
			}

			b.logger.Info("Reconnecting SSE pub/sub in 5 seconds...")
			time.Sleep(5 * time.Second)
		}
	}
}

// PublishMessage publishes an SSE message to Redis for broadcasting
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

// AddClient adds a new SSE client for a user
func (b *Broadcaster) AddClient(userID string) *Client {
	b.mu.Lock()
	defer b.mu.Unlock()

	client := &Client{
		UserID: userID,
		Chan:   make(chan string, 10), // Buffered channel to prevent blocking
	}

	b.clients[userID] = append(b.clients[userID], client)
	b.logger.Info("SSE client added", zap.String("user_id", userID))

	return client
}

// RemoveClient removes an SSE client
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

// Broadcast sends a message to all clients of a user
func (b *Broadcaster) Broadcast(userID string, message string) {
	b.mu.RLock()
	clients := b.clients[userID]
	b.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.Chan <- message:
		default:
			// Client channel is full, skip to avoid blocking
			b.logger.Warn("SSE client channel full, skipping message",
				zap.String("user_id", userID))
		}
	}
}

// BroadcastToAll sends a message to all connected clients
func (b *Broadcaster) BroadcastToAll(message string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for userID, clients := range b.clients {
		for _, client := range clients {
			select {
			case client.Chan <- message:
			default:
				b.logger.Warn("SSE client channel full, skipping message",
					zap.String("user_id", userID))
			}
		}
	}
}

// HandleSSE handles Server-Sent Events for a user
func (b *Broadcaster) HandleSSE(c *fiber.Ctx, userID string) error {
	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Headers", "Cache-Control")

	ctxDone := c.Context().Done()
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Add client
		client := b.AddClient(userID)
		defer func() {
			b.RemoveClient(userID, client)
			b.logger.Info("SSE client connection closed", zap.String("user_id", userID))
		}()

		b.logger.Info("SSE client connected", zap.String("user_id", userID))

		// Send initial connection message
		if _, err := fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n"); err != nil {
			b.logger.Error("Failed to send initial SSE message", zap.Error(err))
			return
		}

		if err := w.Flush(); err != nil {
			b.logger.Error("Failed to flush initial SSE message", zap.Error(err))
			return
		}

		// Listen for messages
		for {
			select {
			case message, ok := <-client.Chan:
				if !ok {
					// Channel closed
					b.logger.Info("SSE client channel closed", zap.String("user_id", userID))
					return
				}

				// Send message to client
				if _, err := fmt.Fprintf(w, "data: %s\n\n", message); err != nil {
					b.logger.Error("Failed to write SSE message", zap.Error(err))
					return
				}

				// Flush after each message
				if err := w.Flush(); err != nil {
					b.logger.Error("Failed to flush SSE message", zap.Error(err))
					return
				}

			case <-ctxDone:
				// Client disconnected
				b.logger.Info("SSE client context done", zap.String("user_id", userID))
				return
			}
		}
	})

	return nil
}

// Close stops the broadcaster
func (b *Broadcaster) Close() {
	close(b.stopChan)
	if b.pubsub != nil {
		b.pubsub.Close()
	}
}
