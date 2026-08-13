package tcpscan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"
)

var errBenchRefused = errors.New("bench: connection refused")

type benchDialer struct{}

func (benchDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, errBenchRefused
}

func BenchmarkScan(b *testing.B) {
	const ports = 1000

	for _, concurrency := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("concurrency=%d", concurrency), func(b *testing.B) {
			s, err := New(WithConcurrency(concurrency))
			if err != nil {
				b.Fatalf("New: %v", err)
			}
			s.cfg.dialer = benchDialer{}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Range(1, ports))
				if err != nil {
					b.Fatalf("Scan: %v", err)
				}

				got := 0
				for range ch {
					got++
				}

				if got != ports {
					b.Fatalf("got %d results, want %d", got, ports)
				}
			}
		})
	}
}

func BenchmarkClassify(b *testing.B) {
	ctx := context.Background()

	cases := []struct {
		name string
		err  error
	}{
		{"nil", nil},
		{"plain", errors.New("something went wrong")},
		{"wrapped errno", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.Errno(10061)}},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = classify(ctx, c.err)
			}
		})
	}
}

func BenchmarkPortSet(b *testing.B) {
	b.Run("full range", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			if err := Range(1, 65535).Err(); err != nil {
				b.Fatalf("Range: %v", err)
			}
		}
	})

	b.Run("union of ranges", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			set := Union(
				Range(1, 1024),
				Range(3000, 4000),
				Ports(8080, 8443, 9090),
			)

			if err := set.Err(); err != nil {
				b.Fatalf("Union: %v", err)
			}
		}
	})
}
