package tcpscan

import (
	"fmt"
	"slices"
)

const (
	minPort = 1
	maxPort = 65535
)

type PortSet struct {
	ports []uint16
	err   error
}

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

func (s PortSet) Err() error {
	if s.err != nil {
		return s.err
	}
	if len(s.ports) == 0 {
		return ErrNoPorts
	}
	return nil
}

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
