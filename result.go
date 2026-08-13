package tcpscan

import (
	"net"
	"time"
)

// State describes the outcome of a single port check.
type State uint8

// The states a port check can end up in.
const (
	// StateUnknown is the zero value and means the result was never filled in.
	StateUnknown State = iota

	// StateOpen means the TCP connection was established.
	StateOpen

	// StateClosed means the host answered but refused the connection,
	// so nothing is listening on that port.
	StateClosed

	// StateTimeout means the attempt ran out of time: the host stayed silent
	// for longer than the connect timeout, or a name lookup timed out.
	StateTimeout

	// StateUnreachable means the host or the network could not be reached.
	StateUnreachable

	// StateCanceled means the context was cancelled before the check finished.
	StateCanceled

	// StateError means the check failed for any other reason, including a
	// failure to resolve the target. The cause is available in Result.Err.
	StateError
)

var stateNames = [...]string{
	StateUnknown:     "unknown",
	StateOpen:        "open",
	StateClosed:      "closed",
	StateTimeout:     "timeout",
	StateUnreachable: "unreachable",
	StateCanceled:    "canceled",
	StateError:       "error",
}

// String returns the lower case name of the state, or "invalid" for a value
// outside the defined range.
func (s State) String() string {
	if int(s) >= len(stateNames) {
		return "invalid"
	}

	return stateNames[s]
}

// Result is the outcome of checking one port on one address.
//
// When a target cannot be resolved, a single Result is produced for the target
// as a whole: Host and Err are set, while IP, Port and Duration keep their zero
// values.
type Result struct {
	// Host is the target as it was passed to Scanner.Scan, with surrounding
	// space removed.
	Host string

	// IP is the address that was dialled. It differs from Host when the
	// target was a DNS name.
	IP net.IP

	// Port is the checked TCP port.
	Port uint16

	// State summarises what happened.
	State State

	// Duration is how long the connection attempt took.
	Duration time.Duration

	// Err is the error behind a non-open state, or nil. It keeps the whole
	// error chain, so errors.Is and errors.As work on it.
	Err error
}
