package tcpscan

import (
	"net"
	"time"
)

const (
	defaultConcurrency    = 100
	defaultConnectTimeout = 2 * time.Second

	maxConcurrency = 65535
)

type config struct {
	concurrency    int
	connectTimeout time.Duration
	resolver       resolver
}

type Scanner struct {
	cfg config
}

func New(opts ...Option) (*Scanner, error) {
	cfg := config{
		concurrency:    defaultConcurrency,
		connectTimeout: defaultConnectTimeout,
		resolver:       net.DefaultResolver,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	return &Scanner{cfg: cfg}, nil
}
