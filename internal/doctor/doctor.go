// Package doctor 实现 work doctor 诊断命令的核心检查逻辑。
//
// 该包将所有检查拆分为可被测试覆盖的小函数；Run 负责编排，
// 纯逻辑分支（如 YAML/JSON 解析判定）独立暴露以便单元测试，
// 无需依赖真实 HOME 目录。
package doctor

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/huangchao257/work-cli/internal/adapter"
	"github.com/huangchao257/work-cli/internal/ai"
	"github.com/huangchao257/work-cli/internal/hooks"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/selfupdate"
	"github.com/huangchao257/work-cli/internal/source"
	"github.com/huangchao257/work-cli/internal/state"
	"gopkg.in/yaml.v3"
)

// CheckResult 表示单项检查结果。
// Severity 取值 "error"（失败会影响退出码）或 "info"（仅展示，不计失败）。
type CheckResult struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Detail   string `json:"detail"`
	Severity string `json:"severity"`
}

// Options 为 Run 的入参。
type Options struct {
	Scope string   // user 或 project
	IDEs  []string // --ide 显式指定的 IDE 名称列表
}

// Severity 常量。
const (
	SeverityError = "error"
	SeverityInfo  = "info"
)

// Run 执行全部诊断检查，返回逐项结果。Run 自身只编排，
// 不返回错误（单项失败体现在 CheckResult 中），便于调用方汇总。
func Run(opts Options) []CheckResult {
	scope := opts.Scope
	if scope == "" {
		scope = "user"
	}
	var results []CheckResult

	results = append(results, checkIDEs(opts.IDEs))
	results = append(results, checkWorkInPath())
	results = append(results, checkConfigYAML())
	results = append(results, checkInstalledJSON(scope))
	results = append(results, checkMCPConfigs(scope)...)
	results = append(results, checkCodegraph())
	results = append(results, checkJQ())
	results = append(results, checkSelfUpdate())
	results = append(results, checkAIConfig())
	results = append(results, checkRegistryConnectivity())
	results = append(results, checkHooksConfig())
	results = append(results, checkStaleTempFiles())
	results = append(results, checkFilePermissions())

	return results
}

// HasError 判断结果集中是否存在 severity=error 且未通过的项。
func HasError(results []CheckResult) bool {
	for _, r := range results {
		if r.Severity == SeverityError && !r.OK {
			return true
		}
	}
	return false
}

// Summary 统计通过/失败数（info 项未通过不计为失败）。
func Summary(results []CheckResult) (passed, failed int) {
	for _, r := range results {
		if r.Severity == SeverityError && !r.OK {
			failed++
		} else {
			passed++
		}
	}
	return passed, failed
}

// checkIDEs 遍历 adapter.All() 调 Detect()，记录已检测/未检测；
// 若 --ide 显式指定但未检测到，该项标失败。
func checkIDEs(explicit []string) CheckResult {
	cr := CheckResult{Name: "IDE 探测", Severity: SeverityError}

	type pair struct {
		name string
		ok   bool
	}
	var pairs []pair
	detected := make(map[string]bool)
	for _, a := range adapter.All() {
		ok := a.Detect()
		pairs = append(pairs, pair{a.Name(), ok})
		if ok {
			detected[a.Name()] = true
		}
	}

	var dNames, uNames []string
	for _, p := range pairs {
		if p.ok {
			dNames = append(dNames, p.name)
		} else {
			uNames = append(uNames, p.name)
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "已检测: %s；未检测: %s", joinOr(dNames, "无"), joinOr(uNames, "无"))

	cr.OK = true
	if len(explicit) > 0 {
		var missing []string
		for _, name := range explicit {
			if !detected[name] {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			cr.OK = false
			fmt.Fprintf(&sb, "；显式指定但未检测: %s（请确认对应 IDE 已安装）", strings.Join(missing, ","))
		}
	}
	cr.Detail = sb.String()
	return cr
}

// checkWorkInPath 检查 work 可执行文件是否在 PATH 中，
// 失败再用 os.Executable() 兜底。
func checkWorkInPath() CheckResult {
	cr := CheckResult{Name: "work 在 PATH", Severity: SeverityError}
	if _, err := exec.LookPath("work"); err == nil {
		cr.OK = true
		cr.Detail = "已在 PATH 中"
		return cr
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		cr.OK = true
		cr.Detail = fmt.Sprintf("PATH 未命中，但当前可执行文件可用: %s", exe)
		return cr
	}
	cr.OK = false
	cr.Detail = "未在 PATH 中找到 work，请将 work 加入 PATH 后重试"
	return cr
}

// checkConfigYAML 检查 ~/.work/config.yaml 是否合法。
// 文件不存在不算失败（标「未创建」）；解析失败标失败。
func checkConfigYAML() CheckResult {
	cr := CheckResult{Name: "config.yaml 合法", Severity: SeverityError}
	dir, err := platform.WorkConfigDir()
	if err != nil {
		cr.OK = false
		cr.Detail = fmt.Sprintf("无法定位配置目录: %v", err)
		return cr
	}
	path := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cr.OK = true
			cr.Detail = "未创建（不要求存在）"
			return cr
		}
		cr.OK = false
		cr.Detail = fmt.Sprintf("读取失败: %v", err)
		return cr
	}
	if err := ParseConfigYAML(data); err != nil {
		cr.OK = false
		cr.Detail = fmt.Sprintf("解析失败 (%s): %v", path, err)
		return cr
	}
	cr.OK = true
	cr.Detail = fmt.Sprintf("合法: %s", path)
	return cr
}

