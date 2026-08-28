package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/huangchao257/work-cli/internal/configcache"
	"github.com/huangchao257/work-cli/internal/openapi"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/usage"
)

// AuthKind 是首期支持的鉴权类型。
type AuthKind string

const (
	AuthNone   AuthKind = "none"
	AuthBearer AuthKind = "bearer"
	AuthAPIKey AuthKind = "apikey"
)

// SystemConfig 是 ~/.work/api/systems/<name>/system.yaml 的结构。
// 凭据只保存环境变量名，不保存 token 明文。
type SystemConfig struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description,omitempty"`
	BaseURL     string                 `yaml:"base_url,omitempty"`
	SourceURL   string                 `yaml:"source_url,omitempty"`
	SpecFile    string                 `yaml:"spec_file,omitempty"` // openapi.yaml|openapi.json
	Auth        AuthConfig             `yaml:"auth"`
	Shortcuts   map[string]ShortcutDef `yaml:"shortcuts,omitempty"`
}

// AuthConfig 描述鉴权方式。
type AuthConfig struct {
	Kind          AuthKind `yaml:"kind"`                     // none | bearer | apikey
	CredentialEnv string   `yaml:"credential_env,omitempty"` // 凭据环境变量名
	Header        string   `yaml:"header,omitempty"`         // kind=apikey 时；默认 X-API-Key
	Query         string   `yaml:"query,omitempty"`          // kind=apikey 且放 query 时
}

// ShortcutDef 是 system.yaml 中的配置型快捷方式。
type ShortcutDef struct {
	Target      string            `yaml:"target"`
	Description string            `yaml:"description,omitempty"`
	Params      map[string]string `yaml:"params,omitempty"`
	Risk        string            `yaml:"risk,omitempty"`
}

// SystemsDir 返回 ~/.work/api/systems 的绝对路径（不创建）。
func SystemsDir() (string, error) {
	base, err := platform.WorkConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "api", "systems"), nil
}

// systemDir 返回单个系统目录。拒绝穿越目标目录的名字（防 ../ 清理越界）。
func systemDir(name string) (string, error) {
	if err := validateSystemName(name); err != nil {
		return "", err
	}
	root, err := SystemsDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, name)
	// 双保险：Join 清理后必须仍位于 systems 根目录内
	if dir != filepath.Join(root, name) || strings.HasPrefix(filepath.Base(dir), ".") && filepath.Dir(dir) == root {
		return "", usage.Newf("非法系统名 %q", name)
	}
	if filepath.Dir(dir) != root {
		return "", usage.Newf("系统名 %q 会越出系统目录", name)
	}
	return dir, nil
}

func validateSystemName(name string) error {
	if strings.TrimSpace(name) == "" {
		return usage.Newf("系统名不能为空")
	}
	if name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return usage.Newf("系统名 %q 不能以点开头（. / .. 保留）", name)
	}
	if ReservedSystemNames[name] {
		return usage.Newf("系统名 %q 是保留字", name)
	}
	for _, r := range name {
		if !(r == '-' || r == '_' || r == '.' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return usage.Newf("系统名 %q 含非法字符（仅允许字母、数字、-、_、.）", name)
		}
	}
	// Windows 保留设备名（项目明确支持 Windows）
	lower := strings.ToLower(name)
	if windowsReservedNames[lower] {
		return usage.Newf("系统名 %q 是 Windows 保留设备名", name)
	}
	return nil
}

// windowsReservedNames 是 Windows 上不可用作目录名的设备名（不区分大小写）。
var windowsReservedNames = map[string]bool{
	"con": true, "aux": true, "nul": true, "prn": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ListSystemNames 列出已导入系统的目录名（排序后）。
func ListSystemNames() ([]string, error) {
	root, err := SystemsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取系统目录失败: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// LoadSystemConfig 读取指定导入系统的 system.yaml。目录不存在返回 (nil, false, nil)。
func LoadSystemConfig(name string) (*SystemConfig, bool, error) {
	dir, err := systemDir(name)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "system.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("读取 system.yaml 失败: %w", err)
	}
	var cfg SystemConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, false, fmt.Errorf("解析 system.yaml 失败: %w", err)
	}
	if cfg.Name == "" {
		cfg.Name = name
	}
	return &cfg, true, nil
}

// SaveSystemConfig 原子写入 system.yaml（0600）。
func SaveSystemConfig(cfg *SystemConfig) error {
	dir, err := systemDir(cfg.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("创建系统目录失败: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("编码 system.yaml 失败: %w", err)
	}
	return atomicWriteFile(filepath.Join(dir, "system.yaml"), data, 0o600)
}

// atomicWriteFile 临时文件 + rename 原子写（含 Windows 回退）。
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("设置文件权限失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		// Windows: 目标已存在时 Rename 可能失败，删除后重试
		_ = os.Remove(path)
		if err2 := os.Rename(tmpPath, path); err2 != nil {
			return fmt.Errorf("原子替换 %s 失败: %w", filepath.Base(path), err2)
		}
	}
	cleanup = false
	configcache.Invalidate(path)
	return nil
}

// configSystem 将导入的（纯数据）系统包装为 System 接口。
type configSystem struct {
	cfg *SystemConfig
}

// NewConfigSystem 用 SystemConfig 构造导入系统的 System 实现。
func NewConfigSystem(cfg *SystemConfig) System { return &configSystem{cfg: cfg} }

func (c *configSystem) Manifest() Manifest {
	return Manifest{
		Name:        c.cfg.Name,
		Description: c.cfg.Description,
		Source:      "imported",
		SourceURL:   c.cfg.SourceURL,
		BaseURL:     c.effectiveBaseURL(),
	}
}

func (c *configSystem) BaseURL() string { return c.effectiveBaseURL() }

