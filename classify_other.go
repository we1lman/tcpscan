//go:build !unix && !windows

package tcpscan

func classifySyscallError(_ error) (State, bool) {
	return StateUnknown, false
}
