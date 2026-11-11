package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

func TestBizEstimateConversionUsesSingleJournalFact(t *testing.T) {
	lifecycle, _, ops, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.EstimateIndex, (&database.IndexTemplates{}).GetBizEstimatesTemplate())
	estimate := &bizbilling.Estimate{ID: uuid.NewString(), AppID: scope.AppID, UserID: scope.UserID, EstimateNumber: "EST-FIXTURE", Status: bizbilling.EstimateStatusAccepted, Currency: scope.Currency, LineItems: []bizbilling.InvoiceLineItem{{Description: "Accepted service", Quantity: 1, UnitPrice: 1000, TaxRate: 18, AmountPaisa: 1000}}, SubtotalPaisa: 1000, TaxPaisa: 180, TotalPaisa: 1180, CreatedAt: time.Now().UTC()}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.Estimate](es, bizbillingrepo.EstimateIndex, zap.NewNop()).Create(ctx, estimate.ID, estimate))
	service := NewBizEstimateService(ops.stores, ops.config, lifecycle, nil, nil, zap.NewNop())
	first, err := service.Convert(ctx, scope.AppID, estimate.ID, nil)
	require.NoError(t, err)
	replay, err := service.Convert(ctx, scope.AppID, estimate.ID, nil)
	require.NoError(t, err)
	require.Equal(t, first.ID, replay.ID)
	require.Equal(t, estimate.ID, first.EstimateID)
	require.Equal(t, bizbilling.InvoiceStatusOpen, first.Status)
	require.Equal(t, int64(1180), first.TotalPaisa)
	converted, err := service.Get(ctx, scope.AppID, estimate.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.EstimateStatusConverted, converted.Status)
	require.Equal(t, first.ID, converted.ConvertedInvoiceID)
	stream, err := ops.streams.Get(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(1), stream.CommittedSequence)
	fact, err := ops.journal.Get(ctx, scope, stream.CommittedSequence)
	require.NoError(t, err)
	require.Equal(t, "estimate_convert", string(fact.Kind))
}
