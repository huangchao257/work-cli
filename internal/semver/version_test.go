package semver

import "testing"

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		// 基本相等性
		{"v0.1.0", "v0.1.0", 0},
		{"0.1.0", "v0.1.0", 0},
		{"1.2.3", "1.2.3", 0},
		{"v1.0.0", "1.0.0", 0},

		// 主版本比较
		{"v0.1.0", "v0.2.0", -1},
		{"v1.0.0", "v0.9.9", 1},
		{"v2.0.0", "v1.99.99", 1},
		{"v0.9.0", "v1.0.0", -1},

		// 次版本/修订版本比较
		{"v0.1.0", "v0.1.1", -1},
		{"v0.1.1", "v0.1.0", 1},
		{"v0.0.1", "v0.0.2", -1},
		{"v1.2.3", "v1.2.4", -1},
		{"v1.2.3", "v1.3.0", -1},

		// dev 版本
		{"dev", "v0.1.0", -1},
		{"v0.1.0", "dev", 1},
		{"dev", "dev", 0},
		{"", "v0.1.0", -1},
		{"v0.1.0", "", 1},
		{"", "", 0},

		// 预发布版本
		{"v0.1.0-next", "v0.1.0", -1},
		{"v0.1.0", "v0.1.0-next", 1},
		{"v1.0.0-alpha", "v1.0.0-beta", -1},
		{"v1.0.0-beta", "v1.0.0-alpha", 1},
		{"v1.0.0-rc1", "v1.0.0-rc2", -1},
		{"v1.0.0-rc2", "v1.0.0-rc1", 1},
		{"v1.0.0-alpha", "v1.0.0-alpha", 0},

		// 多段版本号
		{"1.2.3.4", "1.2.3.5", -1},
		{"1.2.3.5", "1.2.3.4", 1},
		{"1.2.3.4", "1.2.3.4", 0},
		{"1.2", "1.2.0", 0},
		{"1.2", "1.2.1", -1},

		// 空白/边界
		{"  v1.0.0  ", "v1.0.0", 0},
		{"v0.0.0", "v0.0.0", 0},
		{"0.0.0", "0.0.1", -1},

		// 非数字段值（按 0 处理）
		{"1.x.3", "1.0.3", 0},
		{"1.abc.3", "1.0.3", 0},
	}

	for _, tc := range tests {
		got := Compare(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"v1.0.0", "1.0.0"},
		{"1.0.0", "1.0.0"},
		{"dev", "dev"},
		{"", "dev"},
		{"  v1.2.3  ", "1.2.3"},
		{"V1.0.0", "V1.0.0"}, // 大写 V 不视为前缀
	}
	for _, tc := range tests {
		got := Normalize(tc.input)
		if got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCompareTransitive(t *testing.T) {
	// 兼容性：原 selfupdate.CompareVersions 的所有测试用例确保无回归
	legacyTests := []struct {
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
	for _, tc := range legacyTests {
		got := Compare(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("legacy regression: Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
