package tcpscan

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewWithoutOptions(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	if s.cfg.concurrency != defaultConcurrency {
		t.Errorf("concurrency = %d, want %d", s.cfg.concurrency, defaultConcurrency)
	}
	if s.cfg.connectTimeout != defaultConnectTimeout {
		t.Errorf("connectTimeout = %s, want %s", s.cfg.connectTimeout, defaultConnectTimeout)
	}
	if s.cfg.resolver == nil {
		t.Error("resolver = nil, want default resolver")
	}
}

func TestWithConcurrency(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		want    int
		wantErr bool
	}{
		{"one", 1, 1, false},
		{"typical", 500, 500, false},
		{"upper bound", maxConcurrency, maxConcurrency, false},
		{"zero", 0, 0, true},
		{"negative", -1, 0, true},
		{"above upper bound", maxConcurrency + 1, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(WithConcurrency(tt.input))

			if tt.wantErr {
				if !errors.Is(err, ErrInvalidOption) {
					t.Fatalf("err = %v, want %v", err, ErrInvalidOption)
				}
				if s != nil {
					t.Error("scanner is not nil, want nil on error")
				}
				return
			}

			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if s.cfg.concurrency != tt.want {
				t.Errorf("concurrency = %d, want %d", s.cfg.concurrency, tt.want)
			}
		})
	}
}

func TestWithConnectTimeout(t *testing.T) {
	tests := []struct {
		name    string
		input   time.Duration
		wantErr bool
	}{
		{"millisecond", time.Millisecond, false},
		{"half a second", 500 * time.Millisecond, false},
		{"a minute", time.Minute, false},
		{"zero", 0, true},
		{"negative", -time.Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(WithConnectTimeout(tt.input))

			if tt.wantErr {
				if !errors.Is(err, ErrInvalidOption) {
					t.Fatalf("err = %v, want %v", err, ErrInvalidOption)
				}
				return
			}

			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if s.cfg.connectTimeout != tt.input {
				t.Errorf("connectTimeout = %s, want %s", s.cfg.connectTimeout, tt.input)
			}
		})
	}
}

func TestOptionsApplyInOrder(t *testing.T) {
	s, err := New(
		WithConcurrency(10),
		WithConcurrency(20),
	)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	if s.cfg.concurrency != 20 {
		t.Errorf("concurrency = %d, want 20", s.cfg.concurrency)
	}
}

func TestNewStopsAtFirstBadOption(t *testing.T) {
	applied := false

	spy := func(_ *config) error {
		applied = true
		return nil
	}

	_, err := New(
		WithConcurrency(0),
		spy,
	)

	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidOption)
	}
	if applied {
		t.Error("option after the failing one was applied, want skipped")
	}
}

func TestNewIgnoresNilOption(t *testing.T) {
	s, err := New(nil, WithConcurrency(7), nil)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	if s.cfg.concurrency != 7 {
		t.Errorf("concurrency = %d, want 7", s.cfg.concurrency)
	}
}

func TestScannersDoNotShareConfig(t *testing.T) {
	first, err := New(WithConcurrency(10))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	second, err := New(WithConcurrency(20))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	if first.cfg.concurrency != 10 {
		t.Errorf("first concurrency = %d, want 10", first.cfg.concurrency)
	}
	if second.cfg.concurrency != 20 {
		t.Errorf("second concurrency = %d, want 20", second.cfg.concurrency)
	}
}

func TestOptionErrorMentionsValue(t *testing.T) {
	_, err := New(WithConcurrency(-42))
	if err == nil {
		t.Fatal("err = nil, want error")
	}

	if got := err.Error(); !strings.Contains(got, "-42") {
		t.Errorf("error message %q does not contain %q", got, "-42")
	}
}
