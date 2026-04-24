package kata

import (
	"context"
	"time"
)

// FetchWithDeadline returns a derived context with the given timeout and the
// cancel function that must be called by the caller.
func FetchWithDeadline(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, d)
	return ctx, cancel
}
