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
		Stop() Future
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
	stopCh      chan interface{}
	stopFunc    func()
	stop        bool
}

func newScheduledFuture() *futureImpl {
	return &futureImpl{
		errCh:  make(chan error),
		stopCh: make(chan interface{}, 1),
	}
}

func (f *futureImpl) Wait() error {
	for {
		select {
		case e, ok := <-f.errCh:
			if ok {
				f.lastWaitErr = e
			}
		case <-f.stopCh:
			return f.lastWaitErr
		}
	}
}

func (f *futureImpl) setStopFunc(stopFunc func()) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.stopFunc = stopFunc
}

func (f *futureImpl) Stop() Future {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.stop {
		return f
	}
	if f.stopFunc != nil {
		f.stopFunc()
	}
	close(f.errCh)
	close(f.stopCh)
	f.stop = true
	return f
}

func (f *futureImpl) hasStopped() bool {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.stop
}

func (f *futureImpl) writeErrorToChannel(err error, stop bool) {
	if !f.hasStopped() {
		select {
		case f.errCh <- err:
		default:
			// Drop this error as the channel is full
		}
		if stop {
			_ = f.Stop()
		}
	}
}