// checkInstalledJSON 检查 installed.json 可读（state.Open + Load）。
func checkInstalledJSON(scope string) CheckResult {
	cr := CheckResult{Name: "installed.json 可读", Severity: SeverityError}
	path, err := platform.WorkStatePath(scope)
	if err != nil {
		cr.OK = false
		cr.Detail = fmt.Sprintf("无法定位状态文件: %v", err)
		return cr
	}
	store, err := state.Open(path)
	if err != nil {
		cr.OK = false
		cr.Detail = fmt.Sprintf("打开失败: %v", err)
		return cr
	}
	if _, err := store.Load(); err != nil {
		cr.OK = false
		cr.Detail = fmt.Sprintf("读取失败 (%s): %v", path, err)
		return cr
	}
	cr.OK = true
	cr.Detail = fmt.Sprintf("可读: %s", path)
	return cr
}

// checkMCPConfigs 对每个已检测的 IDE，取其 MCP 配置文件路径，
// 若文件存在则 JSON 解析；不存在不算失败，解析失败标失败。
func checkMCPConfigs(scope string) []CheckResult {
	var results []CheckResult
	for _, a := range adapter.All() {
		if !a.Detect() {
			continue
		}
		ideName := a.Name()
		path, err := platform.MCPConfigPath(platform.IDE(ideName), scope)
		if err != nil {
			// 无法定位路径时跳过该 IDE（不影响其它检查）。
			continue
		}
		results = append(results, checkMCPFile(ideName, path))
	}
	return results
}

// checkMCPFile 检查单个 MCP 配置文件。
func checkMCPFile(ideName, path string) CheckResult {
	cr := CheckResult{
		Name:     fmt.Sprintf("MCP 配置合法 (%s)", ideName),
		Severity: SeverityError,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cr.OK = true
			cr.Detail = fmt.Sprintf("未创建（不要求存在）: %s", path)
			return cr
		}
		cr.OK = false
		cr.Detail = fmt.Sprintf("读取失败: %v", err)
		return cr
	}
	if err := ParseMCPJSON(data); err != nil {
		cr.OK = false
		cr.Detail = fmt.Sprintf("解析失败 (%s): %v", path, err)
		return cr
	}
	cr.OK = true
	cr.Detail = fmt.Sprintf("合法: %s", path)
	return cr
}

// checkCodegraph 检查 codegraph 可执行文件是否在 PATH 中。
func checkCodegraph() CheckResult {
	cr := CheckResult{Name: "codegraph 可用", Severity: SeverityError}
	if _, err := exec.LookPath("codegraph"); err == nil {
		cr.OK = true
		cr.Detail = "已在 PATH 中"
		return cr
	}
	cr.OK = false
	cr.Detail = "未找到 codegraph，请安装 codegraph-stack"
	return cr
}

// checkJQ 检查 jq 可执行文件是否在 PATH 中（作为独立检查条目）。
func checkJQ() CheckResult {
	cr := CheckResult{Name: "jq 可用", Severity: SeverityError}
	if _, err := exec.LookPath("jq"); err == nil {
		cr.OK = true
		cr.Detail = "已在 PATH 中"
		return cr
	}
	cr.OK = false
	cr.Detail = "未找到 jq，建议安装（codegraph-stack 依赖）"
	return cr
}

