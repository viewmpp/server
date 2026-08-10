package background

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const sweepTimeout = 30 * time.Second

func Sweep(stop <-chan struct{}, logger *slog.Logger, name string, every time.Duration, fn func(context.Context) (int64, error)) {
	sweep := func() {
		ctx, cancel := context.WithTimeout(context.Background(), sweepTimeout)
		defer cancel()

		n, err := fn(ctx)
		if err != nil {
			logger.Error("sweep failed", "name", name, "err", err)
			return
		}
		if n > 0 {
			logger.Info("expired rows removed", "name", name, "count", n)
		}
	}

	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()

		sweep()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()
}

func Run(wg *sync.WaitGroup, logger *slog.Logger, fn func()) {
	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			if err := recover(); err != nil {
				logger.Error("panic recovering", "err", fmt.Errorf("%v", err))
			}
		}()
		fn()
	}()
}
