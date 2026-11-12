package bizbilling

import "testing"

func TestBizIssuedSnapshotFencesPreviousReader(t *testing.T) {
	// The previous reader accepts only schema 2 and drops unknown invoice
	// fields when projecting. New immutable snapshots must fence it out.
	if FinancialOperationSchemaVersion <= 2 {
		t.Fatal("a schema-2 worker can discard the issued snapshot during recovery")
	}
}
