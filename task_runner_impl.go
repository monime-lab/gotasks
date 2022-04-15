/*
 * Copyright (C) 2021, Monime Ltd, All Rights Reserved.
 * Unauthorized copy or sharing of this file through
 * any medium is strictly not allowed.
 */

package goconc

import (
	"context"
	"errors"
	"github.com/piehlabs/gotries"
	"go.uber.org/multierr"
	"sync"
)

var (
	_          TaskRunner = &taskRunner{}
	ErrNoTasks            = errors.New("no tasks in runner")
)

type (
	taskRunner struct {
		lock             sync.Mutex
		eagerFail        bool
		permits          Permits
		tasks            []Task
		retryOptions     []gotries.Option
		taskRetryOptions [][]gotries.Option
	}
	RunnerOption interface {
		apply(r *taskRunner)
	}
	TaskOption interface {
		apply(r *taskRunner)
	}
	runnerOptionFunc func(r *taskRunner)
)

func (f runnerOptionFunc) apply(r *taskRunner) {
	f(r)
}

func WithEagerFailEnable(enable bool) RunnerOption {
	return runnerOptionFunc(func(r *taskRunner) {
		r.eagerFail = enable
	})
}

func WithRetryOptions(options ...gotries.Option) RunnerOption {
	return runnerOptionFunc(func(r *taskRunner) {
		r.retryOptions = append(r.retryOptions, options...)
	})
}

func WithSequentialParallelism() RunnerOption {
	return WithMaxParallelism(1)
}

// WithMaxParallelism sets the maximum parallelism for the runner;
// if max is less than 1, then no parallelism limit is set
func WithMaxParallelism(max int) RunnerOption {
	var permit Permits = noopPermit{}
	if max >= 1 {
		permit = newSemaphorePermit(max)
	}
	return runnerOptionFunc(func(r *taskRunner) {
		r.permits = permit
	})
}

func NewTaskRunner(options ...RunnerOption) TaskRunner {
	runner := &taskRunner{
		eagerFail:        false,
		permits:          noopPermit{},
		tasks:            make([]Task, 0),
		taskRetryOptions: make([][]gotries.Option, 0),
	}
	for _, option := range options {
		option.apply(runner)
	}
	return runner
}

func (r *taskRunner) AddRunnableTask(runnable Runnable, options ...gotries.Option) TaskRunner {
	return r.AddTask(runnable, options...)
}

func (r *taskRunner) AddCallableTask(callable Callable, options ...gotries.Option) TaskRunner {
	return r.AddTask(callable, options...)
}

func (r *taskRunner) AddTask(task Task, options ...gotries.Option) TaskRunner {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.tasks = append(r.tasks, task)
	options = r.defaultRetry(options)
	r.taskRetryOptions = append(r.taskRetryOptions, options)
	return r
}

func (r *taskRunner) RunAndWaitAll(ctx context.Context) ([]interface{}, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	taskCount := len(r.tasks)
	if taskCount == 0 {
		return nil, ErrNoTasks
	}
	var err error
	cancel, resultChan, errChan := r.runTasks(ctx)
	defer cancel()
	results := make([]interface{}, taskCount)
	for i := 0; i < taskCount; i++ {
		select {
		case res := <-resultChan:
			results[res.position] = res.value
		case err2 := <-errChan:
			err = multierr.Append(err, err2)
			if r.eagerFail {
				return nil, err
			}
		case <-ctx.Done():
			err = multierr.Append(err, ctx.Err())
		}
	}
	return results, err
}

func (r *taskRunner) RunAndWaitAny(ctx context.Context) (interface{}, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	taskCount := len(r.tasks)
	if taskCount == 0 {
		return nil, ErrNoTasks
	}
	var err error
	cancel, resultChan, errChan := r.runTasks(ctx)
	defer cancel()
	for i := 0; i < taskCount; i++ {
		select {
		case res := <-resultChan:
			return res.value, nil
		case err2 := <-errChan:
			err = multierr.Append(err, err2)
		case <-ctx.Done():
			err = multierr.Append(err, ctx.Err())
		}
	}
	return nil, err
}

func (r *taskRunner) runTasks(ctx context.Context) (context.CancelFunc, chan result, chan error) {
	wg := &sync.WaitGroup{}
	errChan := make(chan error, len(r.tasks))
	resultChan := make(chan result, len(r.tasks))
	runCtx, cancel := context.WithCancel(ctx)
	for pos, task := range r.tasks {
		options := r.taskRetryOptions[pos]
		wrapper := &taskWrapper{
			pos: pos, task: task,
			retryOptions: options,
		}
		// limits goroutine creation with the permit
		if err := r.permits.Acquire(runCtx); err != nil {
			errChan <- err
			continue
		}
		wg.Add(1)
		go r.runTask(runCtx, wg, wrapper, resultChan, errChan)
	}
	go func() {
		wg.Wait()
		close(errChan)
		close(resultChan)
	}()
	return cancel, resultChan, errChan
}

func (r *taskRunner) runTask(
	ctx context.Context, wg *sync.WaitGroup,
	wrapper *taskWrapper, resultChan chan result, errChan chan error,
) {
	defer func(c context.Context) {
		r.permits.Release(c)
		wg.Done()
	}(ctx)
	res, err := gotries.Call(ctx,
		func(state gotries.State) (interface{}, error) {
			return wrapper.task.Run(state.Context())
		}, wrapper.retryOptions...)
	// Don't look for nil result as an indication of success,
	// AddRunnableTask tasks are wrapped to return nil result on success
	if err != nil {
		errChan <- err
	} else {
		resultChan <- result{position: wrapper.pos, value: res}
	}
}

func (r *taskRunner) defaultRetry(options []gotries.Option) []gotries.Option {
	options = append(r.retryOptions, options...)
	if len(options) == 0 {
		// if no retry options where set, disable retry
		options = append(options, gotries.WithMaxAttempts(0))
	}
	return options
}

type taskWrapper struct {
	pos          int
	task         Task
	retryOptions []gotries.Option
}

type result struct {
	position int
	value    interface{}
}
