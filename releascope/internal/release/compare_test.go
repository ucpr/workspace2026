package release

import "testing"

func TestIsNewRelease(t *testing.T) {
	cases := []struct {
		name   string
		stored string
		latest string
		want   bool
	}{
		{"first check, nothing stored", "", "v0.100.0", true},
		{"unchanged", "v0.100.0", "v0.100.0", false},
		{"new patch", "v0.100.0", "v0.100.1", true},
		{"latest missing", "v0.100.0", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNewRelease(tc.stored, tc.latest); got != tc.want {
				t.Errorf("IsNewRelease(%q, %q) = %v, want %v", tc.stored, tc.latest, got, tc.want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.100.0", "v0.99.0", 1},
		{"v0.99.0", "v0.100.0", -1},
		{"v0.100.0", "v0.100.0", 0},
		{"not-a-version", "also-not", 1}, // falls back to strings.Compare ('n' > 'a')
	}
	for _, tc := range cases {
		if got := Compare(tc.a, tc.b); sign(got) != sign(tc.want) {
			t.Errorf("Compare(%q, %q) = %d, want sign %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func sign(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	default:
		return 0
	}
}
