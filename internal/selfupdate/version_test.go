package selfupdate

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		// 基本相等性
		{"v0.1.0", "v0.1.0", 0},
		{"0.1.0", "v0.1.0", 0},
		{"1.2.3", "1.2.3", 0},

		// 排序
		{"v0.1.0", "v0.2.0", -1},
		{"v1.0.0", "v0.9.9", 1},
		{"v0.1.0", "v0.1.1", -1},

		// dev
		{"dev", "v0.1.0", -1},
		{"v0.1.0", "dev", 1},
		{"dev", "dev", 0},
		{"", "", 0},

		// 预发布
		{"v0.1.0-next", "v0.1.0", -1},
		{"v0.1.0", "v0.1.0-next", 1},
		{"v1.0.0-alpha", "v1.0.0-beta", -1},
		{"v1.0.0-beta.1", "v1.0.0-beta.2", -1},

		// 多段
		{"1.2.3.4", "1.2.3.5", -1},
		{"1.2", "1.2.0", 0},
		{"1.2", "1.2.1", -1},

		// 空白/边界
		{"  v1.0.0  ", "v1.0.0", 0},
		{"v0.0.0", "v1.0.0", -1},
		{"0.0.0", "0.0.1", -1},
	}
	for _, tc := range tests {
		got := CompareVersions(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNormalizeTag(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"v1.0.0", "v1.0.0"},
		{"1.0.0", "v1.0.0"},
		{"dev", "dev"},
		{"", "dev"},
		{"  v2.0.0  ", "v2.0.0"},
		{"v2.0.0-beta", "v2.0.0-beta"},
		{"2.0.0-beta", "v2.0.0-beta"},
	}
	for _, tc := range tests {
		got := normalizeTag(tc.input)
		if got != tc.want {
			t.Errorf("normalizeTag(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"v1.0.0", "1.0.0"},
		{"1.0.0", "1.0.0"},
		{"dev", "dev"},
		{"", "dev"},
		{"  v2.0.0  ", "2.0.0"},
	}
	for _, tc := range tests {
		got := normalizeVersion(tc.input)
		if got != tc.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
