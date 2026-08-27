package api

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/huangchao257/work-cli/internal/openapi"
	"github.com/huangchao257/work-cli/internal/usage"
)

// ImportOptions 控制 Import 的行为。
type ImportOptions struct {
	Name          string
	Spec          string // 本地文件或 https:// URL
	BaseURL       string
	AuthKind      string
	CredentialEnv string
	AuthHeader    string
	AuthQuery     string
	Overwrite     bool // 重新导入同名系统
	DryRun        bool
}

// ImportResult 是导入结果。
type ImportResult struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Version     string   `json:"version"`
	Operations  int      `json:"operations"`
	DynamicCmds int      `json:"dynamic_commands"`
	Warnings    []string `json:"warnings,omitempty"`
	DryRun      bool     `json:"dry_run,omitempty"`
}

// Import 校验并导入 OpenAPI 规范，生成 system.yaml + 规范快照 + catalog.json。
// 全部校验通过前不做任何删除/写入（--overwrite 也延迟到写入阶段）。
func Import(ctx context.Context, opts ImportOptions) (*ImportResult, error) {
	if err := validateSystemName(opts.Name); err != nil {
		return nil, err
	}
	if !opts.Overwrite && SystemExists(opts.Name) {
		return nil, usage.Newf("系统 %s 已存在，重新导入请加 --overwrite", opts.Name)
	}

	var spec []byte
	var sourceURL string
	specFile := "openapi.yaml"
	switch {
	case strings.HasPrefix(opts.Spec, "https://"):
		_, data, err := openapi.LoadURL(ctx, opts.Spec, nil)
		if err != nil {
			return nil, err
		}
		spec = data
		// JSON/YAML 按内容判断（远程 URL 后缀不可靠）
		if isJSON(spec) {
			specFile = "openapi.json"
		}
		sourceURL = opts.Spec
	default:
		data, err := os.ReadFile(opts.Spec)
		if err != nil {
			return nil, usage.Newf("读取规范文件失败: %v", err)
		}
		if _, err := openapi.LoadBytes(data); err != nil {
			return nil, err
		}
		spec = data
		if strings.HasSuffix(strings.ToLower(opts.Spec), ".json") || isJSON(spec) {
			specFile = "openapi.json"
		}
	}

	doc, err := openapi.LoadBytes(spec)
	if err != nil {
		return nil, err
	}
	catalog, err := doc.Index()
	if err != nil {
		return nil, err
	}
	if opts.BaseURL != "" {
		if err := validateBaseURL(opts.BaseURL); err != nil {
			return nil, err
		}
		catalog.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	}

	dynamic := 0
	for _, op := range catalog.Operations {
		if op.Dynamic {
			dynamic++
		}
	}

	authKind, err := normalizeAuthKind(opts.AuthKind)
	if err != nil {
		return nil, err
	}
	cfg := &SystemConfig{
		Name:        opts.Name,
		Description: doc.Info.Title,
		BaseURL:     catalog.BaseURL,
		SourceURL:   sourceURL,
		SpecFile:    specFile,
		Auth: AuthConfig{
			Kind:          authKind,
			CredentialEnv: opts.CredentialEnv,
			Header:        opts.AuthHeader,
			Query:         opts.AuthQuery,
		},
	}

	result := &ImportResult{
		Name: opts.Name, Title: doc.Info.Title, Version: doc.Info.Version,
		Operations: len(catalog.Operations), DynamicCmds: dynamic,
		Warnings: catalog.Warnings, DryRun: opts.DryRun,
	}
	if opts.DryRun {
		return result, nil
	}
	// 覆盖删除延迟到全部校验通过之后，避免坏规范导致旧系统数据丢失
	if opts.Overwrite {
		if err := RemoveSystem(opts.Name); err != nil {
			return nil, err
		}
	}
	if err := SaveSystemConfig(cfg); err != nil {
		return nil, err
	}
	if err := writeSystemSpec(opts.Name, spec, specFile, catalog); err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeAuthKind(raw string) (AuthKind, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "none":
		return AuthNone, nil
	case "bearer":
		return AuthBearer, nil
	case "apikey":
		return AuthAPIKey, nil
	default:
		return "", usage.Newf("不支持的鉴权类型 %q（支持 none/bearer/apikey）", raw)
	}
}

