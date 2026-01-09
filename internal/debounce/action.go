package debounce

import (
	"sync"
	"time"
)

type Action[T any] struct {
	mu         sync.Mutex
	debouncer  *Debouncer
	delay      time.Duration
	handler    func(T)
	pending    T
	hasPending bool
}

func (a *Action[T]) Configure(delay time.Duration, handler func(T)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.delay = delay
	a.handler = handler
	if a.debouncer != nil {
		a.debouncer.Stop()
		a.debouncer = nil
	}
	a.clearPendingLocked()
}

func (a *Action[T]) Trigger(value T) {
	a.mu.Lock()
	if a.handler == nil {
		a.mu.Unlock()
		return
	}
	a.pending = value
	a.hasPending = true
	if a.debouncer == nil {
		a.debouncer = New(a.delay, a.flush)
	}
	deb := a.debouncer
	a.mu.Unlock()
	deb.Trigger()
}

func (a *Action[T]) SetPending(value T) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending = value
	a.hasPending = true
}

func (a *Action[T]) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.debouncer != nil {
		a.debouncer.Stop()
		a.debouncer = nil
	}
	a.clearPendingLocked()
}

func (a *Action[T]) Active() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.debouncer != nil
}

func (a *Action[T]) HasPending() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.hasPending
}

func (a *Action[T]) flush() {
	a.mu.Lock()
	if !a.hasPending {
		a.mu.Unlock()
		return
	}
	value := a.pending
	handler := a.handler
	a.clearPendingLocked()
	a.mu.Unlock()
	if handler != nil {
		handler(value)
	}
}

func (a *Action[T]) clearPendingLocked() {
	var zero T
	a.pending = zero
	a.hasPending = false
}
