package tcpscan

import "errors"

var (
	ErrNoPorts      = errors.New("tcpscan: no ports specified")
	ErrInvalidPort  = errors.New("tcpscan: invalid port")
	ErrInvalidRange = errors.New("tcpscan: invalid port range")
)
