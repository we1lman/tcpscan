package tcpscan

import (
	"fmt"
	"slices"
)

const (
	minPort = 1
	maxPort = 65535
)

// PortSet is a sorted set of TCP ports without duplicates. Build one with
// Ports, Range, Union or ParsePorts.
//
// Validation is deferred: a constructor never fails on the spot, it stores the
// problem inside the set instead. Check it with Err, or let Scanner.Scan report
// it. The zero value is an empty set and is reported as ErrNoPorts.
//
// A PortSet is immutable and safe to share between concurrent scans.
type PortSet struct {
	ports []uint16
	err   error
}

// Ports returns a set built from the given port numbers. Every number must lie
// in the range 1-65535. Duplicates are removed and the result is sorted.
// Calling Ports without arguments yields ErrNoPorts.
func Ports(ports ...int) PortSet {
	if len(ports) == 0 {
		return PortSet{err: ErrNoPorts}
	}

	out := make([]uint16, 0, len(ports))

	for _, p := range ports {
		if p < minPort || p > maxPort {
			return PortSet{err: fmt.Errorf("%w: %d", ErrInvalidPort, p)}
		}

		out = append(out, uint16(p))
	}

	return PortSet{ports: normalize(out)}
}

// Range returns a set holding every port from "from" to "to" inclusive. Both
// bounds must lie in the range 1-65535, and "from" must not be greater than
// "to"; a reversed range is reported as ErrInvalidRange rather than silently
// swapped.
func Range(from, to int) PortSet {
	if from < minPort || from > maxPort {
		return PortSet{err: fmt.Errorf("%w: %d", ErrInvalidPort, from)}
	}

	if to < minPort || to > maxPort {
		return PortSet{err: fmt.Errorf("%w: %d", ErrInvalidPort, to)}
	}

	if from > to {
		return PortSet{err: fmt.Errorf("%w: %d-%d", ErrInvalidRange, from, to)}
	}

	out := make([]uint16, 0, to-from+1)
	for p := from; p <= to; p++ {
		out = append(out, uint16(p))
	}

	return PortSet{ports: out}
}

// Union merges the given sets into one. Overlapping ports appear once. If any
// input set carries an error, that error is returned in the result. Calling
// Union without arguments yields ErrNoPorts.
func Union(sets ...PortSet) PortSet {
	if len(sets) == 0 {
		return PortSet{err: ErrNoPorts}
	}

	total := 0

	for _, s := range sets {
		if s.err != nil {
			return PortSet{err: s.err}
		}

		total += len(s.ports)
	}

	out := make([]uint16, 0, total)
	for _, s := range sets {
		out = append(out, s.ports...)
	}

	return PortSet{ports: normalize(out)}
}

// Err returns the validation error stored in the set, or ErrNoPorts if the set
// is empty. It returns nil when the set is usable.
func (s PortSet) Err() error {
	if s.err != nil {
		return s.err
	}

	if len(s.ports) == 0 {
		return ErrNoPorts
	}

	return nil
}

// Len returns how many ports the set holds. It is useful for estimating the
// size of a scan before starting it.
func (s PortSet) Len() int {
	return len(s.ports)
}

func normalize(ports []uint16) []uint16 {
	if len(ports) < 2 {
		return ports
	}

	slices.Sort(ports)

	return slices.Compact(ports)
}
