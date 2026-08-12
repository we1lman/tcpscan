package tcpscan

import (
	"fmt"
	"time"
)

type Option func(*config) error

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

func WithConnectTimeout(d time.Duration) Option {
	return func(c *config) error {
		if d <= 0 {
			return fmt.Errorf("%w: connect timeout must be positive, got %s", ErrInvalidOption, d)
		}

		c.connectTimeout = d
		return nil
	}
}
