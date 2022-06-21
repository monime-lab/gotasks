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
		OnError(callback func(err error)) Future
		OnComplete(callback func()) Future
		OnCancel(callback func()) Future
		OnFinally(callback func()) Future
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
	onError     func(err error)
	oComplete   func()
	onCancel    func()
	onFinally   func()
}

func newScheduledFuture() *futureImpl {
	return &futureImpl{
		errCh:       make(chan error),
		doneChannel: make(chan interface{}, 1),
	}
}

func (f *futureImpl) OnError(callback func(err error)) Future {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.onError = callback
	return f
}

func (f *futureImpl) OnComplete(callback func()) Future {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.oComplete = callback
	return f
}

func (f *futureImpl) OnCancel(callback func()) Future {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.onCancel = callback
	return f
}

func (f *futureImpl) OnFinally(callback func()) Future {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.onFinally = callback
	return f
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
	if f.onFinally != nil {
		f.onFinally()
	}
	f.cancel = true
	close(f.errCh)
	close(f.doneChannel)
	if f.onCancel != nil {
		f.onCancel()
	}
	return f
}

func (f *futureImpl) complete(err error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.completed || f.cancel {
		return
	}
	f.completed = true
	close(f.errCh)
	close(f.doneChannel)
	if err == nil && f.oComplete != nil {
		f.oComplete()
	} else if err != nil && f.onError != nil {
		f.onError(err)
	}
	if f.onFinally != nil {
		f.onFinally()
	}
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
			f.complete(err)
		}
	}
}
