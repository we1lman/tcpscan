//go:build !unix

package tcpscan

func classifySyscallError(_ error) (State, bool) {
	return StateUnknown, false
}
