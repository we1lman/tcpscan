package tcpscan

import (
	"net"
	"time"
)

type State uint8

const (
	StateUnknown State = iota
	StateOpen
	StateClosed
	StateTimeout
	StateUnreachable
	StateCanceled
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

func (s State) String() string {
	if int(s) >= len(stateNames) {
		return "invalid"
	}
	return stateNames[s]
}

type Result struct {
	Host     string
	IP       net.IP
	Port     uint16
	State    State
	Duration time.Duration
	Err      error
}
