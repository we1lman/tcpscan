package tcpscan_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/we1lman/tcpscan"
)

func Example() {
	scanner, err := tcpscan.New(
		tcpscan.WithConcurrency(100),
		tcpscan.WithConnectTimeout(500*time.Millisecond),
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := scanner.Scan(
		ctx,
		[]string{"127.0.0.1", "example.com"},
		tcpscan.Range(1, 1024),
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	for r := range results {
		if r.State != tcpscan.StateOpen {
			continue
		}
		fmt.Printf("%s (%s):%d open in %s\n", r.Host, r.IP, r.Port, r.Duration)
	}
}

func ExampleUnion() {
	ports := tcpscan.Union(
		tcpscan.Range(20, 25),
		tcpscan.Range(80, 90),
		tcpscan.Ports(3306, 5432, 6379, 8080),
	)

	scanner, err := tcpscan.New()
	if err != nil {
		fmt.Println(err)
		return
	}

	results, err := scanner.Scan(context.Background(), []string{"127.0.0.1"}, ports)
	if err != nil {
		fmt.Println(err)
		return
	}

	for r := range results {
		fmt.Printf("%d %s\n", r.Port, r.State)
	}
}

func ExampleScanner_Scan_states() {
	scanner, err := tcpscan.New(tcpscan.WithConnectTimeout(200 * time.Millisecond))
	if err != nil {
		fmt.Println(err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results, err := scanner.Scan(ctx, []string{"192.0.2.1"}, tcpscan.Ports(22, 80, 443))
	if err != nil {
		switch {
		case errors.Is(err, tcpscan.ErrNoTargets):
			fmt.Println("targets are empty")
		case errors.Is(err, tcpscan.ErrNoPorts):
			fmt.Println("ports are empty")
		case errors.Is(err, tcpscan.ErrInvalidPort):
			fmt.Println("bad port:", err)
		case errors.Is(err, tcpscan.ErrInvalidTarget):
			fmt.Println("bad target:", err)
		default:
			fmt.Println(err)
		}
		return
	}

	for r := range results {
		switch r.State {
		case tcpscan.StateOpen:
			fmt.Printf("%d open\n", r.Port)
		case tcpscan.StateClosed:
			fmt.Printf("%d closed\n", r.Port)
		case tcpscan.StateTimeout:
			fmt.Printf("%d timeout\n", r.Port)
		case tcpscan.StateUnreachable:
			fmt.Printf("%d unreachable\n", r.Port)
		case tcpscan.StateCanceled:
			fmt.Printf("%d canceled\n", r.Port)
		case tcpscan.StateError:
			fmt.Printf("%d error: %v\n", r.Port, r.Err)
		case tcpscan.StateUnknown:
			fmt.Printf("%d unknown\n", r.Port)
		}
	}
}

func ExampleScanner_Scan_reuse() {
	scanner, err := tcpscan.New(tcpscan.WithConcurrency(50))
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, host := range []string{"127.0.0.1", "::1"} {
		results, err := scanner.Scan(context.Background(), []string{host}, tcpscan.Range(1, 100))
		if err != nil {
			fmt.Println(err)
			return
		}

		open := 0
		for r := range results {
			if r.State == tcpscan.StateOpen {
				open++
			}
		}
		fmt.Printf("%s: %d open\n", host, open)
	}
}
