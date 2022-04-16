/*
 * Copyright 2022 Monime Ltd, licensed under the
 * Apache License, Version 2.0 (the "License");
 */

package gotasks

import (
	"context"
	"github.com/monime-lab/gotries"
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

var _ RunnerTask = Runnable(func(ctx context.Context) error {
	return nil
})

func (r Runnable) Name() string {
	return "func"
}

func (r Runnable) Run(ctx context.Context) (interface{}, error) {
	return nil, r(ctx)
}

func (f Callable) Name() string {
	return "func"
}

func (f Callable) Run(ctx context.Context) (interface{}, error) {
	return f(ctx)
}