func isJSON(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	return strings.HasPrefix(trimmed, "{")
}

// validateBaseURL 在导入期校验 --base-url，避免错误延迟到调用期被误判为网络问题。
func validateBaseURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return usage.Newf("--base-url 必须是 http(s)://host[:port] 形式的完整 URL，收到 %q", raw)
	}
	return nil
}

// RefreshOptions 控制 Refresh 的行为。
type RefreshOptions struct {
	Name   string // 空表示刷新全部可刷新系统
	DryRun bool
}

// RefreshResult 是单个系统的刷新结果。
type RefreshResult struct {
	Name       string   `json:"name"`
	Updated    bool     `json:"updated"`
	Reason     string   `json:"reason,omitempty"`
	Operations int      `json:"operations,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// Refresh 重新拉取记录了 HTTPS source_url 的系统并更新规范与 catalog。
func Refresh(ctx context.Context, opts RefreshOptions) ([]RefreshResult, error) {
	names := []string{}
	if opts.Name != "" {
		names = append(names, opts.Name)
	} else {
		all, err := ListSystemNames()
		if err != nil {
			return nil, err
		}
		names = all
	}
	var results []RefreshResult
	for _, name := range names {
		result, err := refreshSystem(ctx, name, opts.DryRun)
		if err != nil {
			return results, err
		}
		results = append(results, *result)
	}
	return results, nil
}

func refreshSystem(ctx context.Context, name string, dryRun bool) (*RefreshResult, error) {
	cfg, exists, err := LoadSystemConfig(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, usage.Newf("系统 %s 不存在", name)
	}
	if !strings.HasPrefix(cfg.SourceURL, "https://") {
		return &RefreshResult{Name: name, Reason: "未记录 HTTPS source_url，跳过"}, nil
	}
	_, data, err := openapi.LoadURL(ctx, cfg.SourceURL, nil)
	if err != nil {
		return nil, err
	}
	doc, err := openapi.LoadBytes(data)
	if err != nil {
		return nil, err
	}
	catalog, err := doc.Index()
	if err != nil {
		return nil, err
	}
	if cfg.BaseURL != "" {
		catalog.BaseURL = cfg.BaseURL
	}
	result := &RefreshResult{Name: name, Updated: true, Operations: len(catalog.Operations), Warnings: catalog.Warnings}
	if dryRun {
		return result, nil
	}
	if err := writeSystemSpec(name, data, cfg.SpecFile, catalog); err != nil {
		return nil, err
	}
	return result, nil
}

// RemoveSystem 删除导入系统目录（幂等）。
func RemoveSystem(name string) error {
	dir, err := systemDir(name)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("删除系统目录失败: %w", err)
	}
	return nil
}

// SystemExists 判断导入系统目录是否存在。
func SystemExists(name string) bool {
	dir, err := systemDir(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "system.yaml"))
	return err == nil
}

// ImportedSystems 返回全部导入系统的 System 实现（容忍单个损坏并收集诊断）。
func ImportedSystems() (systems []System, warnings []string) {
	names, err := ListSystemNames()
	if err != nil {
		return nil, []string{"读取导入系统目录失败: " + err.Error()}
	}
	for _, name := range names {
		cfg, exists, err := LoadSystemConfig(name)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("跳过导入系统 %s: %v", name, err))
			continue
		}
		if !exists {
			warnings = append(warnings, fmt.Sprintf("跳过导入系统 %s: 缺少 system.yaml", name))
			continue
		}
		systems = append(systems, NewConfigSystem(cfg))
	}
	return systems, warnings
}
