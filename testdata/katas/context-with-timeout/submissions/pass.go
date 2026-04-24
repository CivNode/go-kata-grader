package kata

import (
	"context"
	"time"
)

func FetchWithDeadline(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, d)
	return ctx, cancel
}
