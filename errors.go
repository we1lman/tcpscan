package tcpscan

import "errors"

var (
	ErrNoPorts      = errors.New("tcpscan: no ports specified")
	ErrInvalidPort  = errors.New("tcpscan: invalid port")
	ErrInvalidRange = errors.New("tcpscan: invalid port range")

	ErrNoTargets     = errors.New("tcpscan: no targets specified")
	ErrInvalidTarget = errors.New("tcpscan: invalid target")

	ErrInvalidOption = errors.New("tcpscan: invalid option")
)
