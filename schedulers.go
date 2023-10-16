package gotasks

import (
	"context"
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
	ScheduleTaskFuncWithContext(func(ctx context.Context) error {
		return task()
	})
}

func ScheduleTaskFuncWithContext(task func(ctx context.Context) error) {
	DefaultScheduler().Schedule(context.Background(), func(ctx context.Context) error {
		return task(ctx)
	}, 0).
		OnError(func(err error) {
			log.Printf("ScheduleTask Error: %s", err)
		})
}
