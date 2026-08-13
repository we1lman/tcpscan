//go:build unix

package tcpscan

import (
	"errors"
	"syscall"
)

func classifySyscallError(err error) (State, bool) {
	switch {
	case errors.Is(err, syscall.ECONNREFUSED),
		errors.Is(err, syscall.ECONNRESET):
		return StateClosed, true

	case errors.Is(err, syscall.EHOSTUNREACH),
		errors.Is(err, syscall.ENETUNREACH),
		errors.Is(err, syscall.EHOSTDOWN),
		errors.Is(err, syscall.ENETDOWN),
		errors.Is(err, syscall.EACCES),
		errors.Is(err, syscall.EPERM):
		return StateUnreachable, true
	}

	return StateUnknown, false
}
