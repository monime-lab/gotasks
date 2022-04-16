/*
 * Copyright (C) 2021, Monime Ltd, All Rights Reserved.
 * Unauthorized copy or sharing of this file through
 * any medium is strictly not allowed.
 */

package gotasks

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/semaphore"
	"log"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	once                 sync.Once
	schedulerID          uint64
	defaultScheduler     Scheduler
	_                    Scheduler = &schedulerImpl{}
	ErrSchedulerStopping           = errors.New("scheduler is stopping")
	ErrSchedulerStopped            = errors.New("scheduler is stopped")
)

type (
	SchedulerOption interface {
		appSchedulerOption(s *schedulerImpl)
	}
	schedulerOptionFunc func(s *schedulerImpl)
)

func (f schedulerOptionFunc) appSchedulerOption(s *schedulerImpl) {
	f(s)
}

func DefaultScheduler() Scheduler {
	once.Do(func() {
		defaultScheduler = NewScheduler(
			WithSchedulerName("DefaultScheduler"),
			WithSchedulerPoolSize(runtime.NumCPU()*4),
			WithSchedulerMaxPoolSize(math.MaxInt64),
		)
	})
	return defaultScheduler
}

func WithSchedulerName(name string) SchedulerOption {
	return schedulerOptionFunc(func(s *schedulerImpl) {
		s.name = name
	})
}

func WithSchedulerPoolSize(size int) SchedulerOption {
	if size < 1 {
		panic("Pool size cannot be < 1")
	}
	return schedulerOptionFunc(func(s *schedulerImpl) {
		s.poolSize = size
	})
}

func WithSchedulerMaxPoolSize(max int) SchedulerOption {
	if max < 1 {
		panic("Max pool size cannot be < 1")
	}
	return schedulerOptionFunc(func(s *schedulerImpl) {
		s.maxPoolSize = max
	})
}

func NewScheduler(options ...SchedulerOption) Scheduler {
	s := &schedulerImpl{
		tasks:       list.New(),
		poolSize:    runtime.NumCPU(),
		maxPoolSize: 1000,
	}
	for _, option := range options {
		option.appSchedulerOption(s)
	}
	if s.name == "" {
		id := atomic.AddUint64(&schedulerID, 1)
		s.name = fmt.Sprintf("scheduler-%d", id)
	}
	if s.maxPoolSize < s.poolSize {
		panic("the max pool size cannot be less than the pool size")
	}
	s.wakeupChannel = make(chan interface{}, s.poolSize)
	s.semaphore = semaphore.NewWeighted(int64(s.maxPoolSize))
	s.closeCtx, s.closeFunc = context.WithCancel(context.Background())
	return s.initialize()
}

type schedulerImpl struct {
	poolSize      int
	maxPoolSize   int
	stopped       bool
	stopping      bool
	name          string
	mu            sync.Mutex
	stopMu        sync.Mutex
	tasks         *list.List
	wg            sync.WaitGroup
	wakeupChannel chan interface{}
	semaphore     *semaphore.Weighted
	closeCtx      context.Context
	closeFunc     context.CancelFunc
}

func (s *schedulerImpl) initialize() Scheduler {
	for i := 0; i < s.poolSize; i++ {
		s.wg.Add(1)
		go s.runTasks(i)
	}
	return s
}

func (s *schedulerImpl) Schedule(ctx context.Context, runnable Runnable, delay time.Duration) Future {
	if err := s.ensureRunning(); err != nil {
		panic(err)
	}
	future := newScheduledFuture()
	future.setCancelFunc(s.addTask(ctx, future, func(ctx context.Context) error {
		err := runnable(ctx)
		return err
	}, true, delay))
	return future
}

func (s *schedulerImpl) ScheduleAtFixedRate(
	ctx context.Context, runnable Runnable, startDelay, interval time.Duration) Future {
	future := newScheduledFuture()
	future.setCancelFunc(s.scheduleAtFixedRate(ctx, future, runnable, startDelay, interval))
	return future
}

func (s *schedulerImpl) scheduleAtFixedRate(ctx context.Context, future *futureImpl,
	runnable Runnable, firstDelay, consecutiveDelay time.Duration) func() {
	if future.isCancelled() {
		return nil
	}
	return s.addTask(ctx, future, func(ctx context.Context) error {
		defer s.scheduleAtFixedRate(ctx, future, runnable, consecutiveDelay, consecutiveDelay)
		return runnable(ctx)
	}, false, firstDelay)
}

func (s *schedulerImpl) addTask(ctx context.Context,
	future *futureImpl, runnable Runnable, oneTime bool, delay time.Duration) func() {
	if err := s.semaphore.Acquire(ctx, 1); err != nil {
		log.Printf("Unable to acquire permit in order to enqueue task. Error: %s\n", err)
		future.writeResultToChannel(err, oneTime)
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t := &task{
		future:   future,
		context:  ctx,
		oneTime:  oneTime,
		runnable: runnable,
		execTime: time.Now().Add(delay),
	}
	el := s.tasks.PushBack(t)
	return func(el *list.Element) func() {
		return func() {
			t.cancel()
			s.mu.Lock()
			defer s.mu.Unlock()
			s.tasks.Remove(el)
		}
	}(el)
}

func (s *schedulerImpl) ensureRunning() error {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	if s.stopped {
		return ErrSchedulerStopped
	}
	if s.stopping {
		return ErrSchedulerStopping
	}
	return nil
}

func (s *schedulerImpl) Stop() error {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	if s.stopping {
		log.Println("Stop already called")
		return nil
	}
	s.stopping = true
	s.closeFunc()
	s.wg.Wait()
	close(s.wakeupChannel)
	s.stopped = true
	return nil
}

func (s *schedulerImpl) pollTask() *task {
	s.mu.Lock()
	defer s.mu.Unlock()
	for element := s.tasks.Front(); element != nil; element = element.Next() {
		//nolint:forcetypeassert
		t := element.Value.(*task)
		if t.isCancelled() || t.readyForExecution() {
			s.tasks.Remove(element)
			return t
		}
	}
	return nil
}

func (s *schedulerImpl) runTasks(index int) {
	for {
		select {
		case <-s.closeCtx.Done():
			log.Printf("Scheduler goroutine %s-%d stopping", s.name, index)
			s.wg.Done()
			return
		default:
			if t := s.pollTask(); t != nil {
				t.run(s.semaphore)
				continue
			}
			time.Sleep(1 * time.Millisecond)
		}
	}
}

type task struct {
	cancel2  bool
	oneTime  bool
	lock     sync.Mutex
	execTime time.Time

	runnable Runnable
	future   *futureImpl
	context  context.Context
}

func (t *task) cancel() {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.cancel2 = true
}

func (t *task) isCancelled() bool {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.cancel2
}

func (t *task) readyForExecution() bool {
	now := time.Now()
	return now.After(t.execTime) || now.Equal(t.execTime)
}

func (t *task) run(semaphore *semaphore.Weighted) {
	defer func() {
		semaphore.Release(1)
		if r := recover(); r != nil {
			var err error
			switch v := r.(type) {
			case string:
				err = errors.New(v)
			case error:
				err = v
			default:
				err = fmt.Errorf("%v", r)
			}
			log.Println("Panic on running task. Error: %", err)
			t.future.writeResultToChannel(err, t.oneTime)
		}
	}()
	if !t.isCancelled() {
		err := t.runnable(t.context)
		t.future.writeResultToChannel(err, t.oneTime)
	}
}
