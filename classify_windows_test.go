//go:build windows

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
		Err:  os.NewSyscallError("connectex", errno),
	}
}

func TestClassifySyscallError(t *testing.T) {
	tests := []struct {
		name  string
		errno syscall.Errno
		want  State
		known bool
	}{
		{"connection refused", wsaeConnectionRefused, StateClosed, true},
		{"connection reset", wsaeConnectionReset, StateClosed, true},
		{"host unreachable", wsaeHostUnreachable, StateUnreachable, true},
		{"network unreachable", wsaeNetworkUnreach, StateUnreachable, true},
		{"host down", wsaeHostDown, StateUnreachable, true},
		{"network down", wsaeNetworkDown, StateUnreachable, true},
		{"network reset", wsaeNetworkReset, StateUnreachable, true},
		{"access denied", wsaeAccessDenied, StateUnreachable, true},
		{"timed out", wsaeTimedOut, StateTimeout, true},
		{"address not available", syscall.Errno(10049), StateUnknown, false},
		{"too many open sockets", syscall.Errno(10024), StateUnknown, false},
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

func TestWinsockCodesMatchMicrosoftValues(t *testing.T) {
	tests := []struct {
		name  string
		errno syscall.Errno
		want  uintptr
	}{
		{"WSAEACCES", wsaeAccessDenied, 10013},
		{"WSAENETDOWN", wsaeNetworkDown, 10050},
		{"WSAENETUNREACH", wsaeNetworkUnreach, 10051},
		{"WSAENETRESET", wsaeNetworkReset, 10052},
		{"WSAECONNRESET", wsaeConnectionReset, 10054},
		{"WSAETIMEDOUT", wsaeTimedOut, 10060},
		{"WSAECONNREFUSED", wsaeConnectionRefused, 10061},
		{"WSAEHOSTDOWN", wsaeHostDown, 10064},
		{"WSAEHOSTUNREACH", wsaeHostUnreachable, 10065},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if uintptr(tt.errno) != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, uintptr(tt.errno), tt.want)
			}
		})
	}
}

func TestClassifySyscallErrorAcceptsBareErrno(t *testing.T) {
	got, ok := classifySyscallError(wsaeConnectionRefused)

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
		{"refused means closed", wsaeConnectionRefused, StateClosed},
		{"host unreachable", wsaeHostUnreachable, StateUnreachable},
		{"network unreachable", wsaeNetworkUnreach, StateUnreachable},
		{"winsock timeout", wsaeTimedOut, StateTimeout},
		{"local ports exhausted", syscall.Errno(10049), StateError},
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
	got := classify(canceledContext(t), dialError(wsaeConnectionRefused))

	if got != StateCanceled {
		t.Errorf("classify() = %v, want %v", got, StateCanceled)
	}
}

func TestScanReportsClosedPort(t *testing.T) {
	d := &fakeDialer{err: dialError(wsaeConnectionRefused)}
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
	d := &fakeDialer{err: dialError(wsaeHostUnreachable)}
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
