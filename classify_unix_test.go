//go:build unix

package tcpscan

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
)

func dialError(errno syscall.Errno) error {
	return &net.OpError{
		Op:   "dial",
		Net:  "tcp",
		Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 80},
		Err:  os.NewSyscallError("connect", errno),
	}
}

func TestClassifySyscallError(t *testing.T) {
	tests := []struct {
		name  string
		errno syscall.Errno
		want  State
		known bool
	}{
		{"connection refused", syscall.ECONNREFUSED, StateClosed, true},
		{"connection reset", syscall.ECONNRESET, StateClosed, true},
		{"host unreachable", syscall.EHOSTUNREACH, StateUnreachable, true},
		{"network unreachable", syscall.ENETUNREACH, StateUnreachable, true},
		{"host down", syscall.EHOSTDOWN, StateUnreachable, true},
		{"network down", syscall.ENETDOWN, StateUnreachable, true},
		{"permission denied", syscall.EACCES, StateUnreachable, true},
		{"operation not permitted", syscall.EPERM, StateUnreachable, true},
		{"address not available", syscall.EADDRNOTAVAIL, StateUnknown, false},
		{"too many open files", syscall.EMFILE, StateUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := classifySyscallError(dialError(tt.errno))

			if ok != tt.known {
				t.Fatalf("recognised = %v, want %v", ok, tt.known)
			}
			if got != tt.want {
				t.Errorf("state = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifySyscallErrorAcceptsBareErrno(t *testing.T) {
	got, ok := classifySyscallError(syscall.ECONNREFUSED)

	if !ok {
		t.Fatal("recognised = false, want true")
	}
	if got != StateClosed {
		t.Errorf("state = %v, want %v", got, StateClosed)
	}
}

func TestClassifyWithSyscallErrors(t *testing.T) {
	tests := []struct {
		name  string
		errno syscall.Errno
		want  State
	}{
		{"refused means closed", syscall.ECONNREFUSED, StateClosed},
		{"host unreachable", syscall.EHOSTUNREACH, StateUnreachable},
		{"network unreachable", syscall.ENETUNREACH, StateUnreachable},
		{"kernel timeout", syscall.ETIMEDOUT, StateTimeout},
		{"local ports exhausted", syscall.EADDRNOTAVAIL, StateError},
		{"descriptor limit", syscall.EMFILE, StateError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classify(context.Background(), dialError(tt.errno))

			if got != tt.want {
				t.Errorf("classify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyPrefersCancelOverSyscallError(t *testing.T) {
	got := classify(canceledContext(t), dialError(syscall.ECONNREFUSED))

	if got != StateCanceled {
		t.Errorf("classify() = %v, want %v", got, StateCanceled)
	}
}

func TestScanReportsClosedPort(t *testing.T) {
	d := &fakeDialer{err: dialError(syscall.ECONNREFUSED)}
	s := newTestScanner(t, d, nil)

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	if got[0].State != StateClosed {
		t.Errorf("State = %v, want %v", got[0].State, StateClosed)
	}

	var errno syscall.Errno
	if !errors.As(got[0].Err, &errno) {
		t.Error("Err does not carry the original errno")
	}
}

func TestScanReportsUnreachableHost(t *testing.T) {
	d := &fakeDialer{err: dialError(syscall.EHOSTUNREACH)}
	s := newTestScanner(t, d, nil)

	ch, err := s.Scan(context.Background(), []string{"10.0.0.1"}, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	if got[0].State != StateUnreachable {
		t.Errorf("State = %v, want %v", got[0].State, StateUnreachable)
	}
}
