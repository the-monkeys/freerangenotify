package bizbilling

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestPlanChargeCheckedPreservesModelsAndRejectsOverflow(t *testing.T) {
	tiers := []PricingTier{{UpTo: 10, UnitPaisa: 100, FlatPaisa: 10}, {UpTo: 20, UnitPaisa: 80, FlatPaisa: 20}}
	for _, model := range []PricingModel{PricingModelFlat, PricingModelPerUnit, PricingModelTiered, PricingModelVolume} {
		plan := &Plan{AmountPaisa: 100, PricingModel: model, Tiers: tiers}
		got, err := PlanChargeForChecked(plan, 25)
		require.NoError(t, err)
		require.Equal(t, PlanChargeFor(plan, 25), got)
	}
	_, err := PlanChargeForChecked(&Plan{AmountPaisa: math.MaxInt64, PricingModel: PricingModelPerUnit}, 2)
	require.ErrorIs(t, err, ErrMoneyOverflow)
}
