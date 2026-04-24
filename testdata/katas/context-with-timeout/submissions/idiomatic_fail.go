package kata

import (
	"context"
	"time"
)

// Missed the cancel return — still calls WithTimeout but throws the cancel
// away. The required "binds-cancel" assign shape does not match.
func FetchWithDeadline(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, _ := context.WithTimeout(parent, d)
	return ctx, func() {}
}
