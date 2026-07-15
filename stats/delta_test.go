package stats

import (
	"testing"
	"time"
)

func TestParseDelta(t *testing.T) {
	from := time.Date(2026, time.March, 15, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		delta string
		want  time.Time
	}{
		{"", from},
		{"2y", from.AddDate(-2, 0, 0)},
		{"-2y", from.AddDate(-2, 0, 0)}, // sign ignored, always into the past
		{"3m", from.AddDate(0, -3, 0)},
		{"1w", from.AddDate(0, 0, -7)},
		{"5d", from.AddDate(0, 0, -5)},
	}
	for _, c := range cases {
		got, err := parseDelta(c.delta, from)
		if err != nil {
			t.Errorf("parseDelta(%q) unexpected error: %v", c.delta, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("parseDelta(%q) = %v, want %v", c.delta, got, c.want)
		}
	}
}

func TestParseDeltaErrors(t *testing.T) {
	from := time.Now()
	cases := []struct {
		delta   string
		wantErr string
	}{
		{"xx", "invalid delta value use the format: <int>[y/m/w/d]"},
		{"2x", "invalid delta value use the format: <int>[y/m/w/d]"},
		{"y", "error delta is not a number"},
		{"w", "error delta is not a number"},
	}
	for _, c := range cases {
		_, err := parseDelta(c.delta, from)
		if err == nil {
			t.Errorf("parseDelta(%q) expected error, got nil", c.delta)
			continue
		}
		if err.Error() != c.wantErr {
			t.Errorf("parseDelta(%q) error = %q, want %q", c.delta, err.Error(), c.wantErr)
		}
	}
}
