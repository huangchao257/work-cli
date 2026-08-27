package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/api"
	"github.com/huangchao257/work-cli/internal/output"
	"github.com/huangchao257/work-cli/internal/usage"
)

var (
	apiRegistry *api.Registry
	apiFrozen   bool
)

// apiDeps 是 api 命令族的依赖，便于测试注入。
type apiDeps struct {
	registry *api.Registry
}

// defaultAPIDeps 组装生产依赖：内置系统 + 导入系统 + 全局 timeout 配置。
func defaultAPIDeps() *apiDeps {
	return &apiDeps{registry: api.DefaultRegistry}
}

// collectSystems 返回全部系统（内置 + 导入），并收集导入诊断 warning。
func (d *apiDeps) collectSystems() ([]api.System, []string) {
	var systems []api.System
	var warnings []string
	systems = append(systems, d.registry.Systems()...)
	imported, importWarnings := api.ImportedSystems()
	systems = append(systems, imported...)
	warnings = append(warnings, importWarnings...)
	sort.Slice(systems, func(i, j int) bool {
		return systems[i].Manifest().Name < systems[j].Manifest().Name
	})
	return systems, warnings
}

func (d *apiDeps) findSystem(name string) (api.System, *api.SystemConfig, error) {
	if s, ok := d.registry.ByName(name); ok {
		return s, nil, nil
	}
	cfg, exists, err := api.LoadSystemConfig(name)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return api.NewConfigSystem(cfg), cfg, nil
	}
	return nil, nil, usage.Newf("系统 %s 不存在，运行 work api list 查看可用系统", name)
}

// apiTimeout 读取全局超时配置。
func apiTimeout() string {
	options, err := loadAPITimeoutOptions()
	if err != nil || options.API.Timeout == "" {
		return ""
	}
	return options.API.Timeout
}

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "系统接口 CLI 化（OpenAPI 驱动 + 三层命令）",
	Long: `将内部/第三方系统的 OpenAPI 规范变成统一 CLI 命令。

三层调用：
  L1  work api <system> +<shortcut>          快捷方式（意图级，含默认参数）
  L2  work api <system> <cli-path> [flags]   OpenAPI 动态生成的类型化命令
  L3  work api call <system> <METHOD> <PATH> 通用调用兜底（--params/--data）

管理：
  work api import <name> <spec>   导入 OpenAPI 规范（本地文件或 https:// URL）
  work api refresh [system]       重新拉取远程规范
  work api remove <system>        删除导入的系统
  work api schema <system> [op]   渐进式查看系统/操作契约
  work api list                   列出全部系统
  work api info <system>          查看系统详情与鉴权状态`,
	Example: `  work api list
  work api schema demo --compact
  work api demo pets list-pets --limit 2 --json
  work api demo +seed --dry-run
  work api call demo GET /pets --params '{"limit":2}'`,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return apiUnknownTarget(cmd, "子命令", args[0], "运行 work api --help 查看可用命令")
		}
		return cmd.Help()
	},
}

func init() {
	// 生产装配：内置系统在各自 api_builtin_<system>.go 中 blank-import 注册，
	// 导入系统目录由 collectSystems 动态发现。
	rootCmd.AddCommand(apiCmd)
	apiCmd.AddCommand(
		apiListCmd, apiInfoCmd, apiImportCmd, apiRefreshCmd, apiRemoveCmd,
		apiSchemaCmd, apiCallCmd,
	)
	// api_dynamic.go 的 init 在动态命令装配后再次调用，覆盖新挂载的命令
	// （per-command 幂等：已安装过的命令跳过，新命令补装）
	setAPIErrorFuncs(apiCmd)
}

// apiErrorFuncsApplied 记录已安装过兜底错误函数的命令（per-command 幂等）。
// 布尔守卫会挡住 api_dynamic.go init 对新挂载动态命令的第二次遍历，
// 导致 clone 树动态命令的 flag 错误静默；按命令标记则每次调用都能覆盖新子树。
var apiErrorFuncsApplied = map[*cobra.Command]bool{}

// setAPIErrorFuncs 递归为 api 子树安装兜底错误函数（per-command 幂等，可重复调用）。
// Args 校验/flag 解析错误发生在 RunE 之前，apiFail 拦不到；
// 用统一的 FlagErrorFunc + Args 包装兜底输出（SilenceErrors 下不再静默）。
func setAPIErrorFuncs(cmd *cobra.Command) {
	applyAPIErrorFuncs(cmd)
}

func applyAPIErrorFuncs(cmd *cobra.Command) {
	if !apiErrorFuncsApplied[cmd] {
		apiErrorFuncsApplied[cmd] = true
		cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
			return apiFail(c, usage.New(err.Error()))
		})
		if inner := cmd.Args; inner != nil {
			cmd.Args = func(c *cobra.Command, args []string) error {
				if err := inner(c, args); err != nil {
					return apiFail(c, usage.New(err.Error()))
				}
				return nil
			}
		}
	}
	// 无论本命令是否已装，都要递归子命令：后续挂载的动态命令需要补装
	for _, sub := range cmd.Commands() {
		applyAPIErrorFuncs(sub)
	}
}

