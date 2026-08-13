package tcpscan

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestParsePorts(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    []uint16
		wantErr error
	}{
		{"single port", "80", []uint16{80}, nil},
		{"several ports", "443,22,80", []uint16{22, 80, 443}, nil},
		{"range", "20-23", []uint16{20, 21, 22, 23}, nil},
		{"single port range", "80-80", []uint16{80}, nil},
		{"ranges and ports", "20-22,80,8080", []uint16{20, 21, 22, 80, 8080}, nil},
		{"overlapping ranges", "80-85,83-88", []uint16{80, 81, 82, 83, 84, 85, 86, 87, 88}, nil},
		{"duplicates", "80,80,443", []uint16{80, 443}, nil},
		{"spaces around elements", " 80 , 443 ", []uint16{80, 443}, nil},
		{"spaces inside range", "20 - 22", []uint16{20, 21, 22}, nil},
		{"leading zeros", "0080", []uint16{80}, nil},
		{"lowest and highest", "1,65535", []uint16{1, 65535}, nil},

		{"empty string", "", nil, ErrNoPorts},
		{"only spaces", "   ", nil, ErrNoPorts},
		{"only comma", ",", nil, ErrInvalidPort},
		{"empty element", "80,,443", nil, ErrInvalidPort},
		{"trailing comma", "80,", nil, ErrInvalidPort},
		{"zero", "0", nil, ErrInvalidPort},
		{"above maximum", "65536", nil, ErrInvalidPort},
		{"not a number", "http", nil, ErrInvalidPort},
		{"negative", "-5", nil, ErrInvalidPort},
		{"open range", "80-", nil, ErrInvalidPort},
		{"three parts", "1-2-3", nil, ErrInvalidPort},
		{"reversed range", "1000-1", nil, ErrInvalidRange},
		{"huge number", "99999999999999999999", nil, ErrInvalidPort},
		{"plus sign", "+80", nil, ErrInvalidPort},
		{"hex", "0x50", nil, ErrInvalidPort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePorts(tt.spec)

			if tt.wantErr != nil {
				if !errors.Is(got.Err(), tt.wantErr) {
					t.Fatalf("Err() = %v, want %v", got.Err(), tt.wantErr)
				}
				if got.Len() != 0 {
					t.Errorf("Len() = %d, want 0 on error", got.Len())
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

func TestParsePortsErrorMentionsInput(t *testing.T) {
	const spec = "80,http,443"

	err := ParsePorts(spec).Err()
	if err == nil {
		t.Fatal("Err() = nil, want error")
	}

	for _, want := range []string{"http", spec} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message %q does not contain %q", err.Error(), want)
		}
	}
}

func TestParsePortsMatchesBuilders(t *testing.T) {
	parsed := ParsePorts("20-22,80,8080")
	built := Union(Range(20, 22), Ports(80, 8080))

	if err := parsed.Err(); err != nil {
		t.Fatalf("ParsePorts: %v", err)
	}
	if err := built.Err(); err != nil {
		t.Fatalf("Union: %v", err)
	}

	if !slices.Equal(parsed.ports, built.ports) {
		t.Errorf("parsed %v, built %v", parsed.ports, built.ports)
	}
}

func FuzzParsePorts(f *testing.F) {
	seeds := []string{
		"80",
		"22,80,443",
		"1-1024",
		"20-25,80,8000-8100",
		" 80 , 443 ",
		"80-80",
		"",
		"   ",
		",",
		"0",
		"65535",
		"65536",
		"1000-1",
		"-5",
		"80-",
		"-",
		"abc",
		"1-2-3",
		"80,,443",
		"99999999999999999999",
		"0080",
		"+80",
		"\x00",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, spec string) {
		if len(spec) > 128 {
			return
		}

		set := ParsePorts(spec)

		if err := set.Err(); err != nil {
			if set.Len() != 0 {
				t.Errorf("ParsePorts(%q) returned %d ports together with an error %v",
					spec, set.Len(), err)
			}

			return
		}

		if set.Len() == 0 {
			t.Errorf("ParsePorts(%q) returned no ports and no error", spec)
		}

		for i, port := range set.ports {
			if port < minPort {
				t.Errorf("ParsePorts(%q) produced port %d", spec, port)
			}
			if i > 0 && set.ports[i-1] >= port {
				t.Errorf("ParsePorts(%q) produced %v, want sorted values without duplicates",
					spec, set.ports)
			}
		}
	})
}