// checkSelfUpdate 读自更新配置，输出 enabled 与 check_interval 概况（信息项，不计失败）。
func checkSelfUpdate() CheckResult {
	cr := CheckResult{Name: "自更新配置", Severity: SeverityInfo}
	cfg, err := selfupdate.LoadConfig()
	if err != nil {
		cr.OK = true
		cr.Detail = fmt.Sprintf("读取配置失败（不影响使用）: %v", err)
		return cr
	}
	cr.OK = true
	cr.Detail = fmt.Sprintf(
		"enabled=%t, check_interval=%s",
		cfg.Enabled, formatDuration(cfg.CheckInterval),
	)
	return cr
}

// ParseConfigYAML 用 yaml.v3 解析 config.yaml 内容。
// 抽离为独立纯函数便于单元测试覆盖「解析失败标失败」分支。
func ParseConfigYAML(data []byte) error {
	var v any
	return yaml.Unmarshal(data, &v)
}

// ParseMCPJSON 用 encoding/json 解析 MCP 配置内容。
// 抽离为独立纯函数便于单元测试覆盖「解析失败标失败」分支。
func ParseMCPJSON(data []byte) error {
	var v any
	return json.Unmarshal(data, &v)
}

// joinOr 将切片用逗号连接，空则返回 fallback。
func joinOr(items []string, fallback string) string {
	if len(items) == 0 {
		return fallback
	}
	return strings.Join(items, ",")
}

// formatDuration 将 duration 格式化为可读字符串，零值返回 "0s"。
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	return d.String()
}

// checkAIConfig 检查 ai.models 段是否合法。
// 若配置存在但默认 profile 不可用则标 warning（info 严重度）；
// 若 ai.models 段完全不存在则仅做 info 提示。
func checkAIConfig() CheckResult {
	cr := CheckResult{Name: "AI 模型配置", Severity: SeverityInfo}
	profiles, err := ai.ListProfiles()
	if err != nil {
		cr.Detail = fmt.Sprintf("读取 ai.models 失败: %v", err)
		return cr
	}
	if len(profiles) == 0 {
		cr.OK = true
		cr.Detail = "未配置 ai.models（可选，不影响其余功能）"
		return cr
	}
	// 尝试加载 default profile，失败说明配置不完整
	cfg, err := ai.LoadModelConfig("default")
	if err != nil {
		cr.Detail = fmt.Sprintf("已配置 %d 个 profile（%s），但默认 profile 加载失败: %v",
			len(profiles), strings.Join(profiles, ", "), err)
		return cr
	}
	cr.OK = true
	cr.Detail = fmt.Sprintf("已配置 %d 个 profile（%s），默认: provider=%s model=%s",
		len(profiles), strings.Join(profiles, ", "), cfg.Provider, cfg.Model)
	return cr
}

// checkRegistryConnectivity 若 registry.url 已配置，探测其连通性（超时 5s）。
func checkRegistryConnectivity() CheckResult {
	cr := CheckResult{Name: "Registry 连通性", Severity: SeverityInfo}
	cfg, err := source.LoadUserConfig()
	if err != nil {
		cr.Detail = fmt.Sprintf("读取配置失败: %v", err)
		return cr
	}
	url := strings.TrimSpace(cfg.Registry.URL)
	if url == "" {
		cr.OK = true
		cr.Detail = "未配置 registry.url（可选）"
		return cr
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head(url)
	if err != nil {
		cr.Detail = fmt.Sprintf("%s — 不可连接: %v", url, err)
		return cr
	}
	resp.Body.Close()
	cr.OK = true
	cr.Detail = fmt.Sprintf("%s — 可连接 (HTTP %d)", url, resp.StatusCode)
	return cr
}

// checkHooksConfig 检查 hooks-installed 目录下的 sidecar 记录是否至少有一份合法。
func checkHooksConfig() CheckResult {
	cr := CheckResult{Name: "Hooks 配置检查", Severity: SeverityInfo}
	dir, err := hooks.HooksInstalledDir()
	if err != nil {
		cr.Detail = fmt.Sprintf("无法定位 hooks-installed 目录: %v", err)
		return cr
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			cr.OK = true
			cr.Detail = "hooks-installed 目录不存在（尚未安装 hooks）"
			return cr
		}
		cr.Detail = fmt.Sprintf("读取 hooks-installed 目录失败: %v", err)
		return cr
	}
	var valid, invalid int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		_, err := hooks.LoadSidecar(name)
		if err == nil {
			valid++
		} else {
			invalid++
		}
	}
	cr.OK = true
	if valid == 0 && invalid == 0 {
		cr.Detail = "hooks-installed 目录为空（尚未安装 hooks）"
	} else if invalid > 0 {
		cr.Detail = fmt.Sprintf("有效 %d / 无效 %d（部分 sidecar 记录解析失败，检查 hooks-installed 目录）", valid, invalid)
	} else {
		cr.Detail = fmt.Sprintf("已安装 %d 个 hooks 套装，sidecar 记录均合法", valid)
	}
	return cr
}

