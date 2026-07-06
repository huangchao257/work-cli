package selfupdate

import (
	"strings"

	"github.com/huangchao257/work-cli/internal/semver"
)

// CompareVersions 比较两个语义化版本号。
// 返回值：a < b 为 -1，相等为 0，a > b 为 1。
// 支持 v 前缀；dev 视为最低版本。
// 已委托给 internal/semver.Compare。
func CompareVersions(a, b string) int {
	return semver.Compare(a, b)
}

// normalizeVersion 规范化版本字符串（用于内部，与 semver.Normalize 一致）。
func normalizeVersion(v string) string {
	return semver.Normalize(v)
}

// normalizeTag 规范化 tag 名称：去除首尾空白，确保 v 前缀。
func normalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "dev"
	}
	if !strings.HasPrefix(tag, "v") && tag != "dev" {
		return "v" + tag
	}
	return tag
}
