package bizbillingrepo

import "time"

const BillingPeriodIndex = "frn_biz_periods"

func PeriodID(app, subscription string, start time.Time) string {
	return "bizp_" + digest(struct {
		App          string
		Subscription string
		Start        string
	}{app, subscription, start.UTC().Format(time.RFC3339Nano)})
}
