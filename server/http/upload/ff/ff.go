package ff

import (
	"context"
	"runtime"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

const waitDura = 5 * time.Second

var sema = semaphore.NewWeighted(int64(runtime.GOMAXPROCS(-1) * 2))

func acq() error {
	ctx, cancel := context.WithTimeout(context.Background(), waitDura)
	defer cancel()

	err := sema.Acquire(ctx, 1)
	return errors.Wrap(err, "Failed to wait for pending jobs")
}
