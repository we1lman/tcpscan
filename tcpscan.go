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

// Scanner checks TCP ports. It is created by New, is immutable afterwards and
// is safe for concurrent use by multiple goroutines.
type Scanner struct {
	cfg config
}

// New returns a Scanner configured by the given options. Options are applied on
// top of the defaults, and the first one that fails aborts construction, in
// which case the returned Scanner is nil. A nil option is ignored.
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

// Scan checks every port of the set on every target and streams the results.
//
// Targets are IPv4 addresses, IPv6 addresses or DNS names; a name is resolved
// to every address it points at and each address is scanned separately.
//
// The input is validated before any work starts, so an invalid target or port
// set is reported by the returned error and no channel is created. Failures
// that happen later, including failures to resolve a name, are reported as
// results on the channel.
//
// The returned channel is closed once the scan is over. The caller must either
// read it until it is closed or cancel the context; abandoning the channel
// without cancelling leaves the workers blocked. Results arrive in no
// particular order.
//
// Scan may be called concurrently and repeatedly on the same Scanner.
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
