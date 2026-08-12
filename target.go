package tcpscan

import (
	"context"
	"fmt"
	"net"
	"strings"
)

type target struct {
	host string
	ip   net.IP
}

type resolver interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

func normalizeHosts(hosts []string) ([]string, error) {
	if len(hosts) == 0 {
		return nil, ErrNoTargets
	}

	out := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))

	for i, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			return nil, fmt.Errorf("%w: host at index %d is empty", ErrInvalidTarget, i)
		}
		if _, dup := seen[h]; dup {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}

	return out, nil
}

func resolveHost(ctx context.Context, r resolver, host string) ([]target, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []target{{host: host, ip: ip}}, nil
	}

	ips, err := r.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("%w: %s resolved to no addresses", ErrInvalidTarget, host)
	}

	out := make([]target, 0, len(ips))
	seen := make(map[string]struct{}, len(ips))

	for _, ip := range ips {
		key := ip.String()
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, target{host: host, ip: ip})
	}

	return out, nil
}

func (t target) address(port uint16) string {
	return net.JoinHostPort(t.ip.String(), fmt.Sprint(port))
}
