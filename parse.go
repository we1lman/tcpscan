package tcpscan

import (
	"fmt"
	"strconv"
	"strings"
)

// ParsePorts builds a port set from a textual specification, which is handy for
// command line flags and configuration files.
//
// The specification is a comma separated list of single ports and ranges:
//
//	80
//	22,80,443
//	1-1024
//	20-25,80,8000-8100
//
// Space around elements and around the dash of a range is ignored. Every number
// must consist of decimal digits only and lie in the range 1-65535. Overlapping
// ranges and repeated ports are merged.
//
// Like the other constructors, ParsePorts defers its error to PortSet.Err.
func ParsePorts(spec string) PortSet {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return PortSet{err: ErrNoPorts}
	}

	parts := strings.Split(trimmed, ",")
	sets := make([]PortSet, 0, len(parts))

	for _, part := range parts {
		set := parsePortsPart(part)

		if err := set.Err(); err != nil {
			return PortSet{err: fmt.Errorf("%w in %q", err, spec)}
		}

		sets = append(sets, set)
	}

	return Union(sets...)
}

func parsePortsPart(part string) PortSet {
	trimmed := strings.TrimSpace(part)
	if trimmed == "" {
		return PortSet{err: fmt.Errorf("%w: empty element", ErrInvalidPort)}
	}

	from, to, isRange := strings.Cut(trimmed, "-")
	if !isRange {
		port, err := parsePortNumber(trimmed)
		if err != nil {
			return PortSet{err: err}
		}

		return Ports(port)
	}

	low, err := parsePortNumber(strings.TrimSpace(from))
	if err != nil {
		return PortSet{err: err}
	}

	high, err := parsePortNumber(strings.TrimSpace(to))
	if err != nil {
		return PortSet{err: err}
	}

	return Range(low, high)
}

func parsePortNumber(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("%w: missing port number", ErrInvalidPort)
	}

	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("%w: %q is not a number", ErrInvalidPort, s)
		}
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %q is out of range", ErrInvalidPort, s)
	}

	return n, nil
}
