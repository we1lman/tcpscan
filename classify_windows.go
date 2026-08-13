//go:build windows

package tcpscan

import (
	"errors"
	"syscall"
)

const (
	wsaeAccessDenied      = syscall.Errno(10013)
	wsaeNetworkDown       = syscall.Errno(10050)
	wsaeNetworkUnreach    = syscall.Errno(10051)
	wsaeNetworkReset      = syscall.Errno(10052)
	wsaeConnectionReset   = syscall.Errno(10054)
	wsaeTimedOut          = syscall.Errno(10060)
	wsaeConnectionRefused = syscall.Errno(10061)
	wsaeHostDown          = syscall.Errno(10064)
	wsaeHostUnreachable   = syscall.Errno(10065)
)

func classifySyscallError(err error) (State, bool) {
	switch {
	case errors.Is(err, wsaeConnectionRefused),
		errors.Is(err, wsaeConnectionReset):
		return StateClosed, true

	case errors.Is(err, wsaeHostUnreachable),
		errors.Is(err, wsaeNetworkUnreach),
		errors.Is(err, wsaeHostDown),
		errors.Is(err, wsaeNetworkDown),
		errors.Is(err, wsaeNetworkReset),
		errors.Is(err, wsaeAccessDenied):
		return StateUnreachable, true

	case errors.Is(err, wsaeTimedOut):
		return StateTimeout, true
	}

	return StateUnknown, false
}
