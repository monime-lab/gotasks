package gotasks

import (
	"github.com/monime-lab/gotries"
)

type (
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

// WithEagerFail is a fail fast switch useful
// when calling runner.RunAndWaitAll()
// The call will return on the first failure
func WithEagerFail(enable bool) RunnerOption {
	return runnerOptionFunc(func(r *taskRunner) {
		r.eagerFail = enable
	})
}

// WithRetryOptions sets the default retry options for all the added tasks
func WithRetryOptions(options ...gotries.Option) RunnerOption {
	return runnerOptionFunc(func(r *taskRunner) {
		r.retryOptions = append(r.retryOptions, options...)
	})
}

// WithSequentialParallelism is a syntactic sugar to WithMaxParallelism(1).
// Useful for executing multiple tasks serially
func WithSequentialParallelism() RunnerOption {
	return WithMaxParallelism(1)
}

// WithMaxParallelism sets the maximum parallelism for the runner;
// if max is less than 1, then no parallelism limit is set
// This is a concurrency rate-limiter for when the number of tasks
// can be high. At any point, there are at most `max`
// task (goroutines) running concurrently.
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
