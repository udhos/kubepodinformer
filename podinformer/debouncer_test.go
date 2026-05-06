package podinformer

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncer_Throttling(t *testing.T) {
	delay := 100 * time.Millisecond
	d := newDebouncer(delay)

	var calls atomic.Int32

	// Fire run() 10 times in a tight loop.
	for range 10 {
		d.run(func() {
			calls.Add(1)
		})
	}

	// At this point, no calls should have executed yet because of the delay
	if c := calls.Load(); c != 0 {
		t.Errorf("expected 0 calls immediately, got %d", c)
	}

	// Wait for the delay to pass
	time.Sleep(delay + 50*time.Millisecond)

	// We should only have exactly 1 call executed
	if c := calls.Load(); c != 1 {
		t.Errorf("expected exactly 1 call after delay, got %d", c)
	}

	// If we run again after the window, it should execute exactly 1 more time
	d.run(func() {
		calls.Add(1)
	})
	time.Sleep(delay + 50*time.Millisecond)

	if c := calls.Load(); c != 2 {
		t.Errorf("expected exactly 2 calls total, got %d", c)
	}
}

func TestDebouncer_Stop(t *testing.T) {
	delay := 100 * time.Millisecond
	d := newDebouncer(delay)

	var calls atomic.Int32

	d.run(func() {
		calls.Add(1)
	})

	// Stop it before it can execute
	d.stop()

	time.Sleep(delay + 50*time.Millisecond)

	if c := calls.Load(); c != 0 {
		t.Errorf("expected 0 calls because it was stopped, got %d", c)
	}
}

func TestDebouncer_ContinuousUpdates(t *testing.T) {
	delay := 100 * time.Millisecond
	d := newDebouncer(delay)

	var calls atomic.Int32

	done := make(chan struct{})
	go func() {
		// send updates every 20ms for 350ms total
		for range 17 {
			d.run(func() {
				calls.Add(1)
			})
			time.Sleep(20 * time.Millisecond)
		}
		close(done)
	}()

	<-done
	time.Sleep(delay + 50*time.Millisecond) // wait for the last timer to potentially fire

	// A pure debounce would never fire (0 calls) because 20ms < 100ms delay.
	// No debouncing would fire 17 times.
	// Since we are throttling, we expect it to fire at intervals (approximately at 100ms, 200ms, 300ms, 400ms).
	c := calls.Load()
	if c < 3 || c > 5 {
		t.Errorf("expected between 3 and 5 calls for continuous updates, got %d", c)
	}
}
