// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT
package codexusage

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

// view is a fresh handler context boundary, not a request-owned HTTP lifetime.
// The owner supplies its own epoch cancellation and checks the live model key
// again before publishing. Closures are replaced on every relevant host event.
type view struct {
	connection func() (connection, error)
	key        func() string
	done       func() <-chan struct{}
	err        func() error
	status     func(string)
	notify     func(string, string)
}
type owner struct {
	mu                sync.Mutex
	client            *http.Client
	life              context.Context
	stopLife          context.CancelFunc
	epochCtx          context.Context
	stopEpoch         context.CancelFunc
	epoch, generation uint64
	latest            view
	refreshCancel     context.CancelFunc
	closing, started  bool
	jobs, background  sync.WaitGroup
	interval          time.Duration
}

func newOwner(client *http.Client, interval time.Duration) *owner {
	life, cancel := context.WithCancel(context.Background())
	epoch, stop := context.WithCancel(life)
	return &owner{client: client, life: life, stopLife: cancel, epochCtx: epoch, stopEpoch: stop, interval: interval}
}
func (o *owner) attach(v view, replace bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closing {
		return
	}
	if replace {
		o.epoch++
		o.generation++
		o.stopEpoch()
		o.epochCtx, o.stopEpoch = context.WithCancel(o.life)
	}
	o.latest = v
}
func (o *owner) start(v view) {
	o.mu.Lock()
	if o.started || o.closing {
		o.mu.Unlock()
		return
	}
	o.started = true
	o.background.Add(2)
	o.mu.Unlock()
	go func() {
		defer o.background.Done()
		ticker := time.NewTicker(o.interval)
		defer ticker.Stop()
		for {
			select {
			case <-o.life.Done():
				return
			case <-ticker.C:
				o.mu.Lock()
				latest := o.latest
				closing := o.closing
				o.mu.Unlock()
				if !closing {
					o.refresh(latest, false)
				}
			}
		}
	}()
	go func() {
		defer o.background.Done()
		for {
			select {
			case <-o.life.Done():
				return
			case <-v.done():
			}
			// SDK normal response publication changes Done from request cancellation
			// to runtime cancellation. Re-sample after normal completion, never treat
			// ordinary request cleanup as extension unload.
			if v.err() != nil {
				o.cancelOnly()
				o.jobs.Wait()
				o.client.CloseIdleConnections()
				return
			}
		}
	}()
}
func (o *owner) cancelOnly() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closing {
		return
	}
	o.closing = true
	o.epoch++
	o.generation++
	o.stopLife()
	o.stopEpoch()
	if o.refreshCancel != nil {
		o.refreshCancel()
	}
}
func (o *owner) shutdown() {
	o.cancelOnly()
	o.jobs.Wait()
	o.background.Wait()
	o.client.CloseIdleConnections()
}

type operation struct {
	epoch, generation uint64
	key               string
	ctx               context.Context
	cancel            context.CancelFunc
}

func (o *owner) begin(v view, refresh bool) (operation, bool) {
	o.mu.Lock()
	epoch, closing := o.epoch, o.closing
	o.mu.Unlock()
	if closing {
		return operation{}, false
	}
	key := v.key()
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closing || o.epoch != epoch || v.err() != nil {
		return operation{}, false
	}
	o.latest = v
	op := operation{epoch: o.epoch, key: key}
	op.ctx, op.cancel = context.WithCancel(o.epochCtx)
	if refresh {
		o.generation++
		op.generation = o.generation
		if o.refreshCancel != nil {
			o.refreshCancel()
		}
		o.refreshCancel = op.cancel
	}
	o.jobs.Add(1)
	return op, true
}
func (o *owner) liveLocked(op operation, key string, alive bool) bool {
	return !o.closing && o.epoch == op.epoch && op.ctx.Err() == nil && alive && op.key == key && (op.generation == 0 || op.generation == o.generation)
}
func (o *owner) publish(op operation, v view, action func()) bool {
	// Host metadata calls must not run beneath the owner lock: replacement
	// callbacks can themselves be awaiting the host's lifecycle lock.
	key, alive := v.key(), v.err() == nil
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.liveLocked(op, key, alive) {
		return false
	}
	action()
	return true
}
func requestScope(op operation, v view) func() {
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		for {
			select {
			case <-op.ctx.Done():
				return
			case <-v.done():
				if v.err() != nil {
					op.cancel()
					return
				}
			}
		}
	}()
	return func() { op.cancel(); <-joined }
}
func (o *owner) refresh(v view, explicit bool) {
	op, ok := o.begin(v, true)
	if !ok {
		return
	}
	defer o.jobs.Done()
	defer requestScope(op, v)()
	c, err := v.connection()
	var s status
	if err == nil {
		s, err = fetchStatus(op.ctx, o.client, c)
	}
	o.publish(op, v, func() {
		if err != nil {
			v.status("")
			if explicit {
				v.notify(c.scrub(err.Error()), "error")
			}
			return
		}
		v.status(formatStatusLine(s, time.Local))
		if explicit {
			v.notify(c.scrub(formatStatusCard(s, time.Local)), "info")
		}
	})
}
func (o *owner) reset(v view, args string) {
	op, ok := o.begin(v, false)
	if !ok {
		return
	}
	// Complete this operation before starting the success refresh so it does
	// not own or cancel the newly-created refresh's request context.
	succeeded := false
	func() {
		defer o.jobs.Done()
		defer requestScope(op, v)()
		id := trim(args)
		var text string
		var err error
		var c connection
		if id != "" && !validCreditID(id) {
			err = errors.New("Usage: /codex-reset [reset-id]")
		} else {
			c, err = v.connection()
			if err == nil {
				if id == "" {
					var credits []credit
					credits, err = fetchCredits(op.ctx, o.client, c)
					text = formatCredits(credits)
				} else {
					var result resetResult
					result, err = activateReset(op.ctx, o.client, c, id)
					text = formatResetResult(id, result)
					succeeded = err == nil
				}
			}
		}
		published := o.publish(op, v, func() {
			if err != nil {
				v.notify(c.scrub(err.Error()), "error")
			} else {
				v.notify(c.scrub(text), "info")
			}
		})
		succeeded = succeeded && published
	}()
	if succeeded {
		o.refresh(v, false)
	}
}
