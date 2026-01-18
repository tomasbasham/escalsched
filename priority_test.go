package escalsched_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/tomasbasham/escalsched"
)

type stringer struct {
	value string
}

func (s stringer) String() string {
	return s.value
}

func TestParsePriority(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input any
		want  string
	}{
		"parse from string normal": {
			input: "normal",
			want:  "normal",
		},
		"parse from string high": {
			input: "high",
			want:  "high",
		},
		"parse from string very-low": {
			input: "very-low",
			want:  "very-low",
		},
		"parse from string low": {
			input: "low",
			want:  "low",
		},
		"parse from string unknown": {
			input: "unknown",
			want:  "unknown",
		},
		"parse from invalid string": {
			input: "invalid",
			want:  "unknown",
		},
		"parse from stringer": {
			input: stringer{value: "normal"},
			want:  "normal",
		},
		"parse from int": {
			input: 30,
			want:  "normal",
		},
		"parse from int64": {
			input: int64(30),
			want:  "normal",
		},
		"parse from int32": {
			input: int32(30),
			want:  "normal",
		},
		"parse from existing priority": {
			input: escalsched.Priorities.Normal,
			want:  "normal",
		},
		"parse from unsupported type": {
			input: false,
			want:  "unknown",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			priority := escalsched.ParsePriority(tt.input)

			got := priority.String()
			if got != tt.want {
				t.Errorf("mismatch:\n  got:  %q\n  want: %q", got, tt.want)
			}
		})
	}
}

func TestPriorityJSON_Marshal(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		priority escalsched.Priority
		want     string
		wantErr  bool
	}{
		"marshal normal priority": {
			priority: escalsched.Priorities.Normal,
			want:     `"normal"`,
		},
		"marshal high priority": {
			priority: escalsched.Priorities.High,
			want:     `"high"`,
		},
		"marshal unknown priority": {
			priority: escalsched.Priorities.Unknown,
			want:     `"unknown"`,
		},
		"marshal empty priority": {
			priority: escalsched.Priority{},
			want:     `"unknown"`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			b, err := json.Marshal(tt.priority)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error: %v, got: %v", tt.wantErr, err)
			}
			if !tt.wantErr {
				got := string(b)
				if got != tt.want {
					t.Errorf("mismatch:\n  got:  %q\n  want: %q", got, tt.want)
				}
			}
		})
	}
}

func TestPriorityJSON_Unmarshal(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input    string
		priority escalsched.Priority
		wantErr  bool
	}{
		"unmarshal normal priority": {
			input:    `"normal"`,
			priority: escalsched.Priorities.Normal,
		},
		"unmarshal high priority": {
			input:    `"high"`,
			priority: escalsched.Priorities.High,
		},
		"unmarshal unknown priority": {
			input:    `"unknown"`,
			priority: escalsched.Priorities.Unknown,
		},
		"unmarshal non-string priority": {
			input:    `123`,
			priority: escalsched.Priorities.Unknown,
		},
		"unmarshal empty priority": {
			input:    ``,
			priority: escalsched.Priorities.Unknown,
			wantErr:  true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var p escalsched.Priority
			err := json.Unmarshal([]byte(tt.input), &p)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error: %v, got: %v", tt.wantErr, err)
			}
			if !tt.wantErr {
				got, want := p.String(), tt.priority.String()
				if got != want {
					t.Errorf("mismatch:\n  got:  %q\n  want: %q", got, want)
				}
			}
		})
	}
}

func TestPriorityContainer(t *testing.T) {
	t.Parallel()

	priorities := escalsched.Priorities.All()
	expectedLen := 5 // UNKNOWN, VERY_LOW, LOW, NORMAL, HIGH

	if len(priorities) != expectedLen {
		t.Fatalf("expected %d priorities, got: %d", expectedLen, len(priorities))
	}

	got := make([]string, 0, len(priorities))

	// Verify all priorities are unique.
	seen := make(map[string]bool)
	for _, u := range priorities {
		if seen[u.String()] {
			t.Fatalf("duplicate priority found: %q", u.String())
		}
		got = append(got, u.String())
		seen[u.String()] = true
	}

	want := []string{"unknown", "very-low", "low", "normal", "high"}
	if !slices.Equal(got, want) {
		t.Errorf("mismatch:\n  got:  %#v\n  want: %#v", got, want)
	}
}
