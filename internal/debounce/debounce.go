package debounce

import (
	"sync"
	"time"
)

var afterFunc = time.AfterFunc

type Debouncer struct {
	mu    sync.Mutex
	delay time.Duration
	timer *time.Timer
	fn    func()
	gen   uint64
}

func New(delay time.Duration, fn func()) *Debouncer {
	return &Debouncer{delay: delay, fn: fn}
}

// Ensure returns the provided debouncer if non-nil, otherwise allocates and
// assigns a new one using the supplied delay and callback. The caller must
// provide appropriate synchronization around target.
func Ensure(target **Debouncer, delay time.Duration, fn func()) *Debouncer {
	if *target == nil {
		*target = New(delay, fn)
	}
	return *target
}

func (d *Debouncer) Trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.gen++
	gen := d.gen
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = afterFunc(d.delay, func() {
		d.mu.Lock()
		ok := gen == d.gen
		d.mu.Unlock()
		if !ok {
			return
		}
		d.fn()
	})
}

func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.gen++
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
