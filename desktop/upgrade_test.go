package desktop

import "testing"

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"v0.7.1", "0.7.1"},
		{" 0.7.1 ", "0.7.1"},
		{"0.7.1-beta", "0.7.1"},
		{"dev", "dev"},
	}
	for _, tc := range tests {
		if got := normalizeVersion(tc.in); got != tc.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"0.7.1", "0.7.1", 0},
		{"v0.7.1", "0.7.1", 0},
		{"0.7.0", "0.7.1", -1},
		{"0.7.1", "0.7.0", 1},
		{"0.8.0", "0.7.10", 1},
	}
	for _, tc := range tests {
		got := compareSemver(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
