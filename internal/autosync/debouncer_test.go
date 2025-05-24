package autosync

import (
	"sync"
	"testing"
	"time"
)

func TestDebouncer_Add(t *testing.T) {
	debouncer := NewDebouncer(50 * time.Millisecond)
	defer debouncer.Stop()

	called := false
	debouncer.Add("test", func() {
		called = true
	})

	// Wait for timer to fire
	time.Sleep(100 * time.Millisecond)

	if !called {
		t.Error("Expected callback to be called")
	}
}

func TestDebouncer_Debounce(t *testing.T) {
	debouncer := NewDebouncer(50 * time.Millisecond)
	defer debouncer.Stop()

	callCount := 0
	var mu sync.Mutex

	// Add same key multiple times quickly
	for i := 0; i < 5; i++ {
		debouncer.Add("test", func() {
			mu.Lock()
			callCount++
			mu.Unlock()
		})
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce period to expire
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	finalCount := callCount
	mu.Unlock()

	if finalCount != 1 {
		t.Errorf("Expected callback to be called once, but was called %d times", finalCount)
	}
}

func TestDebouncer_Cancel(t *testing.T) {
	debouncer := NewDebouncer(50 * time.Millisecond)
	defer debouncer.Stop()

	called := false
	debouncer.Add("test", func() {
		called = true
	})

	// Cancel before timer fires
	debouncer.Cancel("test")

	// Wait longer than debounce period
	time.Sleep(100 * time.Millisecond)

	if called {
		t.Error("Expected callback to not be called after cancellation")
	}
}

func TestDebouncer_Pending(t *testing.T) {
	debouncer := NewDebouncer(100 * time.Millisecond)
	defer debouncer.Stop()

	if debouncer.Pending() != 0 {
		t.Error("Expected no pending callbacks initially")
	}

	debouncer.Add("test1", func() {})
	debouncer.Add("test2", func() {})

	if debouncer.Pending() != 2 {
		t.Errorf("Expected 2 pending callbacks, got %d", debouncer.Pending())
	}

	// Wait for timers to fire
	time.Sleep(150 * time.Millisecond)

	if debouncer.Pending() != 0 {
		t.Errorf("Expected no pending callbacks after execution, got %d", debouncer.Pending())
	}
}

func TestDebouncer_Stop(t *testing.T) {
	debouncer := NewDebouncer(50 * time.Millisecond)

	called := false
	debouncer.Add("test", func() {
		called = true
	})

	debouncer.Stop()

	// Wait longer than debounce period
	time.Sleep(100 * time.Millisecond)

	if called {
		t.Error("Expected callback to not be called after stop")
	}

	if debouncer.Pending() != 0 {
		t.Error("Expected no pending callbacks after stop")
	}
}
