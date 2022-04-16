/*
 * Copyright 2022 Monime Ltd, licensed under the
 * Apache License, Version 2.0 (the "License");
 */

package gotasks

import (
	"context"
	"fmt"
	"github.com/monime-lab/gotries"
	"sync/atomic"
)

type (
	Permits interface {
		Acquire(ctx context.Context) error
		Release(ctx context.Context)
	}
	RunnerTask interface {
		Name() string
		Run(context.Context) (interface{}, error)
	}
	Callable   func(ctx context.Context) (interface{}, error)
	TaskRunner interface {
		AddTask(task RunnerTask, options ...gotries.Option) TaskRunner
		AddRunnableTask(runnable Runnable, options ...gotries.Option) TaskRunner
		AddCallableTask(callable Callable, options ...gotries.Option) TaskRunner
		RunAndWaitAll(ctx context.Context) ([]interface{}, error)
		RunAndWaitAny(ctx context.Context) (interface{}, error)
	}
)

var (
	functionRunnableIndex uint64
	functionCallableIndex uint64
	_                     RunnerTask = Runnable(func(ctx context.Context) error {
		return nil
	})
	_ RunnerTask = Callable(func(ctx context.Context) (interface{}, error) {
		return 0, nil
	})
)

func (r Runnable) Name() string {
	id := atomic.AddUint64(&functionRunnableIndex, 1)
	return fmt.Sprintf("runnable-task-%d", id)
}

func (r Runnable) Run(ctx context.Context) (interface{}, error) {
	return nil, r(ctx)
}

func (c Callable) Name() string {
	id := atomic.AddUint64(&functionCallableIndex, 1)
	return fmt.Sprintf("callable-task-%d", id)
}

func (c Callable) Run(ctx context.Context) (interface{}, error) {
	return c(ctx)
}
