package tcpscan

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
)

type testServer struct {
	addr    *net.TCPAddr
	accepts atomic.Int64
}

func expectNoLeaks(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		goleak.VerifyNone(t)
	})
}

func startServer(t *testing.T, address string) *testServer {
	t.Helper()

	ln, err := net.Listen("tcp", address)
	if err != nil {
		t.Skipf("cannot listen on %s: %v", address, err)
	}

	srv := &testServer{addr: ln.Addr().(*net.TCPAddr)}
	done := make(chan struct{})

	go func() {
		defer close(done)

		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			srv.accepts.Add(1)
			_ = conn.Close()
		}
	}()

	t.Cleanup(func() {
		_ = ln.Close()
		<-done
	})

	return srv
}

func freePort(t *testing.T, address string) *net.TCPAddr {
	t.Helper()

	ln, err := net.Listen("tcp", address)
	if err != nil {
		t.Skipf("cannot listen on %s: %v", address, err)
	}

	addr := ln.Addr().(*net.TCPAddr)

	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	return addr
}

func waitForAccepts(t *testing.T, srv *testServer, want int64) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		if srv.accepts.Load() >= want {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Errorf("server accepted %d connections, want %d", srv.accepts.Load(), want)
}

func newRealScanner(t *testing.T, opts ...Option) *Scanner {
	t.Helper()

	s, err := New(opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return s
}

func TestIntegrationOpenPort(t *testing.T) {
	expectNoLeaks(t)

	srv := startServer(t, "127.0.0.1:0")
	s := newRealScanner(t, WithConnectTimeout(2*time.Second))

	ch, err := s.Scan(context.Background(), []string{"127.0.0.1"}, Ports(srv.addr.Port))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	r := got[0]
	if r.State != StateOpen {
		t.Fatalf("State = %v, want %v (Err = %v)", r.State, StateOpen, r.Err)
	}
	if r.Err != nil {
		t.Errorf("Err = %v, want nil", r.Err)
	}
	if r.Port != uint16(srv.addr.Port) {
		t.Errorf("Port = %d, want %d", r.Port, srv.addr.Port)
	}
	if r.Duration <= 0 {
		t.Errorf("Duration = %s, want a positive value", r.Duration)
	}

	waitForAccepts(t, srv, 1)
}

func TestIntegrationClosedPort(t *testing.T) {
	expectNoLeaks(t)

	addr := freePort(t, "127.0.0.1:0")
	s := newRealScanner(t, WithConnectTimeout(2*time.Second))

	ch, err := s.Scan(context.Background(), []string{"127.0.0.1"}, Ports(addr.Port))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	if got[0].State != StateClosed {
		t.Fatalf("State = %v, want %v (Err = %v)", got[0].State, StateClosed, got[0].Err)
	}
	if got[0].Err == nil {
		t.Error("Err = nil, want the original error")
	}
}

func TestIntegrationMixedPorts(t *testing.T) {
	expectNoLeaks(t)

	const servers = 5

	open := make(map[uint16]bool, servers)
	all := make([]int, 0, servers*2)

	for i := 0; i < servers; i++ {
		srv := startServer(t, "127.0.0.1:0")
		open[uint16(srv.addr.Port)] = true
		all = append(all, srv.addr.Port)

		closed := freePort(t, "127.0.0.1:0")
		all = append(all, closed.Port)
	}

	s := newRealScanner(t, WithConcurrency(4), WithConnectTimeout(2*time.Second))

	ch, err := s.Scan(context.Background(), []string{"127.0.0.1"}, Ports(all...))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != len(all) {
		t.Fatalf("got %d results, want %d", len(got), len(all))
	}

	for _, r := range got {
		want := StateClosed
		if open[r.Port] {
			want = StateOpen
		}

		if r.State != want {
			t.Errorf("port %d: State = %v, want %v (Err = %v)", r.Port, r.State, want, r.Err)
		}
	}
}

func TestIntegrationIPv6(t *testing.T) {
	expectNoLeaks(t)

	srv := startServer(t, "[::1]:0")
	s := newRealScanner(t, WithConnectTimeout(2*time.Second))

	ch, err := s.Scan(context.Background(), []string{"::1"}, Ports(srv.addr.Port))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	if got[0].State != StateOpen {
		t.Fatalf("State = %v, want %v (Err = %v)", got[0].State, StateOpen, got[0].Err)
	}
	if got[0].IP.To4() != nil {
		t.Errorf("IP = %v, want an IPv6 address", got[0].IP)
	}
}

func TestIntegrationClosesEveryConnection(t *testing.T) {
	expectNoLeaks(t)

	const servers = 50

	ports := make([]int, 0, servers)
	list := make([]*testServer, 0, servers)

	for i := 0; i < servers; i++ {
		srv := startServer(t, "127.0.0.1:0")
		list = append(list, srv)
		ports = append(ports, srv.addr.Port)
	}

	s := newRealScanner(t, WithConcurrency(16), WithConnectTimeout(2*time.Second))

	ch, err := s.Scan(context.Background(), []string{"127.0.0.1"}, Ports(ports...))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != servers {
		t.Fatalf("got %d results, want %d", len(got), servers)
	}

	for _, r := range got {
		if r.State != StateOpen {
			t.Errorf("port %d: State = %v, want %v (Err = %v)", r.Port, r.State, StateOpen, r.Err)
		}
	}

	for i, srv := range list {
		waitForAccepts(t, srv, 1)

		if n := srv.accepts.Load(); n != 1 {
			t.Errorf("server %d accepted %d connections, want 1", i, n)
		}
	}
}

func TestIntegrationUnroutableAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping a test that waits on a network timeout")
	}

	expectNoLeaks(t)

	s := newRealScanner(t, WithConnectTimeout(300*time.Millisecond))

	ch, err := s.Scan(context.Background(), []string{"192.0.2.1"}, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	switch got[0].State {
	case StateTimeout, StateUnreachable:
	default:
		t.Errorf("State = %v, want %v or %v (Err = %v)",
			got[0].State, StateTimeout, StateUnreachable, got[0].Err)
	}
}
