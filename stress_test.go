package tcpscan

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestStressConcurrentScannersAndScans(t *testing.T) {
	defer goleak.VerifyNone(t)

	const (
		scanners = 4
		scans    = 8
		ports    = 100
	)

	var wg sync.WaitGroup

	for i := 0; i < scanners; i++ {
		s := newTestScanner(t, &countingDialer{}, nil, WithConcurrency(16))

		for j := 0; j < scans; j++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Range(1, ports))
				if err != nil {
					t.Errorf("Scan: %v", err)
					return
				}

				got := 0
				for range ch {
					got++
				}

				if got != ports {
					t.Errorf("got %d results, want %d", got, ports)
				}
			}()
		}
	}

	wg.Wait()
}

func TestStressSharedPortSet(t *testing.T) {
	defer goleak.VerifyNone(t)

	ports := Union(Range(1, 200), Ports(8080, 9090))
	before := slices.Clone(ports.ports)

	s := newTestScanner(t, &countingDialer{}, nil, WithConcurrency(32))

	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, ports)
			if err != nil {
				t.Errorf("Scan: %v", err)
				return
			}

			got := 0
			for range ch {
				got++
			}

			if got != len(before) {
				t.Errorf("got %d results, want %d", got, len(before))
			}
		}()
	}

	wg.Wait()

	if !slices.Equal(ports.ports, before) {
		t.Errorf("PortSet changed to %v, want %v", ports.ports, before)
	}
}

func TestScanDoesNotModifyInputHosts(t *testing.T) {
	defer goleak.VerifyNone(t)

	hosts := []string{"  10.0.0.2  ", "10.0.0.1", "10.0.0.1"}
	before := slices.Clone(hosts)

	s := newTestScanner(t, &countingDialer{}, nil, WithConcurrency(4))

	ch, err := s.Scan(context.Background(), hosts, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}

	if !slices.Equal(hosts, before) {
		t.Errorf("input slice changed to %v, want %v", hosts, before)
	}
}

func TestStressCancelAtDifferentMoments(t *testing.T) {
	defer goleak.VerifyNone(t)

	for _, after := range []int{0, 1, 5, 50, 500} {
		t.Run(fmt.Sprintf("cancel after %d results", after), func(t *testing.T) {
			d := &countingDialer{delay: 100 * time.Microsecond}
			s := newTestScanner(t, d, nil, WithConcurrency(8))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch, err := s.Scan(ctx, []string{"10.0.0.1"}, Range(1, 2000))
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}

			if after == 0 {
				cancel()
			}

			received := 0

			for r := range ch {
				received++

				if r.State == StateUnknown {
					t.Errorf("port %d has no state", r.Port)
				}
				if r.Port == 0 {
					t.Error("result has no port")
				}
				if r.Host == "" {
					t.Error("result has no host")
				}

				if received == after {
					cancel()
				}
			}

			if received > 2000 {
				t.Errorf("received %d results, want at most 2000", received)
			}
		})
	}
}

func TestStressRepeatedScansKeepScannerIntact(t *testing.T) {
	defer goleak.VerifyNone(t)

	d := &countingDialer{}
	s := newTestScanner(t, d, nil, WithConcurrency(8), WithConnectTimeout(time.Second))

	before := s.cfg

	for i := 0; i < 20; i++ {
		ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(80, 443))
		if err != nil {
			t.Fatalf("scan %d: %v", i, err)
		}

		if got := len(collect(t, ch)); got != 2 {
			t.Fatalf("scan %d got %d results, want 2", i, got)
		}
	}

	if s.cfg.concurrency != before.concurrency {
		t.Errorf("concurrency = %d, want %d", s.cfg.concurrency, before.concurrency)
	}
	if s.cfg.connectTimeout != before.connectTimeout {
		t.Errorf("connectTimeout = %s, want %s", s.cfg.connectTimeout, before.connectTimeout)
	}

	if _, dialed := d.stats(); dialed != 40 {
		t.Errorf("dialed %d addresses, want 40", dialed)
	}
}
