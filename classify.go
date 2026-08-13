package tcpscan

import (
	"context"
	"errors"
	"net"
	"os"
)

func classify(ctx context.Context, err error) State {
	switch {
	case err == nil:
		return StateOpen
	case ctx.Err() != nil:
		return StateCanceled
	case errors.Is(err, context.Canceled):
		return StateCanceled
	case isTimeout(err):
		return StateTimeout
	}

	if state, ok := classifySyscallError(err); ok {
		return state
	}

	return StateError
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}

	var netErr net.Error

	return errors.As(err, &netErr) && netErr.Timeout()
}
