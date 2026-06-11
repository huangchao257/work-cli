package selfupdate

import (
	"strconv"
	"strings"
)

// CompareVersions 比较两个语义化版本号。
// 返回值：a < b 为 -1，相等为 0，a > b 为 1。
// 支持 v 前缀；dev 视为最低版本。
func CompareVersions(a, b string) int {
	a = normalizeVersion(a)
	b = normalizeVersion(b)
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

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return "dev"
	}
	return v
}

func splitPrerelease(v string) (core, pre string) {
	if i := strings.IndexByte(v, '-'); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

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

func comparePrerelease(a, b string) int {
	// 无预发布版本 > 有预发布版本
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
