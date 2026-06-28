package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

// WhatsAppSelfHostedService manages multi-device paired client sessions.
type WhatsAppSelfHostedService struct {
	clients     map[string]*whatsmeow.Client
	dbContainer *sqlstore.Container
	logger      *zap.Logger
	mu          sync.RWMutex
}

// NewWhatsAppSelfHostedService creates a new WhatsApp self-hosted connection manager.
func NewWhatsAppSelfHostedService(logger *zap.Logger) (*WhatsAppSelfHostedService, error) {
	// Ensure the storage directory exists for the SQLite database
	if err := os.MkdirAll("storage", 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Initialize whatsmeow sqlstore for multi-device credentials
	dbContainer, err := sqlstore.New(context.Background(), "sqlite", "file:storage/whatsapp_sessions.db?_foreign_keys=on", waLog.Stdout("WhatsMeowDB", "WARN", true))
	if err != nil {
		return nil, fmt.Errorf("failed to open whatsmeow session database: %w", err)
	}

	return &WhatsAppSelfHostedService{
		clients:     make(map[string]*whatsmeow.Client),
		dbContainer: dbContainer,
		logger:      logger,
	}, nil
}

// Connect starts or retrieves a connection session, streaming QR codes if pairing is required.
func (s *WhatsAppSelfHostedService) Connect(ctx context.Context, appID string, sseCh chan<- string) error {
	s.mu.Lock()
	client, exists := s.clients[appID]
	s.mu.Unlock()

	if exists && client.IsConnected() {
		sseCh <- "connected"
		return nil
	}

	s.logger.Info("Initializing whatsmeow client for app", zap.String("app_id", appID))

	// Retrieve or create a device store mapped to this appID
	deviceStore, err := s.dbContainer.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to get whatsmeow device store: %w", err)
	}
	if deviceStore == nil {
		return fmt.Errorf("failed to locate first device store")
	}

	// Inwhatsmeow, we can configure loggers to use stdout or silence them
	nullLog := waLog.Stdout("WhatsMeow", "WARN", true)
	client = whatsmeow.NewClient(deviceStore, nullLog)

	s.mu.Lock()
	s.clients[appID] = client
	s.mu.Unlock()

	// If the device has no saved credentials, spawn QR channel
	if client.Store.ID == nil {
		s.logger.Info("App has no paired credentials. Generating QR code stream...", zap.String("app_id", appID))
		qrChan, err := client.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("failed to request QR channel: %w", err)
		}

		err = client.Connect()
		if err != nil {
			return fmt.Errorf("failed to connect whatsmeow client: %w", err)
		}

		go func() {
			for {
				select {
				case <-ctx.Done():
					s.logger.Info("SSE connection context closed; aborting pairing stream", zap.String("app_id", appID))
					return
				case qrEvt, ok := <-qrChan:
					if !ok {
						return
					}
					switch qrEvt.Event {
					case "code":
						s.logger.Debug("New WhatsApp Pairing QR Code generated", zap.String("code", qrEvt.Code))
						qrDataURL, qrErr := toQRCodeDataURL(qrEvt.Code)
						if qrErr != nil {
							s.logger.Warn("Failed to encode WhatsApp pairing QR image", zap.Error(qrErr))
							// Keep a text fallback so the UI can still present a code path.
							sseCh <- "pairing_code:" + qrEvt.Code
							continue
						}
						// Prefix allows the UI to distinguish QR image payloads from status messages.
						sseCh <- "qr_image:" + qrDataURL
					case "success":
						s.logger.Info("WhatsApp Multi-Device paired successfully!", zap.String("app_id", appID))
						sseCh <- "success"
						return
					case "timeout":
						s.logger.Warn("WhatsApp Pairing QR timed out", zap.String("app_id", appID))
						sseCh <- "timeout"
						return
					}
				}
			}
		}()
	} else {
		// Device is already paired, reconnect
		s.logger.Info("Device credentials found; connecting directly...", zap.String("app_id", appID))
		err = client.Connect()
		if err != nil {
			return fmt.Errorf("failed to reconnect paired client: %w", err)
		}
		sseCh <- "connected"
	}

	return nil
}

// Disconnect closes the client socket and deletes stored session tokens.
func (s *WhatsAppSelfHostedService) Disconnect(ctx context.Context, appID string) error {
	s.mu.Lock()
	client, exists := s.clients[appID]
	if exists {
		client.Disconnect()
		delete(s.clients, appID)
	}
	s.mu.Unlock()

	s.logger.Info("Purging WhatsApp Multi-Device session", zap.String("app_id", appID))

	// Inwhatsmeow, logging out deletes local keys
	if client != nil && client.Store.ID != nil {
		err := client.Logout(ctx)
		if err != nil {
			return fmt.Errorf("failed to log out whatsmeow client: %w", err)
		}
	}

	return nil
}

// GetStatus returns the session pairing state of the client.
func (s *WhatsAppSelfHostedService) GetStatus(appID string) (map[string]interface{}, error) {
	s.mu.RLock()
	client, exists := s.clients[appID]
	s.mu.RUnlock()

	if !exists {
		return map[string]interface{}{
			"connected": false,
			"provider":  "self_hosted",
			"status":    "disconnected",
		}, nil
	}

	connected := client.IsConnected() && client.Store.ID != nil
	status := "disconnected"
	if connected {
		status = "connected"
	} else if client.Store.ID == nil {
		status = "unpaired"
	}

	res := map[string]interface{}{
		"connected":         connected,
		"provider":          "self_hosted",
		"connection_status": status,
	}

	if connected && client.Store.ID != nil {
		res["display_phone"] = client.Store.ID.User
		res["waba_id"] = "self_hosted"
		res["quality_rating"] = "N/A (Self-Hosted)"
		res["connected_at"] = time.Now().UTC().Format(time.RFC3339)
	}

	return res, nil
}

// GetClient retrieves the underlying whatsmeow client if connected.
func (s *WhatsAppSelfHostedService) GetClient(appID string) (*whatsmeow.Client, error) {
	s.mu.RLock()
	client, exists := s.clients[appID]
	s.mu.RUnlock()

	if !exists || client == nil || !client.IsConnected() {
		return nil, fmt.Errorf("whatsapp self-hosted client not connected or initialized for app %s", appID)
	}

	return client, nil
}

// Stop closes all active client connections.
func (s *WhatsAppSelfHostedService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for appID, client := range s.clients {
		s.logger.Info("Closing active whatsmeow client connection", zap.String("app_id", appID))
		client.Disconnect()
	}
	s.clients = make(map[string]*whatsmeow.Client)
}

func toQRCodeDataURL(code string) (string, error) {
	png, err := qrcode.Encode(code, qrcode.Medium, 320)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}
