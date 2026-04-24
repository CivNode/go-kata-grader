package kata

import (
	"context"
	"time"
)

// Wrong: sleeps instead of using the context channel.
func FetchWithDeadline(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, d)
	time.Sleep(d)
	return ctx, cancel
}
