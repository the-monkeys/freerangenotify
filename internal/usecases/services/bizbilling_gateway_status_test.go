package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

func TestBizGatewayEventStatusAppScope(t *testing.T) {
	checkout, merchants, gateway, _, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEventIndex, (&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
	service := NewBizGatewayEventService(es, merchants, checkout, zap.NewNop())
	raw := gatewayRaw("payment.captured", "pay_unmatched", "order_unmatched", gateway.ref.AccountID, scope.Currency, 1000)
	receipt, err := service.Accept(ctx, gateway.ref.ConnectionID, "unmatched", raw, gatewaySignature(raw, "owned-hook-secret"))
	require.NoError(t, err)
	_, err = service.Process(ctx, receipt.ID)
	require.Error(t, err)
	stored, err := service.GetForApp(ctx, scope.AppID, receipt.ID)
	require.NoError(t, err)
	require.Equal(t, "unmatched", stored.State)
	_, err = service.GetForApp(ctx, uuid.NewString(), receipt.ID)
	require.Error(t, err)
	rows, err := service.ListForApp(ctx, scope.AppID, "unmatched", "", 50)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, receipt.ID, rows[0].ID)
	rows, err = service.ListForApp(ctx, uuid.NewString(), "", "", 50)
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = service.ListForApp(ctx, scope.AppID, "unmatched", "", 100000)
	require.Error(t, err, "read limits must be bounded")
}
