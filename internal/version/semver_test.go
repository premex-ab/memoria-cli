package version

import "testing"

// TestCompareSemver covers the full comparison matrix.
func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.2.0", -1},
		{"0.2.0", "0.1.0", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.9.9", "1.0.0", -1},
		{"0.1.0", "0.1.0", 0},
		{"1.2.3", "1.2.3", 0},
		{"1.2.4", "1.2.3", 1},
		{"1.2.3", "1.2.4", -1},
		{"2.0.0", "1.99.99", 1},
		{"0.0.0", "0.0.1", -1},
		{"99.0.0", "0.1.0", 1},
		// Malformed versions fall back to 0.0.0.
		{"", "0.0.0", 0},
		{"not-a-version", "0.0.0", 0},
	}

	for _, c := range cases {
		got := CompareSemver(c.a, c.b)
		if got != c.want {
			t.Errorf("CompareSemver(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
