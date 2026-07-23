package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

// WhatsAppSelfHostedProvider delivers WhatsApp notifications using a self-hosted paired multi-device client.
type WhatsAppSelfHostedProvider struct {
	service *services.WhatsAppSelfHostedService
	logger  *zap.Logger
}

// NewWhatsAppSelfHostedProvider creates a new WhatsAppSelfHostedProvider.
func NewWhatsAppSelfHostedProvider(service *services.WhatsAppSelfHostedService, logger *zap.Logger) *WhatsAppSelfHostedProvider {
	return &WhatsAppSelfHostedProvider{
		service: service,
		logger:  logger,
	}
}

// Send delivers the notification directly using the linked whatsmeow client.
func (p *WhatsAppSelfHostedProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	if usr == nil || usr.Phone == "" {
		return NewErrorResult(
			fmt.Errorf("user has no phone number or group JID for WhatsApp delivery"),
			ErrorTypeInvalid,
		), nil
	}

	// Fetch active whatsmeow client session for this app
	client, err := p.service.GetClient(notif.AppID)
	if err != nil {
		p.logger.Error("WhatsApp self-hosted connection unavailable", zap.String("app_id", notif.AppID), zap.Error(err))
		return NewErrorResult(
			fmt.Errorf("WhatsApp self-hosted client is not paired or active: %w", err),
			ErrorTypeConfiguration,
		), nil
	}

	// Resolve destination JID
	var targetJID types.JID
	if strings.Contains(usr.Phone, "@g.us") {
		// Group dispatch: parsed JID directly
		targetJID, err = types.ParseJID(usr.Phone)
		if err != nil {
			return NewErrorResult(fmt.Errorf("invalid group JID: %w", err), ErrorTypeInvalid), nil
		}
	} else {
		// Individual phone lookup
		cleanPhone := strings.TrimPrefix(usr.Phone, "+")
		cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
		cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
		targetJID = types.NewJID(cleanPhone, types.DefaultUserServer)
	}

	// Construct base text body
	messageBody := notif.Content.Body
	if notif.Content.Title != "" {
		messageBody = fmt.Sprintf("*%s*\n\n%s", notif.Content.Title, notif.Content.Body)
	}

	var waMessage waE2E.Message

	// Detect Rich Content (Lists, Carousels, or Product Sliders) in the notification data
	if listData, exists := notif.Content.Data["list"].(map[string]interface{}); exists {
		// 1. Compile Rich Selection List Message
		p.logger.Info("Compiling WhatsApp Interactive List message", zap.String("notification_id", notif.NotificationID))
		listTitle, _ := listData["title"].(string)
		if listTitle == "" {
			listTitle = notif.Content.Title
		}
		listDesc, _ := listData["description"].(string)
		if listDesc == "" {
			listDesc = notif.Content.Body
		}
		buttonText, _ := listData["button_text"].(string)
		if buttonText == "" {
			buttonText = "View Options"
		}

		listMsg := &waE2E.ListMessage{
			Title:       &listTitle,
			Description: &listDesc,
			ButtonText:  &buttonText,
			ListType:    waE2E.ListMessage_SINGLE_SELECT.Enum(),
		}

		if rawSections, ok := listData["sections"].([]interface{}); ok {
			for _, sVal := range rawSections {
				if sMap, ok := sVal.(map[string]interface{}); ok {
					sTitle, _ := sMap["title"].(string)
					section := &waE2E.ListMessage_Section{
						Title: &sTitle,
					}
					if rawRows, ok := sMap["rows"].([]interface{}); ok {
						for _, rVal := range rawRows {
							if rMap, ok := rVal.(map[string]interface{}); ok {
								rID, _ := rMap["id"].(string)
								rTitle, _ := rMap["title"].(string)
								rDesc, _ := rMap["description"].(string)

								row := &waE2E.ListMessage_Row{
									RowID:       &rID,
									Title:       &rTitle,
									Description: &rDesc,
								}
								section.Rows = append(section.Rows, row)
							}
						}
					}
					listMsg.Sections = append(listMsg.Sections, section)
				}
			}
		}

		waMessage.ListMessage = listMsg

	} else if productData, exists := notif.Content.Data["product"].(map[string]interface{}); exists {
		// 2. Compile Interactive Single Product Card
		p.logger.Info("Compiling WhatsApp Product card", zap.String("notification_id", notif.NotificationID))
		prodID, _ := productData["product_id"].(string)
		prodTitle, _ := productData["title"].(string)
		prodDesc, _ := productData["description"].(string)
		currency, _ := productData["currency"].(string)
		if currency == "" {
			currency = "INR"
		}
		priceInt, _ := productData["price_amount_1000"].(int64)

		prodSnapshot := &waE2E.ProductMessage_ProductSnapshot{
			ProductID:       &prodID,
			Title:           &prodTitle,
			Description:     &prodDesc,
			CurrencyCode:    &currency,
			PriceAmount1000: &priceInt,
		}

		// Support media attachments on product details if available
		if imgURL, _ := productData["image_url"].(string); imgURL != "" {
			imgMsg, err := p.uploadImageFromURL(ctx, client, imgURL)
			if err == nil {
				prodSnapshot.ProductImage = imgMsg
			} else {
				p.logger.Warn("Failed to upload product image; falling back to text product snapshot", zap.Error(err))
			}
		}

		ownerJID := client.Store.ID.ToNonAD().String()
		waMessage.ProductMessage = &waE2E.ProductMessage{
			Product:          prodSnapshot,
			BusinessOwnerJID: &ownerJID,
		}

	} else if notif.Content.MediaURL != "" {
		// 3. Compile Standard Image/Attachment Message
		p.logger.Info("Uploading media attachment for WhatsApp delivery", zap.String("notification_id", notif.NotificationID))
		imgMsg, err := p.uploadImageFromURL(ctx, client, notif.Content.MediaURL)
		if err == nil {
			imgMsg.Caption = &messageBody
			waMessage.ImageMessage = imgMsg
		} else {
			// Fallback to sending standard text containing the media link
			p.logger.Warn("Failed to upload media; falling back to direct text delivery", zap.Error(err))
			fallbackText := fmt.Sprintf("%s\n\nAttachment: %s", messageBody, notif.Content.MediaURL)
			waMessage.Conversation = &fallbackText
		}
	} else {
		// 4. Default Standard Free-form Text Message
		waMessage.Conversation = &messageBody
	}

	// Dispatch message via whatsmeow Multi-Device Socket
	resp, err := client.SendMessage(ctx, targetJID, &waMessage)
	if err != nil {
		p.logger.Error("Failed to deliver WhatsApp message", zap.String("notification_id", notif.NotificationID), zap.Error(err))
		return NewErrorResult(fmt.Errorf("whatsmeow SendMessage failed: %w", err), ErrorTypeProviderAPI), nil
	}

	p.logger.Info("Self-hosted WhatsApp message dispatched successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.String("message_id", resp.ID),
		zap.Duration("duration", time.Since(start)))

	result := NewResult(resp.ID, time.Since(start))
	result.Metadata["credential_source"] = CredSourceBYOC
	result.Metadata["billing_channel"] = "whatsapp"
	result.Metadata["whatsapp_sender"] = client.Store.ID.User
	result.Metadata["self_hosted"] = "true"

	return result, nil
}

