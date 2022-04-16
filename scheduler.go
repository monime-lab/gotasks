/*
 * Copyright 2022 Monime Ltd, licensed under the
 * Apache License, Version 2.0 (the "License");
 */

package gotasks

import (
	"context"
	"sync"
	"time"
)

type (
	Future interface {
		Cancel() Future
		Wait() error
	}
	OnStopCallback   func()
	RunnableCallback func(error)
	CallableCallback func(interface{}, error)
	Scheduler        interface {
		Schedule(ctx context.Context, runnable Runnable, delay time.Duration) Future
		ScheduleAtFixedRate(ctx context.Context, runnable Runnable, initialDelay, interval time.Duration) Future
	}
)

var (
	_ Future = &futureImpl{}
)

type futureImpl struct {
	lock        sync.RWMutex
	errCh       chan error
	lastWaitErr error
	doneChannel chan interface{}
	cancelFunc  func()
	cancel      bool
	completed   bool
}

func newScheduledFuture() *futureImpl {
	return &futureImpl{
		errCh:       make(chan error),
		doneChannel: make(chan interface{}, 1),
	}
}

func (f *futureImpl) Wait() error {
	for {
		select {
		case e, ok := <-f.errCh:
			if ok {
				f.lastWaitErr = e
			}
		case <-f.doneChannel:
			return f.lastWaitErr
		}
	}
}

func (f *futureImpl) setCancelFunc(cancelFunc func()) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.cancelFunc = cancelFunc
}

func (f *futureImpl) Cancel() Future {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.cancel || f.completed {
		return f
	}
	if f.cancelFunc != nil {
		f.cancelFunc()
	}
	close(f.errCh)
	close(f.doneChannel)
	f.cancel = true
	return f
}

func (f *futureImpl) complete() {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.completed || f.cancel {
		return
	}
	close(f.errCh)
	close(f.doneChannel)
	f.completed = true
}

func (f *futureImpl) isCancelled() bool {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.cancel
}

func (f *futureImpl) writeResultToChannel(err error, complete bool) {
	if !f.isCancelled() {
		select {
		case f.errCh <- err:
		default:
			// Drop this error as the channel is full
		}
		if complete {
			f.complete()
		}
	}
}
