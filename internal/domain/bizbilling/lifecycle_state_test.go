package bizbilling

import "testing"

func TestBizLifecycleDefaults(t *testing.T) {
	s := DefaultLifecycleState("app-a")
	if s.AppID != "app-a" || s.LifecycleVersion != 1 || s.Status != "legacy" || s.MutationPermits == nil {
		t.Fatalf("unsafe legacy defaults: %+v", s)
	}
}
