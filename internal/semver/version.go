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

// IsSemantic 判断版本字符串是否为语义化版本（X.Y[.Z][-pre][+build]，至少两段数字）。
// 非语义化（如 git describe --always 无 tag 时产出的短 SHA "1034545"）不应
// 参与版本比较——调用方应视为 dev 构建跳过更新。要求至少两段：单段纯数字
// 无法与短 SHA 区分，且项目发布版本从未使用过单段形式。
func IsSemantic(v string) bool {
	v = Normalize(v)
	if v == "dev" {
		return false
	}
	// 去掉 build 元数据与 prerelease
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return false
	}
	// 核心必须是点分数字段，且至少两段（X.Y）
	segs := strings.Split(v, ".")
	if len(segs) < 2 {
		return false
	}
	for _, seg := range segs {
		if seg == "" {
			return false
		}
		for _, r := range seg {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
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

// comparePrerelease 比较预发布标识（遵循 SemVer 2.0 规范 11.4）。
// 逐点分隔段比较：纯数字段按数值比较，非纯数字段按字典序比较；
// 数字段优先级低于非数字段；无预发布者优先级最高。
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

	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	minLen := len(aParts)
	if len(bParts) < minLen {
		minLen = len(bParts)
	}

	for i := 0; i < minLen; i++ {
		ai, aIsNum := partToNum(aParts[i])
		bi, bIsNum := partToNum(bParts[i])

		if aIsNum && bIsNum {
			if ai < bi {
				return -1
			}
			if ai > bi {
				return 1
			}
		} else if aIsNum {
			// 数字段优先级低于非数字段
			return -1
		} else if bIsNum {
			return 1
		} else {
			if aParts[i] < bParts[i] {
				return -1
			}
			if aParts[i] > bParts[i] {
				return 1
			}
		}
	}

	// 更长的预发布标识优先级更高（段越多表示越远离正式版，例如 alpha < alpha.1）
	if len(aParts) < len(bParts) {
		return -1
	}
	if len(aParts) > len(bParts) {
		return 1
	}
	return 0
}

// partToNum 将预发布段转为数字（若为纯数字），返回值和是否为数字。
func partToNum(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}
