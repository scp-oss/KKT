package notify

import (
	"testing"
	"time"
)

func TestParseHHMM(t *testing.T) {
	cases := []struct {
		in     string
		wantHH int
		wantMM int
		wantOK bool
	}{
		{"09:00", 9, 0, true},
		{"23:59", 23, 59, true},
		{"9:5", 9, 5, true},
		{"00:00", 0, 0, true},
		{"24:00", 0, 0, false},
		{"12:60", 0, 0, false},
		{"-1:00", 0, 0, false},
		{"abc", 0, 0, false},
		{"", 0, 0, false},
		{"12", 0, 0, false},
	}
	for _, c := range cases {
		hh, mm, ok := parseHHMM(c.in)
		if ok != c.wantOK {
			t.Errorf("parseHHMM(%q) ok = %v, want %v", c.in, ok, c.wantOK)
			continue
		}
		if ok && (hh != c.wantHH || mm != c.wantMM) {
			t.Errorf("parseHHMM(%q) = %d:%d, want %d:%d", c.in, hh, mm, c.wantHH, c.wantMM)
		}
	}
}

func TestNextOccurrence(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 9, 11, 14, 30, 0, 0, loc)

	// Time later today -> fires today.
	got := nextOccurrence(now, "18:00")
	want := time.Date(2026, 9, 11, 18, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("nextOccurrence(later today) = %v, want %v", got, want)
	}

	// Time already passed today -> fires tomorrow.
	got = nextOccurrence(now, "09:00")
	want = time.Date(2026, 9, 12, 9, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("nextOccurrence(earlier today) = %v, want %v", got, want)
	}

	// Exactly now -> not after now, so rolls to tomorrow.
	got = nextOccurrence(now, "14:30")
	want = time.Date(2026, 9, 12, 14, 30, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("nextOccurrence(exactly now) = %v, want %v", got, want)
	}

	// Malformed schedule -> zero value.
	if got := nextOccurrence(now, "not-a-time"); !got.IsZero() {
		t.Errorf("nextOccurrence(malformed) = %v, want zero", got)
	}
}
