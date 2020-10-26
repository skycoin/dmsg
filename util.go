package dmsg

import (
	"context"
)

func awaitDone(ctx context.Context, done chan struct{}) {
	select {
	case <-ctx.Done():
	case <-done:
	}
}

func isClosed(done chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}
