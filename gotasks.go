package gotasks

import (
	"context"
)

type (
	Runnable func(ctx context.Context) error
)