// uploadImageFromURL fetches the file from the remote URL and registers it in WhatsApp's servers.
func (p *WhatsAppSelfHostedProvider) uploadImageFromURL(ctx context.Context, client *whatsmeow.Client, url string) (*waE2E.ImageMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote media server returned status %d", resp.StatusCode)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Upload to WhatsApp CDN
	uploadResp, err := client.Upload(ctx, bytes, whatsmeow.MediaImage)
	if err != nil {
		return nil, fmt.Errorf("whatsapp CDN upload failed: %w", err)
	}

	return &waE2E.ImageMessage{
		URL:           &uploadResp.URL,
		DirectPath:    &uploadResp.DirectPath,
		MediaKey:      uploadResp.MediaKey,
		FileEncSHA256: uploadResp.FileEncSHA256,
		FileSHA256:    uploadResp.FileSHA256,
		FileLength:    &uploadResp.FileLength,
	}, nil
}

// GetName returns the provider name.
func (p *WhatsAppSelfHostedProvider) GetName() string { return "whatsapp_self_hosted" }

// GetSupportedChannel returns the notification channel supported by this provider.
func (p *WhatsAppSelfHostedProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelWhatsApp
}

// IsHealthy returns connection availability.
func (p *WhatsAppSelfHostedProvider) IsHealthy(_ context.Context) bool { return true }

// Close terminates any active pipelines.
func (p *WhatsAppSelfHostedProvider) Close() error { return nil }
