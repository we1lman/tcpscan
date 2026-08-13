package tcpscan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

type fakeNetError struct {
	timeout bool
}

func (e fakeNetError) Error() string   { return "fake net error" }
func (e fakeNetError) Timeout() bool   { return e.timeout }
func (e fakeNetError) Temporary() bool { return false }

func canceledContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	return ctx
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name           string
		parentCanceled bool
		err            error
		want           State
	}{
		{
			name: "no error",
			err:  nil,
			want: StateOpen,
		},
		{
			name:           "open port wins over cancelled context",
			parentCanceled: true,
			err:            nil,
			want:           StateOpen,
		},
		{
			name:           "cancelled parent context",
			parentCanceled: true,
			err:            errors.New("dial failed"),
			want:           StateCanceled,
		},
		{
			name: "wrapped context.Canceled",
			err:  fmt.Errorf("dial tcp: %w", context.Canceled),
			want: StateCanceled,
		},
		{
			name: "context.DeadlineExceeded",
			err:  context.DeadlineExceeded,
			want: StateTimeout,
		},
		{
			name: "wrapped context.DeadlineExceeded",
			err:  fmt.Errorf("dial tcp 10.0.0.1:80: %w", context.DeadlineExceeded),
			want: StateTimeout,
		},
		{
			name: "os.ErrDeadlineExceeded",
			err:  os.ErrDeadlineExceeded,
			want: StateTimeout,
		},
		{
			name: "net.Error that timed out",
			err:  fakeNetError{timeout: true},
			want: StateTimeout,
		},
		{
			name: "net.Error that did not time out",
			err:  fakeNetError{timeout: false},
			want: StateError,
		},
		{
			name: "net.OpError wrapping a timeout",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: fakeNetError{timeout: true},
			},
			want: StateTimeout,
		},
		{
			name: "dns timeout",
			err:  &net.DNSError{Err: "i/o timeout", Name: "slow.invalid", IsTimeout: true},
			want: StateTimeout,
		},
		{
			name: "dns not found",
			err:  &net.DNSError{Err: "no such host", Name: "nope.invalid", IsNotFound: true},
			want: StateError,
		},
		{
			name: "plain error",
			err:  errors.New("something went wrong"),
			want: StateError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.parentCanceled {
				ctx = canceledContext(t)
			}

			if got := classify(ctx, tt.err); got != tt.want {
				t.Errorf("classify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyIgnoresErrorText(t *testing.T) {
	texts := []string{
		"connection refused",
		"i/o timeout",
		"no route to host",
		"context canceled",
	}

	for _, text := range texts {
		t.Run(text, func(t *testing.T) {
			got := classify(context.Background(), errors.New(text))

			if got != StateError {
				t.Errorf("classify() = %v, want %v: the text of an error must not matter", got, StateError)
			}
		})
	}
}

func TestScanReportsTimeout(t *testing.T) {
	d := &fakeDialer{delay: time.Second}
	s := newTestScanner(t, d, nil, WithConnectTimeout(20*time.Millisecond))

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	if got[0].State != StateTimeout {
		t.Errorf("State = %v, want %v (Err = %v)", got[0].State, StateTimeout, got[0].Err)
	}
	if got[0].Err == nil {
		t.Error("Err = nil, want the original error")
	}
}

func TestScanKeepsOpenStateWhenTimeoutIsGenerous(t *testing.T) {
	d := &fakeDialer{delay: 10 * time.Millisecond}
	s := newTestScanner(t, d, nil, WithConnectTimeout(time.Second))

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	if got[0].State != StateOpen {
		t.Errorf("State = %v, want %v", got[0].State, StateOpen)
	}
}

func TestScanReportsDNSFailureAsError(t *testing.T) {
	r := &fakeResolver{err: &net.DNSError{Err: "no such host", Name: "nope.invalid", IsNotFound: true}}
	s := newTestScanner(t, &fakeDialer{}, r)

	ch, err := s.Scan(context.Background(), []string{"nope.invalid"}, Ports(80))
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
}
