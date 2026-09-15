package alert

import (
	"sync"
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

func TestConcurrentAlertCount(t *testing.T) {
	alertCache.Flush()
	key := "test:concurrent"
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			incrAlertCount(key)
		}()
	}
	wg.Wait()
	if got := incrAlertCount(key); got < 10 {
		t.Fatalf("concurrent incr count = %d, want >= 10", got)
	}
}
