package tcpscan

import (
	"context"
	"net"
	"slices"
	"sync"
	"testing"
	"time"
)

type countingDialer struct {
	mu      sync.Mutex
	current int
	peak    int
	total   int

	delay time.Duration
}

func (c *countingDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	c.mu.Lock()
	c.current++
	c.total++
	if c.current > c.peak {
		c.peak = c.current
	}
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.current--
		c.mu.Unlock()
	}()

	if c.delay > 0 {
		select {
		case <-time.After(c.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	client, server := net.Pipe()
	_ = server.Close()

	return client, nil
}

func (c *countingDialer) stats() (peak, total int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.peak, c.total
}

func TestScanRespectsConcurrencyLimit(t *testing.T) {
	const limit = 8

	d := &countingDialer{delay: 2 * time.Millisecond}
	s := newTestScanner(t, d, nil, WithConcurrency(limit))

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Range(1, 400))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 400 {
		t.Fatalf("got %d results, want 400", len(got))
	}

	peak, _ := d.stats()
	if peak > limit {
		t.Errorf("peak concurrency = %d, want at most %d", peak, limit)
	}
	if peak < 2 {
		t.Errorf("peak concurrency = %d, scan looks sequential", peak)
	}
}

func TestScanWithConcurrencyOneKeepsOrder(t *testing.T) {
	d := &fakeDialer{}
	s := newTestScanner(t, d, nil, WithConcurrency(1))

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(443, 22, 80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	collect(t, ch)

	want := []string{"10.0.0.1:22", "10.0.0.1:80", "10.0.0.1:443"}
	if !slices.Equal(d.dialed(), want) {
		t.Errorf("dialed %v, want %v", d.dialed(), want)
	}
}

func TestScanCompletesEveryJob(t *testing.T) {
	d := &countingDialer{}
	s := newTestScanner(t, d, nil, WithConcurrency(64))

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Range(1, 2000))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 2000 {
		t.Fatalf("got %d results, want 2000", len(got))
	}

	seen := make(map[uint16]int, len(got))
	for _, r := range got {
		seen[r.Port]++
	}

	for port := uint16(1); port <= 2000; port++ {
		switch seen[port] {
		case 1:
		case 0:
			t.Fatalf("port %d has no result", port)
		default:
			t.Fatalf("port %d has %d results, want 1", port, seen[port])
		}
	}
}

func TestScanStopsProducingAfterCancel(t *testing.T) {
	const total = 3000

	d := &countingDialer{delay: time.Millisecond}
	s := newTestScanner(t, d, nil, WithConcurrency(4))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := s.Scan(ctx, []string{"10.0.0.1"}, Range(1, total))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	received := 0
	for range ch {
		received++
		if received == 10 {
			cancel()
		}
	}

	_, dialed := d.stats()
	if dialed >= total {
		t.Errorf("dialed %d addresses, want far fewer than %d", dialed, total)
	}
	if received >= total {
		t.Errorf("received %d results, want far fewer than %d", received, total)
	}
}

func TestScanClosesChannelAfterCancel(t *testing.T) {
	d := &countingDialer{delay: time.Millisecond}
	s := newTestScanner(t, d, nil, WithConcurrency(4))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch, err := s.Scan(ctx, []string{"10.0.0.1"}, Range(1, 1000))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("channel was not closed after cancel")
	}
}

func TestScanStopsWhenConsumerAbandonsChannel(t *testing.T) {
	d := &countingDialer{delay: time.Millisecond}
	s := newTestScanner(t, d, nil, WithConcurrency(4))

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := s.Scan(ctx, []string{"10.0.0.1"}, Range(1, 5000))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	for range 5 {
		<-ch
	}

	cancel()
	time.Sleep(300 * time.Millisecond)

	select {
	case _, open := <-ch:
		if open {
			t.Error("received a result, want closed channel: a worker is stuck sending")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("channel is neither closed nor readable: the scan is deadlocked")
	}
}

func TestScanStopsOnCancelWhenResolverFails(t *testing.T) {
	d := &countingDialer{}
	r := &fakeResolver{err: &net.DNSError{Err: "no such host", Name: "nope.invalid"}}
	s := newTestScanner(t, d, r, WithConcurrency(4))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch, err := s.Scan(ctx, []string{"a.invalid", "b.invalid", "c.invalid"}, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	select {
	case _, open := <-ch:
		if open {
			t.Error("received a result, want closed channel: the producer is stuck sending")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("channel is neither closed nor readable: the scan is deadlocked")
	}
}

func TestScanDeliversEveryResultToSlowConsumer(t *testing.T) {
	const ports = 50

	d := &countingDialer{}
	s := newTestScanner(t, d, nil, WithConcurrency(16))

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Range(1, ports))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	received := 0
	for range ch {
		received++
		time.Sleep(time.Millisecond)
	}

	if received != ports {
		t.Errorf("received %d results, want %d", received, ports)
	}

	_, dialed := d.stats()
	if dialed != ports {
		t.Errorf("dialed %d addresses, want %d", dialed, ports)
	}
}

func TestScannerIsReusableConcurrently(t *testing.T) {
	d := &countingDialer{}
	s := newTestScanner(t, d, nil, WithConcurrency(16))

	var wg sync.WaitGroup
	counts := make([]int, 4)

	for i := range counts {
		wg.Add(1)

		go func() {
			defer wg.Done()

			ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Range(1, 250))
			if err != nil {
				t.Errorf("Scan: %v", err)
				return
			}

			for range ch {
				counts[i]++
			}
		}()
	}

	wg.Wait()

	for i, got := range counts {
		if got != 250 {
			t.Errorf("scan %d got %d results, want 250", i, got)
		}
	}

	_, dialed := d.stats()
	if dialed != 1000 {
		t.Errorf("dialed %d addresses, want 1000", dialed)
	}
}

func TestScanSequentialReuse(t *testing.T) {
	d := &countingDialer{}
	s := newTestScanner(t, d, nil, WithConcurrency(8))

	for i := range 3 {
		ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(80, 443))
		if err != nil {
			t.Fatalf("scan %d: %v", i, err)
		}

		if got := len(collect(t, ch)); got != 2 {
			t.Fatalf("scan %d got %d results, want 2", i, got)
		}
	}

	_, dialed := d.stats()
	if dialed != 6 {
		t.Errorf("dialed %d addresses, want 6", dialed)
	}
}
