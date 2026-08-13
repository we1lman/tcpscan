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

	for _, h := range hosts {
		trimmed := strings.TrimSpace(h)
		if trimmed == "" {
			return nil, &TargetError{Input: h, Err: ErrInvalidTarget}
		}

		if _, dup := seen[trimmed]; dup {
			continue
		}

		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}

	return out, nil
}

func resolveHost(ctx context.Context, r resolver, host string) ([]target, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []target{{host: host, ip: ip}}, nil
	}

	ips, err := r.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, &TargetError{Input: host, Err: err}
	}

	if len(ips) == 0 {
		return nil, &TargetError{
			Input: host,
			Err:   fmt.Errorf("%w: resolved to no addresses", ErrInvalidTarget),
		}
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
