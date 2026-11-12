package bizbilling

import "testing"

func TestBizOutboxCheckpointFencesPreviousReader(t *testing.T) {
	if FinancialOperationSchemaVersion <= 4 {
		t.Fatal("schema-4 recovery can finish money progress without materializing its durable delivery intents")
	}
	for _, version := range []int{2, 3, 4} {
		if !SupportsFinancialSchema(version) {
			t.Fatalf("historical schema %d must remain readable", version)
		}
	}
}
