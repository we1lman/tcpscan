package tcpscan

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"
)

type fakeResolver struct {
	ips  map[string][]net.IP
	err  error
	hits int
}

func (f *fakeResolver) LookupIP(_ context.Context, _, host string) ([]net.IP, error) {
	f.hits++
	if f.err != nil {
		return nil, f.err
	}
	return f.ips[host], nil
}

func TestNormalizeHosts(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr error
	}{
		{"single", []string{"127.0.0.1"}, []string{"127.0.0.1"}, nil},
		{"keeps order", []string{"b.com", "a.com"}, []string{"b.com", "a.com"}, nil},
		{"drops duplicates", []string{"a.com", "a.com", "b.com"}, []string{"a.com", "b.com"}, nil},
		{"trims spaces", []string{"  a.com  "}, []string{"a.com"}, nil},
		{"dedup after trim", []string{"a.com", " a.com"}, []string{"a.com"}, nil},
		{"nil slice", nil, nil, ErrNoTargets},
		{"empty slice", []string{}, nil, ErrNoTargets},
		{"empty host", []string{"a.com", ""}, nil, ErrInvalidTarget},
		{"blank host", []string{"   "}, nil, ErrInvalidTarget},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeHosts(tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("hosts = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveHostWithLiteralIP(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{"ipv4", "192.168.1.10"},
		{"ipv6", "2001:db8::1"},
		{"ipv6 loopback", "::1"},
		{"ipv4 loopback", "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeResolver{}

			got, err := resolveHost(context.Background(), r, tt.host)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}

			if len(got) != 1 {
				t.Fatalf("got %d targets, want 1", len(got))
			}
			if got[0].host != tt.host {
				t.Errorf("host = %q, want %q", got[0].host, tt.host)
			}
			if !got[0].ip.Equal(net.ParseIP(tt.host)) {
				t.Errorf("ip = %v, want %v", got[0].ip, tt.host)
			}
			if r.hits != 0 {
				t.Errorf("resolver called %d times, want 0", r.hits)
			}
		})
	}
}

func TestResolveHostWithDNSName(t *testing.T) {
	r := &fakeResolver{
		ips: map[string][]net.IP{
			"example.com": {
				net.ParseIP("93.184.216.34"),
				net.ParseIP("2606:2800:220:1:248:1893:25c8:1946"),
			},
		},
	}

	got, err := resolveHost(context.Background(), r, "example.com")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d targets, want 2", len(got))
	}
	for _, tg := range got {
		if tg.host != "example.com" {
			t.Errorf("host = %q, want %q", tg.host, "example.com")
		}
	}
}

func TestResolveHostDeduplicatesAddresses(t *testing.T) {
	r := &fakeResolver{
		ips: map[string][]net.IP{
			"dup.com": {
				net.ParseIP("10.0.0.1"),
				net.ParseIP("10.0.0.1"),
				net.ParseIP("10.0.0.2"),
			},
		},
	}

	got, err := resolveHost(context.Background(), r, "dup.com")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d targets, want 2", len(got))
	}
}

func TestResolveHostWithoutAddresses(t *testing.T) {
	r := &fakeResolver{ips: map[string][]net.IP{}}

	_, err := resolveHost(context.Background(), r, "empty.com")

	if !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("err = %v, want %v", err, ErrInvalidTarget)
	}
}

func TestResolveHostPropagatesResolverError(t *testing.T) {
	want := &net.DNSError{Err: "no such host", Name: "nope.invalid", IsNotFound: true}
	r := &fakeResolver{err: want}

	_, err := resolveHost(context.Background(), r, "nope.invalid")

	var dnsErr *net.DNSError
	if !errors.As(err, &dnsErr) {
		t.Fatalf("err = %v, want *net.DNSError", err)
	}
	if !dnsErr.IsNotFound {
		t.Error("IsNotFound = false, want true")
	}
}

func TestTargetAddress(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		port uint16
		want string
	}{
		{"ipv4", "192.168.1.10", 443, "192.168.1.10:443"},
		{"ipv6", "2001:db8::1", 80, "[2001:db8::1]:80"},
		{"ipv6 loopback", "::1", 22, "[::1]:22"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tg := target{host: tt.ip, ip: net.ParseIP(tt.ip)}

			if got := tg.address(tt.port); got != tt.want {
				t.Errorf("address() = %q, want %q", got, tt.want)
			}
		})
	}
}
