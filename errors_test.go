package tcpscan

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

func TestTargetErrorUnwrapsSentinel(t *testing.T) {
	err := error(&TargetError{Input: "bad host", Err: ErrInvalidTarget})

	if !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("errors.Is(err, ErrInvalidTarget) = false, want true")
	}
}

func TestTargetErrorUnwrapsDNSError(t *testing.T) {
	dns := &net.DNSError{Err: "no such host", Name: "nope.invalid", IsNotFound: true}
	err := error(&TargetError{Input: "nope.invalid", Err: dns})

	var got *net.DNSError
	if !errors.As(err, &got) {
		t.Fatal("errors.As(err, *net.DNSError) = false, want true")
	}
	if !got.IsNotFound {
		t.Error("IsNotFound = false, want true")
	}
}

func TestTargetErrorMessage(t *testing.T) {
	err := &TargetError{Input: "  ", Err: ErrInvalidTarget}

	got := err.Error()

	for _, want := range []string{`"  "`, "invalid target"} {
		if !strings.Contains(got, want) {
			t.Errorf("message %q does not contain %q", got, want)
		}
	}
}

func TestScanReturnsTargetErrorForEmptyHost(t *testing.T) {
	s := newTestScanner(t, &fakeDialer{}, nil)

	_, err := s.Scan(context.Background(), []string{"10.0.0.1", "   "}, Ports(80))
	if err == nil {
		t.Fatal("err = nil, want error")
	}

	if !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("errors.Is(err, ErrInvalidTarget) = false, want true")
	}

	var targetErr *TargetError
	if !errors.As(err, &targetErr) {
		t.Fatal("errors.As(err, *TargetError) = false, want true")
	}
	if targetErr.Input != "   " {
		t.Errorf("Input = %q, want %q", targetErr.Input, "   ")
	}
}

func TestResultCarriesTargetError(t *testing.T) {
	r := &fakeResolver{err: &net.DNSError{Err: "no such host", Name: "nope.invalid"}}
	s := newTestScanner(t, &fakeDialer{}, r)

	ch, err := s.Scan(context.Background(), []string{"nope.invalid"}, Ports(80))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := collect(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}

	var targetErr *TargetError
	if !errors.As(got[0].Err, &targetErr) {
		t.Fatalf("Err = %v, want *TargetError", got[0].Err)
	}
	if targetErr.Input != "nope.invalid" {
		t.Errorf("Input = %q, want %q", targetErr.Input, "nope.invalid")
	}

	var dnsErr *net.DNSError
	if !errors.As(got[0].Err, &dnsErr) {
		t.Error("the original DNS error is not reachable through the chain")
	}
}

func TestResolveHostWrapsMissingAddresses(t *testing.T) {
	r := &fakeResolver{ips: map[string][]net.IP{}}

	_, err := resolveHost(context.Background(), r, "empty.invalid")

	if !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("errors.Is(err, ErrInvalidTarget) = false, want true")
	}

	var targetErr *TargetError
	if !errors.As(err, &targetErr) {
		t.Fatal("errors.As(err, *TargetError) = false, want true")
	}
	if targetErr.Input != "empty.invalid" {
		t.Errorf("Input = %q, want %q", targetErr.Input, "empty.invalid")
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	all := []error{
		ErrNoPorts,
		ErrInvalidPort,
		ErrInvalidRange,
		ErrNoTargets,
		ErrInvalidTarget,
		ErrInvalidOption,
	}

	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("errors.Is(%v, %v) = true, want false", a, b)
			}
		}
	}
}
