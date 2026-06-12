package source

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/huangchao257/work-cli/internal/catalog"
)

type Kind int

const (
	KindRegistry Kind = iota
	KindGit
	KindLocal
)

type Ref struct {
	Kind   Kind
	Raw    string
	Name   string
	GitURL string
	GitRef string
	Local  string
}

var installNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

// ParseInstallName parses a configured resource name for work install.
// Local paths and git refs are not allowed.
func ParseInstallName(raw string) (Ref, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Ref{}, fmt.Errorf("资源名称不能为空")
	}
	if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "../") {
		return Ref{}, fmt.Errorf("不支持手动指定资源路径，请使用已配置的资源名称，例如: work install dev-kit")
	}
	if strings.HasPrefix(raw, "git:") {
		return Ref{}, fmt.Errorf("不支持 git 引用安装，请使用已配置的资源名称，例如: work install dev-kit")
	}
	if strings.ContainsAny(raw, `/\:`) || strings.Contains(raw, "..") {
		return Ref{}, fmt.Errorf("无效的资源名称 %q，请使用已配置的资源名称", raw)
	}
	if !installNamePattern.MatchString(raw) {
		return Ref{}, fmt.Errorf("无效的资源名称 %q，名称只能包含小写字母、数字和连字符", raw)
	}
	return Ref{Kind: KindRegistry, Raw: raw, Name: raw}, nil
}

// ValidateInstallName checks that the name resolves to a built-in or registry package.
func ValidateInstallName(name string) error {
	if _, ok := catalog.Resolve(name); ok {
		return nil
	}
	cfg, err := LoadUserConfig()
	if err != nil {
		return err
	}
	if cfg != nil && strings.TrimSpace(cfg.Registry.URL) != "" {
		return nil
	}
	names := catalog.Names()
	sort.Strings(names)
	return fmt.Errorf("未知资源 %q，可用内置资源: %s", name, strings.Join(names, ", "))
}

func ParseRef(raw string) (Ref, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Ref{}, fmt.Errorf("安装引用不能为空")
	}
	if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "../") {
		return Ref{Kind: KindLocal, Raw: raw, Local: raw}, nil
	}
	if strings.HasPrefix(raw, "git:") {
		rest := strings.TrimPrefix(raw, "git:")
		at := strings.LastIndex(rest, "@")
		if at <= 0 {
			return Ref{}, fmt.Errorf("git 引用格式应为 git:host/org/repo@ref")
		}
		url := "https://" + rest[:at]
		ref := rest[at+1:]
		return Ref{Kind: KindGit, Raw: raw, GitURL: url, GitRef: ref}, nil
	}
	return Ref{Kind: KindRegistry, Raw: raw, Name: raw}, nil
}
