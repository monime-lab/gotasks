package gotasks

import (
	"context"
	"golang.org/x/sync/semaphore"
)

type noopPermit struct {
}

func (p noopPermit) Acquire(context.Context) error {
	return nil
}

func (p noopPermit) Release(context.Context) {
}

type semaphorePermit struct {
	worker *semaphore.Weighted
}

func newSemaphorePermit(n int) Permits {
	worker := semaphore.NewWeighted(int64(n))
	p := &semaphorePermit{worker: worker}
	return p
}

func (s *semaphorePermit) Acquire(ctx context.Context) error {
	return s.worker.Acquire(ctx, 1)
}

func (s *semaphorePermit) Release(context.Context) {
	s.worker.Release(1)
}