func (c *configSystem) effectiveBaseURL() string {
	if c.cfg.BaseURL != "" {
		return strings.TrimRight(c.cfg.BaseURL, "/")
	}
	if doc, err := c.Document(nil); err == nil && doc != nil {
		return doc.BaseURL()
	}
	return ""
}

// Catalog 读取导入时生成的 catalog.json。
func (c *configSystem) Catalog(ctx context.Context) (*openapi.Catalog, error) {
	dir, err := systemDir(c.cfg.Name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if err != nil {
		return nil, fmt.Errorf("读取 catalog.json 失败: %w", err)
	}
	var catalog openapi.Catalog
	if err := jsonUnmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("解析 catalog.json 失败: %w", err)
	}
	sanitizeCatalog(&catalog)
	catalog.Warnings = append(catalog.Warnings, c.shortcutWarnings()...)
	catalog.Warnings = append(catalog.Warnings, ValidateShortcutTargets(&catalog, c.cfg)...)
	// shortcut 装配期的同名覆盖警告（BuildShortcuts 在命令树装配时跑，这里补收）
	// 空配置直接 Build 一次只为取警告没有意义，仅在存在配置型 shortcut 时才有产出
	if len(c.cfg.Shortcuts) > 0 {
		_, _ = BuildShortcuts(c, c.cfg)
		catalog.Warnings = append(catalog.Warnings, TakeShortcutWarnings()...)
	}
	return &catalog, nil
}

// sanitizeCatalog 校验持久化目录的动态命令数据（catalog.json 可被手编，
// 二次消费不可无条件信任）。非法条目降级为非动态（仍可经 schema/call 使用），
// 只影响该条目而不放任 panic 吞掉整个系统的动态命令；
// 末尾与导入期同语义地消解 cli_path 冲突（手编可造出重复路径）。
func sanitizeCatalog(catalog *openapi.Catalog) {
	for i := range catalog.Operations {
		op := &catalog.Operations[i]
		if !op.Dynamic || len(op.CLIPath) == 0 {
			if op.Dynamic {
				op.Dynamic = false
				message := fmt.Sprintf("operation %s 的 cli_path 缺失或为空，已降级到 schema/call", displayPath(op))
				op.Warnings = append(op.Warnings, message)
				catalog.Warnings = append(catalog.Warnings, message)
			}
			continue
		}
		for _, segment := range op.CLIPath {
			if segment == "" || strings.ContainsAny(segment, " \t/+") || openapi.IsReservedCLIWord(segment) {
				op.Dynamic = false
				message := fmt.Sprintf("operation %s 的 cli_path %q 含非法段或保留字，已降级到 schema/call", displayPath(op), commandPathString(op.CLIPath))
				op.Warnings = append(op.Warnings, message)
				catalog.Warnings = append(catalog.Warnings, message)
				break
			}
		}
		// flag 字段同样不可信：保留字/非法名会在 cobra 装配期 panic
		//（重复注册），吞掉整棵系统的动态命令。逐个降级。
		for j := range op.Parameters {
			p := &op.Parameters[j]
			if p.FlagEnabled && (p.Flag == "" || openapi.IsReservedFlag(p.Flag) || strings.ContainsAny(p.Flag, " \t=")) {
				p.FlagEnabled = false
				message := fmt.Sprintf("operation %s 的参数 flag %q 非法或为保留字，已降级到 --set", displayPath(op), p.Flag)
				op.Warnings = append(op.Warnings, message)
				catalog.Warnings = append(catalog.Warnings, message)
			}
		}
	}
	// cli_path 冲突：与导入期 resolveCommandConflicts 同语义（双双降级 + warning）
	owners := map[string]int{}
	for i := range catalog.Operations {
		op := &catalog.Operations[i]
		if !op.Dynamic {
			continue
		}
		key := strings.Join(op.CLIPath, "\x00")
		if previous, exists := owners[key]; exists {
			other := &catalog.Operations[previous]
			op.Dynamic = false
			other.Dynamic = false
			message := fmt.Sprintf("动态命令路径 %q 冲突，相关 operation 已降级到 schema/call", commandPathString(op.CLIPath))
			op.Warnings = append(op.Warnings, message)
			other.Warnings = append(other.Warnings, message)
			catalog.Warnings = append(catalog.Warnings, message)
			continue
		}
		owners[key] = i
	}
}

func displayPath(op *openapi.CatalogOperation) string {
	if op.ID != "" {
		return op.ID
	}
	return op.Method + " " + op.Path
}

func commandPathString(segments []string) string {
	return strings.Join(segments, " ")
}

// Document 加载导入的原始规范。spec_file 限系统目录内（拒绝手编 system.yaml 的路径穿越）。
func (c *configSystem) Document(ctx context.Context) (*openapi.Document, error) {
	dir, err := systemDir(c.cfg.Name)
	if err != nil {
		return nil, err
	}
	specFile := c.cfg.SpecFile
	if specFile == "" {
		specFile = "openapi.yaml"
	}
	if !safeSpecFileName(specFile) {
		return nil, usage.Newf("system.yaml 的 spec_file %q 非法（只允许当前系统目录内的文件名）", specFile)
	}
	doc, err := openapi.LoadFile(filepath.Join(dir, specFile))
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// safeSpecFileName 校验 spec_file 是纯文件名：无路径分隔符、无 ..，不越出系统目录。
func safeSpecFileName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return false
	}
	return name == filepath.Base(filepath.Clean(name)) && !strings.HasPrefix(name, ".")
}

func (c *configSystem) shortcutWarnings() []string {
	var warnings []string
	for name, def := range c.cfg.Shortcuts {
		if def.Target == "" {
			warnings = append(warnings, fmt.Sprintf("快捷方式 %s 未指定 target", name))
		}
	}
	return warnings
}
