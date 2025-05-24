package autosync

import (
	"sync"
	"time"
)

// Debouncer groups rapid changes to the same file path
type Debouncer struct {
	delay     time.Duration
	timers    map[string]*time.Timer
	callbacks map[string]func()
	mutex     sync.RWMutex
}

// NewDebouncer creates a new debouncer with the specified delay
func NewDebouncer(delay time.Duration) *Debouncer {
	return &Debouncer{
		delay:     delay,
		timers:    make(map[string]*time.Timer),
		callbacks: make(map[string]func()),
	}
}

// Add schedules a callback to be executed after the delay
// If the same key is added again before the timer expires, the timer is reset
func (d *Debouncer) Add(key string, callback func()) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Cancel existing timer if it exists
	if timer, exists := d.timers[key]; exists {
		timer.Stop()
	}

	// Store the callback
	d.callbacks[key] = callback

	// Create a new timer
	d.timers[key] = time.AfterFunc(d.delay, func() {
		d.mutex.Lock()
		defer d.mutex.Unlock()

		// Execute callback if it still exists
		if cb, exists := d.callbacks[key]; exists {
			cb()
			delete(d.callbacks, key)
			delete(d.timers, key)
		}
	})
}

// Cancel removes a pending callback
func (d *Debouncer) Cancel(key string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if timer, exists := d.timers[key]; exists {
		timer.Stop()
		delete(d.timers, key)
		delete(d.callbacks, key)
	}
}

// Stop cancels all pending callbacks and clears internal state
func (d *Debouncer) Stop() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Stop all timers
	for _, timer := range d.timers {
		timer.Stop()
	}

	// Clear maps
	d.timers = make(map[string]*time.Timer)
	d.callbacks = make(map[string]func())
}

// Pending returns the number of pending callbacks
func (d *Debouncer) Pending() int {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return len(d.callbacks)
}
