package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Kind 表示套装类型。
type Kind string

const (
	KindBundle Kind = "bundle"
	KindCLI    Kind = "cli"
	KindHooks  Kind = "hooks"
)

// Meta 是 manifest 文件的通用元数据（name/version/description），
// 供 pack/search/publish 复用，避免各包重复定义 manifestMeta。
type Meta struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

// ReadMeta 读取并解析 manifest 文件的 name/version 字段。
func ReadMeta(path string) (Meta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Meta{}, fmt.Errorf("读取 manifest 失败: %w", err)
	}
	var m Meta
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Meta{}, fmt.Errorf("解析 manifest 失败: %w", err)
	}
	return m, nil
}

// FileName 返回 Kind 对应的 manifest 文件名。
// 未知 Kind 返回空字符串。
func FileName(k Kind) string {
	switch k {
	case KindBundle:
		return "bundle.yaml"
	case KindCLI:
		return "installer.yaml"
	case KindHooks:
		return "hooks.yaml"
	}
	return ""
}

// KindFromFile 根据 manifest 文件名反查 Kind。不匹配时返回空字符串。
func KindFromFile(name string) (Kind, bool) {
	switch filepath.Base(name) {
	case "bundle.yaml":
		return KindBundle, true
	case "installer.yaml":
		return KindCLI, true
	case "hooks.yaml":
		return KindHooks, true
	}
	return "", false
}

// ManifestFileNames 返回所有支持的 manifest 文件名集合。
func ManifestFileNames() []string {
	return []string{"bundle.yaml", "installer.yaml", "hooks.yaml"}
}

// DetectKind 按优先级查找目录中的 manifest 文件并返回类型。
func DetectKind(dir string) (Kind, error) {
	if fileExists(filepath.Join(dir, "installer.yaml")) {
		return KindCLI, nil
	}
	if fileExists(filepath.Join(dir, "hooks.yaml")) {
		return KindHooks, nil
	}
	if fileExists(filepath.Join(dir, "bundle.yaml")) {
		return KindBundle, nil
	}
	return "", fmt.Errorf("未找到 installer.yaml、hooks.yaml 或 bundle.yaml")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// idPattern 限制资源标识符的字符集：字母、数字、连字符、下划线、点，且以字母或数字开头。
// 这些标识符会进入文件路径（skills/<id>、rules/<id>.md）与生成的 shell 脚本
// （telemetry.sh/wrapper 的引号内），禁止路径分隔符、`..`、空白与控制字符，防路径穿越与命令注入。
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidateID 校验资源标识符（skill/rule/mcp/hooks 的 id、套装 name 等）。
// 返回 nil 表示合法，否则返回描述性错误。
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("标识符不能为空")
	}
	if len(id) > 128 {
		return fmt.Errorf("标识符过长（>128）: %q", id)
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf("标识符 %q 含非法字符，仅允许字母、数字、连字符、下划线、点，且以字母或数字开头", id)
	}
	return nil
}
