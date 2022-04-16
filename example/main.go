//nolint
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/monime-lab/gotasks"
	"github.com/monime-lab/gotries"
	"log"
	"time"
)

func saveToStore1() error {
	return nil
}

func saveToStore2() error {
	return nil
}

func getUserByID(id int) (interface{}, error) {
	return fmt.Sprintf("user-%d", id), nil
}

func main() {
	runnerExampleOne()
	runnerExampleTwo()
	schedulerExampleOne()
	schedulerExampleTwo()
}

func runnerExampleOne() {
	_, err := gotasks.NewTaskRunner( /* Options here... */ ).
		AddRunnableTask(func(ctx context.Context) error {
			return saveToStore1()
		}).
		AddRunnableTask(func(ctx context.Context) error {
			return saveToStore2()
		}).RunAndWaitAny(context.TODO())
	if err != nil {
		panic(err)
	}
	log.Printf("At least one of them succeeds!!!")
}

func runnerExampleTwo() {
	runner := gotasks.NewTaskRunner(
		// This is a fail fast switch useful
		// when calling runner.RunAndWaitAll()
		// The call will return on the first failure
		gotasks.WithEagerFail(true),
		// The maximum parallelism. This is a
		// concurrency rate-limiter for when
		// the number of tasks can be high.
		// At any point, there are at most `max`
		// task (goroutines) running concurrently.
		// Value < 1 means unbounded parallelism
		gotasks.WithMaxParallelism(10),
		// Syntactic sugar to WithMaxParallelism(1).
		// Useful for executing multiple tasks serially
		gotasks.WithSequentialParallelism(),
		// Default retry options for all the added tasks
		gotasks.WithRetryOptions(
			// Retry all tasks twice...
			gotries.WithMaxAttempts(2),
		),
	)
	for i := 1; i <= 5; i++ {
		func(id int) {
			runner.AddCallableTask(func(ctx context.Context) (interface{}, error) {
				return getUserByID(id)
			}, gotries.WithTaskName(fmt.Sprintf("RunnerTask-%d", i)))
		}(i)
	}
	users, err := runner.RunAndWaitAll(context.TODO())
	if err != nil {
		// The error(s) are composed using https://github.com/uber-go/multierr
		log.Fatalf("At least one failed. Error: %s", err)
	}
	log.Printf("Users: %s", users)
}

func schedulerExampleOne() {
	_ = gotasks.DefaultScheduler().Schedule(context.Background(), func(ctx context.Context) error {
		println("Printed after 1 second")
		return nil
	}, 1*time.Second)
	_ = gotasks.DefaultScheduler().Schedule(context.Background(), func(ctx context.Context) error {
		println("Printed after 2 seconds")
		return nil
	}, 2*time.Second)
	future3 := gotasks.DefaultScheduler().Schedule(context.Background(), func(ctx context.Context) error {
		println("Printed after 5 seconds")
		return errors.New("error after printing: 'Printed after 5 seconds")
	}, 5*time.Second)
	if err := future3.Wait(); err != nil {
		log.Fatal(err)
	}
}

func schedulerExampleTwo() {
	future := gotasks.DefaultScheduler().ScheduleAtFixedRate(context.Background(), func(ctx context.Context) error {
		fmt.Printf("Running at: %s\n", time.Now().Format(time.RFC3339))
		return errors.New("oops!!! What's wrong")
	}, 0, 1*time.Second)
	go func() {
		time.Sleep(10 * time.Second)
		println("Stopping the scheduled action")
		future.Cancel()
	}()
	err := future.Wait()
	log.Printf(":::::::::::::::::::: Stopped. Err: %v", err)
}
