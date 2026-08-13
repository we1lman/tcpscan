package tcpscan

import (
	"errors"
	"fmt"
)

var (
	ErrNoPorts      = errors.New("tcpscan: no ports specified")
	ErrInvalidPort  = errors.New("tcpscan: invalid port")
	ErrInvalidRange = errors.New("tcpscan: invalid port range")

	ErrNoTargets     = errors.New("tcpscan: no targets specified")
	ErrInvalidTarget = errors.New("tcpscan: invalid target")

	ErrInvalidOption = errors.New("tcpscan: invalid option")
)

type TargetError struct {
	Input string
	Err   error
}

func (e *TargetError) Error() string {
	return fmt.Sprintf("target %q: %v", e.Input, e.Err)
}

func (e *TargetError) Unwrap() error {
	return e.Err
}
