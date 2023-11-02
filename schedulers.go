package gotasks

import (
	"context"
	"errors"
	"log"
	"math"
	"runtime"
	"sync"
)

var (
	once             sync.Once
	defaultScheduler Scheduler
)

func DefaultScheduler() Scheduler {
	once.Do(func() {
		defaultScheduler = NewScheduler(
			WithSchedulerName("DefaultScheduler"),
			WithSchedulerPoolSize(runtime.NumCPU()*8),
			WithSchedulerMaxPoolSize(math.MaxInt64),
		)
	})
	return defaultScheduler
}

func ScheduleTaskFunc(task func() error) {
	ScheduleTaskFuncWithContext(context.Background(), func(ctx context.Context) error {
		return task()
	})
}

func ScheduleTaskFuncWithContext(ctx context.Context, task func(ctx context.Context) error) {
	DefaultScheduler().Schedule(ctx, func(ctx context.Context) error {
		return task(ctx)
	}, 0).
		OnError(func(err error) {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Printf("ScheduleTask Error: %s", err)
		})
}
