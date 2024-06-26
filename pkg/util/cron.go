package util

import (
	"context"
	"time"

	usync "github.com/mgtv-tech/redis-GunYu/pkg/sync"
)

func CronWithCtx(ctx context.Context, duration time.Duration, fn func(context.Context)) {
	usync.SafeGo(func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fn(ctx)
			}
		}
	}, nil)
}
