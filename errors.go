package tcpscan

import (
	"errors"
	"fmt"
)

// Errors reported by the package. Compare against them with errors.Is.
var (
	// ErrNoPorts means the port set is empty.
	ErrNoPorts = errors.New("tcpscan: no ports specified")

	// ErrInvalidPort means a port lies outside the range 1-65535 or could
	// not be parsed.
	ErrInvalidPort = errors.New("tcpscan: invalid port")

	// ErrInvalidRange means the bounds of a port range are reversed.
	ErrInvalidRange = errors.New("tcpscan: invalid port range")

	// ErrNoTargets means no targets were supplied.
	ErrNoTargets = errors.New("tcpscan: no targets specified")

	// ErrInvalidTarget means a target is empty or cannot be used.
	ErrInvalidTarget = errors.New("tcpscan: invalid target")

	// ErrInvalidOption means an option was given an unusable value.
	ErrInvalidOption = errors.New("tcpscan: invalid option")
)

// TargetError reports which target a failure belongs to. It is returned by
// Scanner.Scan for invalid input and carried in Result.Err for failures that
// happen while a target is being resolved.
type TargetError struct {
	// Input is the target as it was supplied by the caller.
	Input string

	// Err is the underlying cause.
	Err error
}

// Error implements the error interface.
func (e *TargetError) Error() string {
	return fmt.Sprintf("target %q: %v", e.Input, e.Err)
}

// Unwrap returns the underlying cause, so that errors.Is and errors.As reach
// through this error.
func (e *TargetError) Unwrap() error {
	return e.Err
}
