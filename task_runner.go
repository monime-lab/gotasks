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
	Runnable func(context.Context) error
	Callable func(context.Context) (interface{}, error)
	Permits  interface {
		Acquire(ctx context.Context) error
		Release(ctx context.Context)
	}
	Task interface {
		Name() string
		Run(context.Context) (interface{}, error)
	}
	TaskRunner interface {
		AddTask(task Task, options ...gotries.Option) TaskRunner
		AddRunnableTask(runnable Runnable, options ...gotries.Option) TaskRunner
		AddCallableTask(callable Callable, options ...gotries.Option) TaskRunner
		RunAndWaitAll(ctx context.Context) ([]interface{}, error)
		RunAndWaitAny(ctx context.Context) (interface{}, error)
	}
)

var _ Task = Runnable(func(ctx context.Context) error {
	return nil
})

func (f Runnable) Name() string {
	return "func"
}

func (f Runnable) Run(ctx context.Context) (interface{}, error) {
	return nil, f(ctx)
}

func (f Callable) Name() string {
	return "func"
}

func (f Callable) Run(ctx context.Context) (interface{}, error) {
	return f(ctx)
}
