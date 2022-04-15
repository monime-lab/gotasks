[![Go Report Card](https://goreportcard.com/badge/github.com/monime-lab/gotasks)](https://goreportcard.com/report/github.com/monime-lab/gotasks)
[![LICENSE](https://img.shields.io/badge/License-Apache%202-blue.svg)](https://github.com/monime-lab/gotasks/blob/main/LICENSE)

# gotasks

A simple, flexible and production inspired golang retry library

### Install

```bash
go get -u github.com/monime-lab/gotasks
```

### Sample

```go
package main

import (
	"context"
	"fmt"
	"github.com/monime-lab/gotasks"
	"github.com/monime-lab/gotries"
	"log"
)

func saveToStore1() error {
	return nil
}

func saveToStore2() error {
	return nil
}

func getUserByID(_ int) (interface{}, error) {
	return struct{}{}, nil
}

func main() {
	exampleOne()
	exampleTwo()
}

func exampleOne() {
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

func exampleTwo() {
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
	for i := 0; i < 10; i++ {
		func(n int) {
			runner.AddCallableTask(func(ctx context.Context) (interface{}, error) {
				return getUserByID(1)
			}, gotries.WithTaskName(fmt.Sprintf("Task-%d", i)))
		}(i)
	}
	users, err := runner.RunAndWaitAll(context.TODO())
	if err != nil {
		// The error(s) are composed using https://github.com/uber-go/multierr
		log.Fatalf("At least one failed. Error: %s", err)
	}
	log.Printf("Users: %s", users)
}

```

## Contribute

For issues, comments, recommendation or feedback please [do it here](https://github.com/monime-lab/gotries/issues).

Contributions are highly welcome.

:thumbsup:
