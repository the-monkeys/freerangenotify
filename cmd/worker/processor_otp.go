package main

import (
	"context"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
)

func (p *NotificationProcessor) SetOTPDeliveryAuthorizer(a otp.DeliveryAuthorizer) {
	p.otpDeliveryAuthorizer = a
}

func (p *NotificationProcessor) authorizeOTPDelivery(ctx context.Context, n *notification.Notification) error {
	op, present := n.Metadata["otp_operation_id"]
	if !present {
		return nil
	} // Legacy deadline-only notifications retain their path.
	operationID, ok := op.(string)
	requestID, _ := n.Metadata["otp_request_id"].(string)
	if !ok || operationID == "" || requestID == "" {
		return otp.ErrNotFound
	}
	if p.otpDeliveryAuthorizer == nil {
		return otp.ErrStateUnavailable
	}
	return p.otpDeliveryAuthorizer.AuthorizeDelivery(ctx, otp.Scope{AppID: n.AppID, EnvironmentID: n.EnvironmentID}, requestID, operationID, n.NotificationID)
}