// --- list ---

var apiListCmd = &cobra.Command{
	Use:           "list",
	Short:         "列出全部可用系统",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := defaultAPIDeps()
		systems, warnings := deps.collectSystems()
		type systemRow struct {
			Name        string `json:"name"`
			Source      string `json:"source"`
			Description string `json:"description,omitempty"`
			Operations  int    `json:"operations"`
			Shortcuts   int    `json:"shortcuts"`
		}
		rows := []systemRow{}
		for _, s := range systems {
			m := s.Manifest()
			row := systemRow{Name: m.Name, Source: m.Source, Description: m.Description}
			if catalog, err := s.Catalog(cmd.Context()); err == nil {
				row.Operations = len(catalog.Operations)
			}
			if _, cfg, err := deps.findSystem(m.Name); err == nil {
				if cfg != nil {
					row.Shortcuts = len(cfg.Shortcuts)
				}
			}
			if provider, ok := s.(api.Shortcuts); ok {
				row.Shortcuts += len(provider.Shortcuts())
			}
			rows = append(rows, row)
		}
		payload := map[string]any{"systems": rows}
		if len(warnings) > 0 {
			payload["warnings"] = warnings
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), payload)
		}
		w := cmd.OutOrStdout()
		if len(rows) == 0 {
			fmt.Fprintln(w, "暂无系统。运行 work api import <name> <spec> 导入 OpenAPI 规范。")
			return nil
		}
		fmt.Fprintf(w, "%-16s %-9s %-6s %-9s %s\n", "系统", "来源", "操作数", "快捷方式", "描述")
		for _, row := range rows {
			fmt.Fprintf(w, "%-16s %-9s %-6d %-9d %s\n", row.Name, row.Source, row.Operations, row.Shortcuts, row.Description)
		}
		for _, warning := range warnings {
			fmt.Fprintf(cmd.ErrOrStderr(), "⚠ %s\n", warning)
		}
		return nil
	},
}

// --- info ---

var apiInfoCmd = &cobra.Command{
	Use:           "info <system>",
	Short:         "查看系统详情与鉴权状态",
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := defaultAPIDeps()
		s, cfg, err := deps.findSystem(args[0])
		if err != nil {
			return apiFail(cmd, err)
		}
		m := s.Manifest()
		catalog, err := s.Catalog(cmd.Context())
		if err != nil {
			return apiFail(cmd, fmt.Errorf("读取系统目录失败: %w", err))
		}
		var authCfg api.AuthConfig
		shortcuts, _ := api.BuildShortcuts(s, cfg)
		if cfg != nil {
			authCfg = cfg.Auth
		}
		type infoPayload struct {
			Name        string   `json:"name"`
			Source      string   `json:"source"`
			Description string   `json:"description,omitempty"`
			Title       string   `json:"title"`
			Version     string   `json:"spec_version"`
			OpenAPI     string   `json:"openapi"`
			BaseURL     string   `json:"base_url,omitempty"`
			Auth        string   `json:"auth"`
			Operations  int      `json:"operations"`
			Shortcuts   []string `json:"shortcuts,omitempty"`
			Warnings    []string `json:"warnings,omitempty"`
		}
		payload := infoPayload{
			Name: m.Name, Source: m.Source, Description: m.Description,
			Title: catalog.Title, Version: catalog.Version, OpenAPI: catalog.OpenAPI,
			BaseURL: s.BaseURL(), Auth: api.AuthStatusText(s, authCfg),
			Operations: len(catalog.Operations), Warnings: catalog.Warnings,
		}
		for _, sc := range shortcuts {
			payload.Shortcuts = append(payload.Shortcuts, sc.Name)
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), payload)
		}
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "系统: %s（%s）\n", payload.Name, payload.Source)
		if payload.Description != "" {
			fmt.Fprintf(w, "描述: %s\n", payload.Description)
		}
		fmt.Fprintf(w, "规范: %s %s（OpenAPI %s）\n", payload.Title, payload.Version, payload.OpenAPI)
		if payload.BaseURL != "" {
			fmt.Fprintf(w, "Base URL: %s\n", payload.BaseURL)
		} else {
			fmt.Fprintf(w, "Base URL: [未配置，离线系统]\n")
		}
		fmt.Fprintf(w, "鉴权: %s\n", payload.Auth)
		fmt.Fprintf(w, "操作数: %d\n", payload.Operations)
		if len(payload.Shortcuts) > 0 {
			fmt.Fprintf(w, "快捷方式: %s\n", strings.Join(payload.Shortcuts, ", "))
		}
		for _, warning := range payload.Warnings {
			fmt.Fprintf(w, "⚠ %s\n", warning)
		}
		return nil
	},
}

// --- import ---

var (
	apiImportBaseURL    string
	apiImportAuthKind   string
	apiImportCredEnv    string
	apiImportAuthHeader string
	apiImportAuthQuery  string
	apiImportOverwrite  bool
)

