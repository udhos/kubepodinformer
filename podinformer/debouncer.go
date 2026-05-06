package podinformer

import (
	"sync"
	"time"
)

// debouncer is a simple implementation of a debounce.
// It coalesces multiple calls to run() within a certain delay window.
// Only the last call is executed after the delay.
type debouncer struct {
	delay time.Duration
	mutex sync.Mutex
	timer *time.Timer
}

func newDebouncer(delay time.Duration) *debouncer {
	return &debouncer{delay: delay}
}

// run schedules a function to be executed after the delay.
// If the function is called multiple times within the delay window,
// only the last call is executed.
func (d *debouncer) run(f func()) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.timer != nil {
		return
	}

	d.timer = time.AfterFunc(d.delay, func() {
		d.mutex.Lock()
		d.timer = nil
		d.mutex.Unlock()

		f()
	})
}

func (d *debouncer) stop() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
}
