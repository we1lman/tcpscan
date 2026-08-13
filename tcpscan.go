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

func (s *Scanner) run(ctx context.Context, hosts []string, ports []uint16, out chan<- Result) {
	for _, host := range hosts {
		targets, err := resolveHost(ctx, s.cfg.resolver, host)
		if err != nil {
			out <- Result{Host: host, State: StateError, Err: err}
			continue
		}

		for _, tg := range targets {
			for _, port := range ports {
				out <- s.check(ctx, tg, port)
			}
		}
	}
}

func (s *Scanner) check(ctx context.Context, tg target, port uint16) Result {
	dialCtx, cancel := context.WithTimeout(ctx, s.cfg.connectTimeout)
	defer cancel()

	started := time.Now()
	conn, err := s.cfg.dialer.DialContext(dialCtx, "tcp", tg.address(port))
	elapsed := time.Since(started)

	res := Result{
		Host:     tg.host,
		IP:       tg.ip,
		Port:     port,
		Duration: elapsed,
	}

	if err != nil {
		res.State = StateError
		res.Err = err
		return res
	}

	_ = conn.Close()
	res.State = StateOpen

	return res
}