// checkStaleTempFiles 检查常见位置是否有残留的临时文件。
func checkStaleTempFiles() CheckResult {
	cr := CheckResult{Name: "残留临时文件", Severity: SeverityInfo}
	patterns := []string{".work-upgrade-*", ".installed-*.json", ".work-tmp-*"}
	var found []string

	// 检查 HOME 目录
	home, err := platform.UserHome()
	searchDirs := []string{}
	if err == nil {
		searchDirs = append(searchDirs, home)
	}
	// 检查 work 配置目录
	if dir, err := platform.WorkConfigDir(); err == nil {
		searchDirs = append(searchDirs, dir)
	}

	for _, dir := range searchDirs {
		for _, pat := range patterns {
			glob := filepath.Join(dir, pat)
			matches, err := filepath.Glob(glob)
			if err != nil {
				continue
			}
			for _, m := range matches {
				// 排除目录
				if info, err := os.Stat(m); err == nil && info.IsDir() {
					continue
				}
				found = append(found, m)
			}
		}
	}

	if len(found) == 0 {
		cr.OK = true
		cr.Detail = "未发现残留临时文件"
		return cr
	}
	cr.Detail = fmt.Sprintf("发现 %d 个残留临时文件（可安全删除）: %s", len(found), strings.Join(found, ", "))
	return cr
}

// checkFilePermissions 检查 ~/.work/ 目录与 config.yaml 文件权限是否安全。
func checkFilePermissions() CheckResult {
	cr := CheckResult{Name: "文件权限检查", Severity: SeverityError}
	dir, err := platform.WorkConfigDir()
	if err != nil {
		cr.OK = false
		cr.Detail = fmt.Sprintf("无法定位配置目录: %v", err)
		return cr
	}

	// 检查目录
	dirInfo, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			cr.OK = true
			cr.Detail = "~/.work 目录不存在（尚未初始化）"
			return cr
		}
		cr.OK = false
		cr.Detail = fmt.Sprintf("无法读取目录信息: %v", err)
		return cr
	}
	dirPerm := dirInfo.Mode().Perm()
	if dirPerm&0o002 != 0 {
		cr.OK = false
		cr.Detail = fmt.Sprintf("~/.work 目录权限过于宽松 (%s)，应为 0700 或 0755，禁止所有人可写",
			permString(dirPerm))
		return cr
	}

	// 检查 config.yaml
	configPath := filepath.Join(dir, "config.yaml")
	cfgInfo, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cr.OK = true
			cr.Detail = fmt.Sprintf("目录权限安全 (%s)，config.yaml 不存在", permString(dirPerm))
			return cr
		}
		cr.OK = false
		cr.Detail = fmt.Sprintf("无法读取 config.yaml 信息: %v", err)
		return cr
	}
	cfgPerm := cfgInfo.Mode().Perm()
	if cfgPerm&0o077 != 0 {
		cr.OK = false
		cr.Detail = fmt.Sprintf("config.yaml 权限过于宽松 (%s)，应为 0600，禁止同组/其他人读",
			permString(cfgPerm))
		return cr
	}

	cr.OK = true
	cr.Detail = fmt.Sprintf("目录 %s，config.yaml %s，均安全",
		permString(dirPerm), permString(cfgPerm))
	return cr
}

// permString 将 os.FileMode 权位格式化为 octor 字符串，如 "0700"。
// 仅显示 Unix 权限位（低 12 位：特殊位 + rwx×3）。
func permString(mode fs.FileMode) string {
	bits := mode & fs.ModePerm
	return fmt.Sprintf("0%o", bits)
}
