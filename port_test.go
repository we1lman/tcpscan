package tcpscan

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestPorts(t *testing.T) {
	tests := []struct {
		name    string
		input   []int
		want    []uint16
		wantErr error
	}{
		{"single", []int{80}, []uint16{80}, nil},
		{"several", []int{443, 80, 22}, []uint16{22, 80, 443}, nil},
		{"duplicates", []int{80, 80, 443, 80}, []uint16{80, 443}, nil},
		{"lowest", []int{1}, []uint16{1}, nil},
		{"highest", []int{65535}, []uint16{65535}, nil},
		{"empty", nil, nil, ErrNoPorts},
		{"zero", []int{0}, nil, ErrInvalidPort},
		{"negative", []int{-1}, nil, ErrInvalidPort},
		{"too big", []int{65536}, nil, ErrInvalidPort},
		{"one bad among good", []int{80, 70000, 443}, nil, ErrInvalidPort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Ports(tt.input...)

			if tt.wantErr != nil {
				if !errors.Is(got.Err(), tt.wantErr) {
					t.Fatalf("Err() = %v, want %v", got.Err(), tt.wantErr)
				}
				return
			}

			if err := got.Err(); err != nil {
				t.Fatalf("Err() = %v, want nil", err)
			}
			if !slices.Equal(got.ports, tt.want) {
				t.Errorf("ports = %v, want %v", got.ports, tt.want)
			}
		})
	}
}

func TestRange(t *testing.T) {
	tests := []struct {
		name    string
		from    int
		to      int
		want    []uint16
		wantErr error
	}{
		{"small range", 20, 23, []uint16{20, 21, 22, 23}, nil},
		{"single port range", 80, 80, []uint16{80}, nil},
		{"full range length", 1, 65535, nil, nil},
		{"reversed", 1000, 1, nil, ErrInvalidRange},
		{"from is zero", 0, 100, nil, ErrInvalidPort},
		{"to is too big", 1, 65536, nil, ErrInvalidPort},
		{"both invalid", -5, 70000, nil, ErrInvalidPort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Range(tt.from, tt.to)

			if tt.wantErr != nil {
				if !errors.Is(got.Err(), tt.wantErr) {
					t.Fatalf("Err() = %v, want %v", got.Err(), tt.wantErr)
				}
				return
			}

			if err := got.Err(); err != nil {
				t.Fatalf("Err() = %v, want nil", err)
			}
			if tt.want != nil && !slices.Equal(got.ports, tt.want) {
				t.Errorf("ports = %v, want %v", got.ports, tt.want)
			}
			if wantLen := tt.to - tt.from + 1; got.Len() != wantLen {
				t.Errorf("Len() = %d, want %d", got.Len(), wantLen)
			}
		})
	}
}

func TestUnion(t *testing.T) {
	t.Run("merges and sorts", func(t *testing.T) {
		got := Union(
			Ports(8080, 443),
			Range(20, 22),
			Ports(80),
		)

		if err := got.Err(); err != nil {
			t.Fatalf("Err() = %v, want nil", err)
		}

		want := []uint16{20, 21, 22, 80, 443, 8080}
		if !slices.Equal(got.ports, want) {
			t.Errorf("ports = %v, want %v", got.ports, want)
		}
	})

	t.Run("removes overlaps", func(t *testing.T) {
		got := Union(Range(80, 85), Range(83, 88), Ports(84))

		want := []uint16{80, 81, 82, 83, 84, 85, 86, 87, 88}
		if !slices.Equal(got.ports, want) {
			t.Errorf("ports = %v, want %v", got.ports, want)
		}
	})

	t.Run("propagates first error", func(t *testing.T) {
		got := Union(Ports(80), Range(1000, 1), Ports(443))

		if !errors.Is(got.Err(), ErrInvalidRange) {
			t.Errorf("Err() = %v, want %v", got.Err(), ErrInvalidRange)
		}
	})

	t.Run("without arguments", func(t *testing.T) {
		if !errors.Is(Union().Err(), ErrNoPorts) {
			t.Errorf("Err() = %v, want %v", Union().Err(), ErrNoPorts)
		}
	})
}

func TestZeroPortSetIsEmpty(t *testing.T) {
	var s PortSet

	if !errors.Is(s.Err(), ErrNoPorts) {
		t.Errorf("Err() = %v, want %v", s.Err(), ErrNoPorts)
	}
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}
}

func TestErrorMessageContainsValue(t *testing.T) {
	err := Ports(70000).Err()

	if err == nil {
		t.Fatal("Err() = nil, want error")
	}
	if want := "70000"; !strings.Contains(err.Error(), want) {
		t.Errorf("error message %q does not contain %q", err.Error(), want)
	}
}
