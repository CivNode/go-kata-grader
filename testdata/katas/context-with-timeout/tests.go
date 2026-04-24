package kata

import (
	"context"
	"testing"
	"time"
)

func TestFetchWithDeadline_ReturnsCancelled(t *testing.T) {
	ctx, cancel := FetchWithDeadline(context.Background(), 10*time.Millisecond)
	defer cancel()
	<-ctx.Done()
	if ctx.Err() == nil {
		t.Fatal("expected ctx.Err to be set")
	}
}
