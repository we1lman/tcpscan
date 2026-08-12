package tcpscan

import "testing"

func TestStateString(t *testing.T) {
	tests := []struct {
		name  string
		state State
		want  string
	}{
		{"unknown", StateUnknown, "unknown"},
		{"open", StateOpen, "open"},
		{"closed", StateClosed, "closed"},
		{"timeout", StateTimeout, "timeout"},
		{"unreachable", StateUnreachable, "unreachable"},
		{"canceled", StateCanceled, "canceled"},
		{"error", StateError, "error"},
		{"out of range", State(200), "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.String()
			if got != tt.want {
				t.Errorf("State(%d).String() = %q, want %q", uint8(tt.state), got, tt.want)
			}
		})
	}
}

func TestStateNamesAreUnique(t *testing.T) {
	seen := make(map[string]State, len(stateNames))

	for i, name := range stateNames {
		if name == "" {
			t.Errorf("state %d has no name", i)
			continue
		}
		if prev, ok := seen[name]; ok {
			t.Errorf("states %d and %d share the name %q", prev, i, name)
		}
		seen[name] = State(i)
	}
}

func TestZeroResultIsUnknown(t *testing.T) {
	var r Result

	if r.State != StateUnknown {
		t.Errorf("zero Result.State = %v, want %v", r.State, StateUnknown)
	}
}
