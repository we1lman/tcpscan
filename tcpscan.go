package tcpscan

import (
	"context"
	"net"
	"time"
)

const (
	defaultConcurrency    = 100
	defaultConnectTimeout = 2 * time.Second

	maxConcurrency = 65535
)

type dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type config struct {
	concurrency    int
	connectTimeout time.Duration
	resolver       resolver
	dialer         dialer
}

type Scanner struct {
	cfg config
}

func New(opts ...Option) (*Scanner, error) {
	cfg := config{
		concurrency:    defaultConcurrency,
		connectTimeout: defaultConnectTimeout,
		resolver:       net.DefaultResolver,
		dialer:         &net.Dialer{},
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

func (s *Scanner) Scan(ctx context.Context, hosts []string, ports PortSet) (<-chan Result, error) {
	if err := ports.Err(); err != nil {
		return nil, err
	}

	normalized, err := normalizeHosts(hosts)
	if err != nil {
		return nil, err
	}

	out := make(chan Result)

	go func() {
		defer close(out)
		s.run(ctx, normalized, ports.ports, out)
	}()

	return out, nil
}
