package bizbilling

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestBizAnchoredCalendar(t *testing.T) {
	start := time.Date(2027, 1, 31, 12, 3, 4, 5, time.UTC)
	feb, err := NextAnchoredPeriod(start, BillingCycleMonthly, 0, 31)
	require.NoError(t, err)
	require.Equal(t, "2027-02-28", feb.Format("2006-01-02"))
	march, err := NextAnchoredPeriod(feb, BillingCycleMonthly, 0, 31)
	require.NoError(t, err)
	require.Equal(t, "2027-03-31", march.Format("2006-01-02"))
	require.Equal(t, 5, march.Nanosecond())
	leap, err := NextAnchoredPeriod(time.Date(2028, 1, 31, 0, 0, 0, 0, time.UTC), BillingCycleMonthly, 0, 31)
	require.NoError(t, err)
	require.Equal(t, 29, leap.Day())
	_, err = NextAnchoredPeriod(start, BillingCycleCustom, 0, 31)
	require.Error(t, err)
	_, err = NextAnchoredPeriod(start, BillingCycleMonthly, 0, 32)
	require.Error(t, err)
	_, err = NextAnchoredPeriod(start, "unknown", 0, 31)
	require.Error(t, err)
}
