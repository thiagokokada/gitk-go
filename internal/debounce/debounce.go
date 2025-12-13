package debounce

import (
	"sync/atomic"
	"time"
)

type Debouncer struct {
	delay time.Duration
	timer atomic.Pointer[time.Timer]
	fn    func()
}

func New(delay time.Duration, fn func()) *Debouncer {
	return &Debouncer{delay: delay, fn: fn}
}

func (d *Debouncer) Trigger() {
	timer := time.AfterFunc(d.delay, d.fn)
	if current := d.timer.Swap(timer); current != nil {
		current.Stop()
	}
}

func (d *Debouncer) Stop() {
	if current := d.timer.Swap(nil); current != nil {
		current.Stop()
	}
}
