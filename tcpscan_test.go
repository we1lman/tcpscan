package tcpscan

import (
	"context"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"
	"time"
)

type fakeDialer struct {
	mu    sync.Mutex
	calls []string

	err   error
	delay time.Duration
}

func (f *fakeDialer) DialContext(ctx context.Context, _, address string) (net.Conn, error) {
	f.mu.Lock()
	f.calls = append(f.calls, address)
	f.mu.Unlock()

	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if f.err != nil {
		return nil, f.err
	}

	client, server := net.Pipe()
	_ = server.Close()

	return client, nil
}

func (f *fakeDialer) dialed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.calls)
}

func newTestScanner(t *testing.T, d dialer, r resolver, opts ...Option) *Scanner {
	t.Helper()

	s, err := New(opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	s.cfg.dialer = d
	if r != nil {
		s.cfg.resolver = r
	}

	return s
}

func collect(t *testing.T, ch <-chan Result) []Result {
	t.Helper()

	var out []Result
	for r := range ch {
		out = append(out, r)
	}

	return out
}

func TestScanRejectsBadPorts(t *testing.T) {
	s := newTestScanner(t, &fakeDialer{}, nil)

	ch, err := s.Scan(context.Background(), []string{"127.0.0.1"}, Range(1000, 1))

	if !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidRange)
	}
	if ch != nil {
		t.Error("channel is not nil, want nil on error")
	}
}

func TestScanRejectsBadHosts(t *testing.T) {
	s := newTestScanner(t, &fakeDialer{}, nil)

	_, err := s.Scan(context.Background(), nil, Ports(80))

	if !errors.Is(err, ErrNoTargets) {
		t.Fatalf("err = %v, want %v", err, ErrNoTargets)
	}
}

func TestScanReturnsResultPerPort(t *testing.T) {
	d := &fakeDialer{}
	s := newTestScanner(t, d, nil)

	ch, err := s.Scan(context.Background(), []string{"127.0.0.1"}, Ports(22, 80, 443))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 3 {
		t.Fatalf("got %d results, want 3", len(got))
	}

	want := []string{"127.0.0.1:22", "127.0.0.1:80", "127.0.0.1:443"}
	if !slices.Equal(d.dialed(), want) {
		t.Errorf("dialed %v, want %v", d.dialed(), want)
	}
}

func TestScanMarksSuccessfulDialAsOpen(t *testing.T) {
	s := newTestScanner(t, &fakeDialer{}, nil)

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(443))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	r := got[0]
	if r.State != StateOpen {
		t.Errorf("State = %v, want %v", r.State, StateOpen)
	}
	if r.Err != nil {
		t.Errorf("Err = %v, want nil", r.Err)
	}
	if r.Host != "10.0.0.1" {
		t.Errorf("Host = %q, want %q", r.Host, "10.0.0.1")
	}
	if !r.IP.Equal(net.ParseIP("10.0.0.1")) {
		t.Errorf("IP = %v, want 10.0.0.1", r.IP)
	}
	if r.Port != 443 {
		t.Errorf("Port = %d, want 443", r.Port)
	}
}

func TestScanMarksFailedDialAsError(t *testing.T) {
	want := errors.New("dial failed")
	s := newTestScanner(t, &fakeDialer{err: want}, nil)

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(443))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	if got[0].State != StateError {
		t.Errorf("State = %v, want %v", got[0].State, StateError)
	}
	if !errors.Is(got[0].Err, want) {
		t.Errorf("Err = %v, want %v", got[0].Err, want)
	}
}

func TestScanExpandsDNSNameToEveryAddress(t *testing.T) {
	d := &fakeDialer{}
	r := &fakeResolver{
		ips: map[string][]net.IP{
			"example.com": {
				net.ParseIP("10.0.0.1"),
				net.ParseIP("10.0.0.2"),
			},
		},
	}
	s := newTestScanner(t, d, r)

	ch, err := s.Scan(context.Background(), []string{"example.com"}, Ports(80, 443))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 4 {
		t.Fatalf("got %d results, want 4", len(got))
	}

	for _, res := range got {
		if res.Host != "example.com" {
			t.Errorf("Host = %q, want %q", res.Host, "example.com")
		}
	}

	want := []string{
		"10.0.0.1:80", "10.0.0.1:443",
		"10.0.0.2:80", "10.0.0.2:443",
	}
	if !slices.Equal(d.dialed(), want) {
		t.Errorf("dialed %v, want %v", d.dialed(), want)
	}
}

func TestScanReportsResolveFailure(t *testing.T) {
	d := &fakeDialer{}
	r := &fakeResolver{err: &net.DNSError{Err: "no such host", Name: "nope.invalid"}}
	s := newTestScanner(t, d, r)

	ch, err := s.Scan(context.Background(), []string{"nope.invalid"}, Ports(80, 443))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
	if got[0].State != StateError {
		t.Errorf("State = %v, want %v", got[0].State, StateError)
	}

	var dnsErr *net.DNSError
	if !errors.As(got[0].Err, &dnsErr) {
		t.Errorf("Err = %v, want *net.DNSError", got[0].Err)
	}
	if len(d.dialed()) != 0 {
		t.Errorf("dialed %v, want nothing", d.dialed())
	}
}

func TestScanUsesBracketsForIPv6(t *testing.T) {
	d := &fakeDialer{}
	s := newTestScanner(t, d, nil)

	ch, err := s.Scan(context.Background(), []string{"2001:db8::1"}, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	collect(t, ch)

	want := []string{"[2001:db8::1]:80"}
	if !slices.Equal(d.dialed(), want) {
		t.Errorf("dialed %v, want %v", d.dialed(), want)
	}
}

func TestScanMeasuresDuration(t *testing.T) {
	const delay = 20 * time.Millisecond

	s := newTestScanner(t, &fakeDialer{delay: delay}, nil)

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	if got[0].Duration < delay/2 {
		t.Errorf("Duration = %s, want at least %s", got[0].Duration, delay/2)
	}
}

func TestScanClosesChannel(t *testing.T) {
	s := newTestScanner(t, &fakeDialer{}, nil)

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	collect(t, ch)

	if _, open := <-ch; open {
		t.Error("channel is still open, want closed")
	}
}

func TestScanDoesNotMutateScanner(t *testing.T) {
	d := &fakeDialer{}
	s := newTestScanner(t, d, nil, WithConcurrency(7), WithConnectTimeout(time.Second))

	before := s.cfg

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	collect(t, ch)

	if s.cfg.concurrency != before.concurrency {
		t.Errorf("concurrency changed to %d, want %d", s.cfg.concurrency, before.concurrency)
	}
	if s.cfg.connectTimeout != before.connectTimeout {
		t.Errorf("connectTimeout changed to %s, want %s", s.cfg.connectTimeout, before.connectTimeout)
	}
}
