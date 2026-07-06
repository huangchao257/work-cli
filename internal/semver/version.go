// Package semver 提供语义化版本号的比较与规范化工具。
// 供 internal/selfupdate 与其他需要版本比较的包使用。
package semver

import (
	"strconv"
	"strings"
)

// Compare 比较两个语义化版本号。
// 返回值：a < b 为 -1，相等为 0，a > b 为 1。
// 支持 v 前缀；dev 视为最低版本。
func Compare(a, b string) int {
	a = Normalize(a)
	b = Normalize(b)
	if a == "dev" && b == "dev" {
		return 0
	}
	if a == "dev" {
		return -1
	}
	if b == "dev" {
		return 1
	}

	aCore, aPre := splitPrerelease(a)
	bCore, bPre := splitPrerelease(b)

	if cmp := compareCore(aCore, bCore); cmp != 0 {
		return cmp
	}
	return comparePrerelease(aPre, bPre)
}

// Normalize 规范化版本字符串：去除 v 前缀与首尾空白；空字符串返回 "dev"。
func Normalize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return "dev"
	}
	return v
}

// splitPrerelease 将版本号拆分为核心版本号与预发布标识。
func splitPrerelease(v string) (core, pre string) {
	if i := strings.IndexByte(v, '-'); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

// compareCore 按点分数字逐段比较两个核心版本号。
func compareCore(a, b string) int {
	ap := parseParts(a)
	bp := parseParts(b)
	max := len(ap)
	if len(bp) > max {
		max = len(bp)
	}
	for i := 0; i < max; i++ {
		av, bv := 0, 0
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

// parseParts 解析点分隔的数字段，非法值按 0 处理。
func parseParts(v string) []int {
	segments := strings.Split(v, ".")
	out := make([]int, 0, len(segments))
	for _, seg := range segments {
		n, err := strconv.Atoi(seg)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

// comparePrerelease 比较预发布标识（遵循 SemVer 2.0）。
// 无预发布者优先级最高；相同时返回 0；否则按字典序比较。
func comparePrerelease(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}
