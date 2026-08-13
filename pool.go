package tcpscan

import (
	"context"
	"sync"
	"time"
)

type job struct {
	target target
	port   uint16
}

func (s *Scanner) run(ctx context.Context, hosts []string, ports []uint16, out chan<- Result) {
	jobs := make(chan job)

	var wg sync.WaitGroup
	wg.Add(s.cfg.concurrency)

	for range s.cfg.concurrency {
		go func() {
			defer wg.Done()
			s.worker(ctx, jobs, out)
		}()
	}

	s.produce(ctx, hosts, ports, jobs, out)
	close(jobs)

	wg.Wait()
}

func (s *Scanner) produce(ctx context.Context, hosts []string, ports []uint16, jobs chan<- job, out chan<- Result) {
	for _, host := range hosts {
		if ctx.Err() != nil {
			return
		}

		targets, err := resolveHost(ctx, s.cfg.resolver, host)
		if err != nil {
			out <- Result{Host: host, State: StateError, Err: err}
			continue
		}

		for _, tg := range targets {
			for _, port := range ports {
				select {
				case jobs <- job{target: tg, port: port}:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (s *Scanner) worker(ctx context.Context, jobs <-chan job, out chan<- Result) {
	for j := range jobs {
		out <- s.check(ctx, j.target, j.port)
	}
}

func (s *Scanner) check(ctx context.Context, tg target, port uint16) Result {
	dialCtx, cancel := context.WithTimeout(ctx, s.cfg.connectTimeout)
	defer cancel()

	started := time.Now()
	conn, err := s.cfg.dialer.DialContext(dialCtx, "tcp", tg.address(port))
	elapsed := time.Since(started)

	res := Result{
		Host:     tg.host,
		IP:       tg.ip,
		Port:     port,
		Duration: elapsed,
	}

	if err != nil {
		res.State = StateError
		res.Err = err
		return res
	}

	_ = conn.Close()
	res.State = StateOpen

	return res
}
