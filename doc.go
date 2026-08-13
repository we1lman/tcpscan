// Package tcpscan checks the reachability of TCP ports on one or more hosts.
//
// The package performs a plain TCP connect scan: it asks the operating system
// to establish an ordinary TCP connection and closes it immediately. No raw
// sockets and no elevated privileges are required.
//
// # Getting started
//
// Create a [Scanner] once, then reuse it for as many scans as needed:
//
//	scanner, err := tcpscan.New(
//		tcpscan.WithConcurrency(100),
//		tcpscan.WithConnectTimeout(500*time.Millisecond),
//	)
//	if err != nil {
//		return err
//	}
//
//	results, err := scanner.Scan(ctx, []string{"192.168.1.10"}, tcpscan.Range(1, 1024))
//	if err != nil {
//		return err
//	}
//
//	for r := range results {
//		fmt.Printf("%s:%d %s\n", r.Host, r.Port, r.State)
//	}
//
// A target is an IPv4 address, an IPv6 address or a DNS name; a name is scanned
// at every address it resolves to. Port sets are built with [Ports], [Range]
// and [Union], or parsed from a string with [ParsePorts].
//
// Every checked port produces a [Result] whose [State] is derived from typed
// error values, never from the text of an error.
//
// # Concurrency
//
// A [Scanner] is immutable after construction. It is safe to call
// [Scanner.Scan] from several goroutines at once and to reuse the same scanner
// for any number of scans. The number of connections in flight never exceeds
// the value passed to [WithConcurrency].
//
// Results are produced by a pool of workers, so their order is not defined.
//
// # Cancellation
//
// The caller must either read the result channel until it is closed or cancel
// the context. Abandoning the channel without cancelling the context leaves the
// workers blocked forever. The idiomatic form is:
//
//	ctx, cancel := context.WithCancel(ctx)
//	defer cancel()
//
// Once the context is cancelled the scanner stops handing out new work, lets
// the checks already in flight finish and closes the result channel. Results
// that were in flight at that moment may be delivered with [StateCanceled] if
// the caller is still reading; otherwise they are dropped.
//
// # Resource limits
//
// Each check occupies one file descriptor and one local ephemeral port for the
// duration of the connection. The default concurrency of 100 stays well below
// the limits of a typical system. Raising it above the descriptor limit
// (ulimit -n on unix) produces errors that describe the local machine rather
// than the scanned host.
package tcpscan