var apiImportCmd = &cobra.Command{
	Use:   "import <name> <spec>",
	Short: "导入 OpenAPI 规范作为可调用系统",
	Long: `导入 OpenAPI 3.0/3.1 规范（本地 YAML/JSON 文件或 https:// URL），
生成动态命令目录并缓存规范快照。凭据通过环境变量注入，不在命令行传递明文。`,
	Example: `  work api import petstore ./petstore.yaml
  work api import petstore https://example.com/openapi.json \
    --auth bearer --credential-env PETSTORE_TOKEN
  work api import petstore ./petstore.yaml --overwrite`,
	Args:          cobra.ExactArgs(2),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := api.ImportOptions{
			Name: args[0], Spec: args[1],
			BaseURL: apiImportBaseURL, AuthKind: apiImportAuthKind,
			CredentialEnv: apiImportCredEnv, AuthHeader: apiImportAuthHeader,
			AuthQuery: apiImportAuthQuery, Overwrite: apiImportOverwrite, DryRun: dryRun,
		}
		result, err := api.Import(cmd.Context(), opts)
		if err != nil {
			return apiFail(cmd, err)
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), result)
		}
		w := cmd.OutOrStdout()
		label := "已导入"
		if result.DryRun {
			label = "预览导入"
		}
		fmt.Fprintf(w, "%s %s（%s %s）：%d 个操作，%d 个动态命令\n",
			label, result.Name, result.Title, result.Version, result.Operations, result.DynamicCmds)
		for _, warning := range result.Warnings {
			fmt.Fprintf(w, "⚠ %s\n", warning)
		}
		return nil
	},
}

// --- refresh ---

var apiRefreshCmd = &cobra.Command{
	Use:           "refresh [system]",
	Short:         "重新拉取远程 OpenAPI 规范",
	Long:          `仅刷新导入时记录了 HTTPS source_url 的系统；本地导入的系统会被跳过。`,
	Args:          cobra.MaximumNArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		results, err := api.Refresh(cmd.Context(), api.RefreshOptions{Name: name, DryRun: dryRun})
		if err != nil {
			return apiFail(cmd, err)
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), map[string]any{"results": results})
		}
		w := cmd.OutOrStdout()
		for _, result := range results {
			if result.Updated {
				fmt.Fprintf(w, "✓ %s 已刷新（%d 个操作）\n", result.Name, result.Operations)
			} else {
				fmt.Fprintf(w, "- %s %s\n", result.Name, result.Reason)
			}
			for _, warning := range result.Warnings {
				fmt.Fprintf(w, "⚠ %s\n", warning)
			}
		}
		return nil
	},
}

// --- remove ---

var apiRemoveYes bool

var apiRemoveCmd = &cobra.Command{
	Use:           "remove <system>",
	Short:         "删除导入的系统",
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if !api.SystemExists(name) {
			if _, ok := api.DefaultRegistry.ByName(name); ok {
				return apiFail(cmd, fmt.Errorf("内置系统 %s 不可删除", name))
			}
			return apiFail(cmd, usage.Newf("系统 %s 不存在，运行 work api list 查看可用系统", name))
		}
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "预览删除系统 %s\n", name)
			return nil
		}
		if !apiRemoveYes {
			confirmed := false
			if stat, err := os.Stdin.Stat(); err == nil && stat.Mode()&os.ModeCharDevice != 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "将删除系统 %s 的全部本地数据（规范、目录、配置），确认？[y/N] ", name)
				var answer string
				if _, err := fmt.Scanln(&answer); err == nil {
					answer = strings.ToLower(strings.TrimSpace(answer))
					confirmed = answer == "y" || answer == "yes"
				}
			}
			if !confirmed {
				return apiFail(cmd, fmt.Errorf("已取消删除（非交互环境请追加 --yes）"))
			}
		}
		if err := api.RemoveSystem(name); err != nil {
			return apiFail(cmd, err)
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), map[string]any{"removed": name})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ 已删除系统 %s\n", name)
		return nil
	},
}

func loadAPITimeoutOptions() (apiTimeoutOptions, error) {
	return loadAPITimeoutFromConfig()
}

func init() {
	apiImportCmd.Flags().StringVar(&apiImportBaseURL, "base-url", "", "覆盖规范 servers 的 base URL")
	apiImportCmd.Flags().StringVar(&apiImportAuthKind, "auth", "none", "鉴权类型：none | bearer | apikey")
	apiImportCmd.Flags().StringVar(&apiImportCredEnv, "credential-env", "", "凭据环境变量名（不保存明文）")
	apiImportCmd.Flags().StringVar(&apiImportAuthHeader, "auth-header", "", "apikey 模式的 header 名（默认 X-API-Key）")
	apiImportCmd.Flags().StringVar(&apiImportAuthQuery, "auth-query", "", "apikey 模式的 query 参数名（与 header 二选一）")
	apiImportCmd.Flags().BoolVar(&apiImportOverwrite, "overwrite", false, "覆盖已存在的同名系统")
	apiRemoveCmd.Flags().BoolVar(&apiRemoveYes, "yes", false, "跳过删除确认")
}
