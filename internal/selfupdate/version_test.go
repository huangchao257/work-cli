package selfupdate

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v0.1.0", "v0.1.0", 0},
		{"0.1.0", "v0.1.0", 0},
		{"v0.1.0", "v0.2.0", -1},
		{"v1.0.0", "v0.9.9", 1},
		{"dev", "v0.1.0", -1},
		{"v0.1.0", "dev", 1},
		{"v0.1.0-next", "v0.1.0", -1},
		{"v0.1.0", "v0.1.1", -1},
	}
	for _, tc := range tests {
		got := CompareVersions(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
