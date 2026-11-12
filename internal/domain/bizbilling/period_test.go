package bizbilling

import "testing"

func TestBizPeriodProjectionFencesPreviousReader(t *testing.T) {
	if FinancialOperationSchemaVersion <= 3 {
		t.Fatal("schema-3 recovery can drop invoice period identity")
	}
	for _, historical := range []int{2, 3} {
		if !SupportsFinancialSchema(historical) {
			t.Fatalf("historical schema %d must remain readable", historical)
		}
	}
}
