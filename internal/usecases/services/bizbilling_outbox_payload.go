package services

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
)

func deliverySecretBinding(row *bizbilling.NotificationDelivery) secrets.SecretBinding {
	return secrets.SecretBinding{ConnectionID: row.ID, AppID: row.AppID, EnvironmentID: string(row.Scope.Mode), Provider: "biz_billing", Field: "notification_payload"}
}

func (s *BizOutboxService) preparePayload(ctx context.Context, row *bizbilling.NotificationDelivery) (map[string]interface{}, error) {
	data := map[string]interface{}{"payment_link": "", "company_name": ""}
	for key, value := range row.Intent.Payload {
		data[key] = value
	}
	if row.Intent.InvoiceID != "" {
		invoice, err := s.operations.financialInvoice(ctx, row.Scope, row.Intent.InvoiceID)
		if err != nil {
			return nil, err
		}
		if err = guardFinancialRead(ctx, s.operations.gate, s.operations, invoice.FinancialScope(), invoice.LastSequence); err != nil {
			return nil, err
		}
		reminder := strings.HasPrefix(row.Intent.TemplateID, "biz_reminder")
		if reminder && (invoice.AmountDuePaisa <= 0 || invoice.Status == bizbilling.InvoiceStatusVoid || invoice.Status == bizbilling.InvoiceStatusDraft) {
			return nil, nil
		}
		data["invoice_number"] = invoice.InvoiceNumber
		if _, ok := data["amount"]; !ok {
			amount := invoice.TotalPaisa
			if reminder {
				amount = invoice.AmountDuePaisa
			}
			data["amount"] = FormatPaisa(amount, invoice.Currency)
		}
		if invoice.DueDate != nil {
			data["due_date"] = invoice.DueDate.Format("02 Jan 2006")
		}
		if invoice.IssuedSnapshot != nil {
			data["company_name"] = invoice.IssuedSnapshot.Issuer.Name
		}
		if row.SensitivePayload == nil && s.portal != nil && s.portal.portalBaseURL != "" {
			// Require usable encryption before minting a bearer URL. Only this
			// encrypted envelope retains it for an admission retry.
			if s.codec == nil {
				return nil, bizbilling.ErrReconciliationRequired
			}
			if _, err = s.codec.Seal(nil, deliverySecretBinding(row)); err != nil {
				return nil, err
			}
			link := s.portal.PaymentLinkFor(ctx, row.AppID, row.Scope.UserID, invoice.ID)
			if link == "" {
				return nil, bizbilling.ErrReconciliationRequired
			}
			raw, err := json.Marshal(map[string]string{"payment_link": link})
			if err != nil {
				return nil, err
			}
			sealed, err := s.codec.Seal(raw, deliverySecretBinding(row))
			if err != nil {
				return nil, err
			}
			saved, err := s.records.Update(ctx, row.ID, func(current *bizbilling.NotificationDelivery) error {
				if current.LeaseOwner != row.LeaseOwner {
					return bizbilling.ErrConflict
				}
				if current.SensitivePayload == nil {
					current.SensitivePayload = &sealed
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			row.SensitivePayload = saved.SensitivePayload
		}
	}
	if row.SensitivePayload != nil {
		raw, err := s.codec.Open(*row.SensitivePayload, deliverySecretBinding(row))
		if err != nil {
			return nil, err
		}
		var sensitive map[string]string
		if json.Unmarshal(raw, &sensitive) != nil {
			return nil, bizbilling.ErrReconciliationRequired
		}
		for key, value := range sensitive {
			data[key] = value
		}
	}
	return data, nil
}
