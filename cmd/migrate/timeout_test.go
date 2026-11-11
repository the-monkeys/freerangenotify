package main

import (
	"testing"
	"time"
)

func TestMigrationUpTimeoutFlag(t *testing.T) {
	flag := upCmd.Flags().Lookup("timeout")
	if flag == nil {
		t.Fatal("bounded migration timeout option is missing")
	}
	if flag.DefValue != (time.Minute).String() {
		t.Fatalf("legacy default changed: %s", flag.DefValue)
	}
}
