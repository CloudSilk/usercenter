package alert

import (
	"testing"
)

func TestIncrAlertCount(t *testing.T) {
	alertCache.Flush()
	key := "test:incr"

	if got := incrAlertCount(key); got != 1 {
		t.Fatalf("first incr = %d, want 1", got)
	}
	if got := incrAlertCount(key); got != 2 {
		t.Fatalf("second incr = %d, want 2", got)
	}
}

func TestAlertEmptyIP(t *testing.T) {
	AlertLoginFailure("", "")
	AlertAuthFailure("", "")
}
