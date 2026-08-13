package tcpscan

import (
	"fmt"
	"time"
)

// Option configures a Scanner. Options are applied by New in the order they are
// given, on top of the defaults. The set of options is closed: only this
// package can define them.
type Option func(*config) error

// WithConcurrency limits how many connection attempts may be in flight at once.
// The value must be between 1 and 65535, the number of TCP ports a machine can
// address; the default is 100.
//
// The practical ceiling is lower. Every attempt holds a file descriptor and a
// local ephemeral port, and both are limited by the operating system. Values
// above those limits produce errors that describe the local machine rather than
// the scanned host.
func WithConcurrency(n int) Option {
	return func(c *config) error {
		if n < 1 {
			return fmt.Errorf("%w: concurrency must be positive, got %d", ErrInvalidOption, n)
		}

		if n > maxConcurrency {
			return fmt.Errorf("%w: concurrency must not exceed %d, got %d", ErrInvalidOption, maxConcurrency, n)
		}

		c.concurrency = n

		return nil
	}
}

// WithConnectTimeout sets how long a single connection attempt may take before
// it is reported as StateTimeout. The value must be positive; the default is
// two seconds.
//
// The timeout applies to one port. An overall deadline for the whole scan
// belongs on the context passed to Scanner.Scan.
func WithConnectTimeout(d time.Duration) Option {
	return func(c *config) error {
		if d <= 0 {
			return fmt.Errorf("%w: connect timeout must be positive, got %s", ErrInvalidOption, d)
		}

		c.connectTimeout = d

		return nil
	}
}
