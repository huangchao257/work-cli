package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/api"
	"github.com/huangchao257/work-cli/internal/output"
)

// registerDynamicAPICommands 在进程启动时为每个系统挂载 L1/L2 动态命令。
// 仅读取 system.yaml + catalog.json（不解析完整规范、不联网）；
// 单个系统损坏时跳过其动态命令，不阻塞 CLI 启动。
func registerDynamicAPICommands() {
	registered := map[string]bool{}

	// 内置系统（编译期注册）
	for _, s := range api.DefaultRegistry.Systems() {
		registerSystemCommands(s, nil, registered)
	}
	// 导入系统（配置驱动）
	imported, _ := api.ImportedSystems()
	for _, s := range imported {
		m := s.Manifest()
		cfg, _, _ := api.LoadSystemConfig(m.Name)
		registerSystemCommands(s, cfg, registered)
	}
}

// attachSystemCmd 指定动态系统命令的挂载目标（测试用 clone 树注入）。
var attachSystemCmd = func(cmd *cobra.Command) { apiCmd.AddCommand(cmd) }

func registerSystemCommands(s api.System, cfg *api.SystemConfig, registered map[string]bool) {
	m := s.Manifest()
	if registered[m.Name] {
		return
	}
	registered[m.Name] = true

	catalog, err := s.Catalog(nil)
	if err != nil {
		// 启动期静默跳过；诊断在 work api list/info 中暴露
		return
	}

	systemCmd := &cobra.Command{
		Use:    m.Name,
		Short:  fmt.Sprintf("调用 %s 系统接口（%s）", m.Name, firstNonEmpty(m.Description, catalog.Title)),
		Hidden: false,
		Long: fmt.Sprintf(`%s（%s %s）

L1  work api %s +<shortcut>          快捷方式
L2  work api %s <cli-path> [flags]   类型化命令
L3  work api call %s <op|METHOD PATH> 通用调用

运行 work api schema %s 查看全部操作。`,
			m.Name, catalog.Title, catalog.Version, m.Name, m.Name, m.Name, m.Name),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return apiUnknownTarget(cmd, "子命令", strings.Join(args, " "), "运行 work api "+m.Name+" --help 查看可用命令")
			}
			return cmd.Help()
		},
	}

	// L1 shortcut 命令
	shortcuts, _ := api.BuildShortcuts(s, cfg)
	for _, sc := range shortcuts {
		systemCmd.AddCommand(newShortcutCmd(s, cfg, catalog, sc))
	}

	// L2 动态类型化命令（按 cli-path 层级嵌套）；
	// 单条装配失败（如异常目录数据）跳过该系统其余命令，不影响其他系统
	func() {
		defer func() {
			if r := recover(); r != nil {
				// 启动期不做任何输出，保住整棵命令树；诊断走 work api list/info
				systemCmd.Commands()
			}
		}()
		for i := range catalog.Operations {
			op := catalog.Operations[i]
			if !op.Dynamic {
				continue
			}
			attachOperationCmd(systemCmd, s, cfg, &op)
		}
	}()

	attachSystemCmd(systemCmd)
}

// newShortcutCmd 构造 L1 快捷命令（+name）。
func newShortcutCmd(s api.System, cfg *api.SystemConfig, catalog *api.CatalogAlias, sc api.Shortcut) *cobra.Command {
	name := strings.TrimPrefix(sc.Name, "+")
	risk := api.EffectiveShortcutRisk(catalog, sc)
	cmd := &cobra.Command{
		Use:     "+" + name,
		Short:   firstNonEmpty(sc.Description, "快捷方式 "+sc.Target),
		Aliases: []string{name},
		Hidden:  false,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPIShortcut(cmd, s, cfg, sc)
		},
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.Flags().BoolVar(&apiCallYes, "yes", false, "跳过写操作确认")
	cmd.Flags().StringVar(&apiCallData, "data", "", "请求体 JSON（- stdin，@file 相对路径）")
	cmd.Flags().StringArrayVar(&apiCallSet, "set", nil, "单个参数 k=v")
	cmd.Flags().StringArrayVar(&apiCallParams, "params", nil, "query 参数 JSON 对象")
	cmd.Flags().StringArrayVar(&apiCallHeader, "header", nil, "附加 header k=v")
	if risk != api.RiskRead {
		cmd.Short = fmt.Sprintf("%s（风险 %s）", cmd.Short, risk)
	}
	return cmd
}

// runAPIShortcut 执行快捷方式：先做统一风险门禁，再进入编排/合并参数逻辑。
func runAPIShortcut(cmd *cobra.Command, s api.System, cfg *api.SystemConfig, sc api.Shortcut) error {
	catalog, err := s.Catalog(cmd.Context())
	if err != nil {
		return apiFail(cmd, fmt.Errorf("读取系统目录失败: %w", err))
	}
	risk := api.EffectiveShortcutRisk(catalog, sc)

	var authCfg api.AuthConfig
	if cfg != nil {
		authCfg = cfg.Auth
	}
	params, err := mergeCallParams(apiCallParams, apiCallSet)
	if err != nil {
		return apiFail(cmd, err)
	}
	data, err := readCallData(apiCallData)
	if err != nil {
		return apiFail(cmd, err)
	}
	headers, err := parseCallHeaders(apiCallHeader)
	if err != nil {
		return apiFail(cmd, err)
	}
	yes := apiCallYes
	// 包级 flag 变量读后即清（防同进程多次 Execute 残留绕过确认门禁）
	resetCallFlagState()

	opts := api.CallOptions{
		System: s.Manifest().Name, Operation: sc.Target,
		Params: params, Headers: headers, Body: data,
		Yes: yes, DryRun: dryRun,
		Confirm:    confirmTTY(cmd),
		AuthConfig: authCfg, Timeout: apiTimeout(),
	}

	// dry-run：仅预览，不确认不发送。统一走 ExecuteShortcut 合并预设参数，保证预览与实际请求一致
	if dryRun {
		if sc.Handler != nil {
			return renderShortcutDryRun(cmd, s, sc, opts)
		}
		result, err := api.ExecuteShortcut(cmd.Context(), s, sc, opts)
		if err != nil {
			return apiErrorToExit(cmd, err)
		}
		return renderCallResult(cmd, result)
	}

	// 统一风险门禁（handler 型 shortcut 无法预览请求，同样 fail-closed）
	if api.Confirmable(risk) && !apiCallYes {
		confirmed := false
		if opts.Confirm != nil {
			confirmed, err = opts.Confirm(api.ConfirmSummary{
				System: opts.System, Operation: sc.Name, Risk: risk, Body: opts.Body,
			})
			if err != nil {
				return err
			}
		}
		if !confirmed {
			return apiErrorToExit(cmd, api.NewConfirmationRequired(
				opts.System, sc.Name, "", "", risk, "非交互环境且未提供 --yes",
			))
		}
	}

	result, err := api.ExecuteShortcut(cmd.Context(), s, sc, opts)
	if err != nil {
		return apiErrorToExit(cmd, err)
	}
	return renderCallResult(cmd, result)
}

func renderShortcutDryRun(cmd *cobra.Command, s api.System, sc api.Shortcut, opts api.CallOptions) error {
	// handler 型：展示声明信息（handler 可能编排多请求，无法安全预览）
	if asJSON {
		// 与其他 dry-run 一致的 JSON 信封契约（--json 成功输出必须可被 Agent 解析）
		return output.PrintJSON(cmd.OutOrStdout(), map[string]any{
			"ok":        true,
			"system":    opts.System,
			"operation": sc.Name,
			"dry_run":   true,
			"note":      "handler 型快捷方式可能编排多个请求，不预览具体调用",
		})
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "✓ 快捷方式 %s（dry-run，handler 型不预览请求）\n", sc.Name)
	if sc.Description != "" {
		fmt.Fprintf(w, "  %s\n", sc.Description)
	}
	return nil
}

// attachOperationCmd 按 cli-path 层级挂载 L2 命令，并生成类型化 flags。
func attachOperationCmd(systemCmd *cobra.Command, s api.System, cfg *api.SystemConfig, op *api.CatalogOperationAlias) {
	parent := systemCmd
	// 中间层：分组节点
	for i := 0; i < len(op.CLIPath)-1; i++ {
		segment := op.CLIPath[i]
		var next *cobra.Command
		for _, child := range parent.Commands() {
			if child.Name() == segment {
				next = child
				break
			}
		}
		if next == nil {
			groupLabel := segment
			if len(op.Tags) > 0 && strings.TrimSpace(op.Tags[0]) != "" {
				groupLabel = op.Tags[0] + " 资源组"
			}
			seg := segment
			next = &cobra.Command{
				Use:   seg,
				Short: groupLabel,
				RunE: func(cmd *cobra.Command, args []string) error {
					if len(args) > 0 {
						return apiUnknownTarget(cmd, "子命令", strings.Join(args, " "),
							"运行 work api "+s.Manifest().Name+" "+seg+" --help 查看可用命令")
					}
					return cmd.Help()
				},
				SilenceErrors: true,
				SilenceUsage:  true,
			}
			parent.AddCommand(next)
		}
		parent = next
	}

	leafName := op.CLIPath[len(op.CLIPath)-1]
	operationRef := op.ID
	if operationRef == "" {
		operationRef = op.Method + " " + op.Path
	}
	short := firstNonEmpty(op.Summary, operationRef)
	if op.Risk != "read" {
		short = fmt.Sprintf("%s（风险 %s）", short, op.Risk)
	}
	cmd := &cobra.Command{
		Use:   leafName,
		Short: short,
		Args:  cobra.NoArgs, // 多余位置参数多为拼错的子命令，静默丢弃会执行错误操作
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDynamicOperation(cmd, s, cfg, op, operationRef)
		},
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.Flags().BoolVar(&apiCallYes, "yes", false, "跳过写操作确认")
	cmd.Flags().StringVar(&apiCallData, "data", "", "请求体 JSON（- stdin，@file 相对路径）")
	cmd.Flags().StringArrayVar(&apiCallSet, "set", nil, "单个参数 k=v")
	cmd.Flags().StringArrayVar(&apiCallParams, "params", nil, "query 参数 JSON 对象")
	cmd.Flags().StringArrayVar(&apiCallHeader, "header", nil, "附加 header k=v")

	// 类型化 flags：从目录元数据生成
	for _, p := range op.Parameters {
		if !p.FlagEnabled {
			continue
		}
		value := new(string)
		usage := fmt.Sprintf("%s 参数（%s）", p.In, p.Type)
		if p.Required {
			usage += "，必填"
		}
		if len(p.Enum) > 0 {
			usage += "，可选: " + strings.Join(p.Enum, "|")
		}
		if p.Description != "" {
			usage += "：" + p.Description
		}
		cmd.Flags().StringVar(value, p.Flag, "", usage)
		if len(p.Enum) > 0 {
			_ = cmd.RegisterFlagCompletionFunc(p.Flag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
				return p.Enum, cobra.ShellCompDirectiveNoFileComp
			})
		}
	}

	parent.AddCommand(cmd)
}

// runDynamicOperation 执行 L2 动态命令：类型化 flag → 参数合并 → 统一 Call。
func runDynamicOperation(cmd *cobra.Command, s api.System, cfg *api.SystemConfig, op *api.CatalogOperationAlias, operationRef string) error {
	var authCfg api.AuthConfig
	if cfg != nil {
		authCfg = cfg.Auth
	}
	params, err := mergeCallParams(apiCallParams, apiCallSet)
	if err != nil {
		return apiFail(cmd, err)
	}
	// 类型化 flag 值写入参数（显式 flag 优先）；同时删除同位置裸名键，
	// 避免裸名落入"未放置键"循环覆盖显式 flag 值
	for _, p := range op.Parameters {
		if !p.FlagEnabled {
			continue
		}
		if cmd.Flags().Changed(p.Flag) {
			value, _ := cmd.Flags().GetString(p.Flag)
			params[p.In+"."+p.Name] = value
			delete(params, p.Name)
		}
	}
	data, err := readCallData(apiCallData)
	if err != nil {
		return apiFail(cmd, err)
	}
	headers, err := parseCallHeaders(apiCallHeader)
	if err != nil {
		return apiFail(cmd, err)
	}
	yes := apiCallYes
	// 包级 flag 变量读后即清（防同进程多次 Execute 残留绕过确认门禁）
	resetCallFlagState()

	opts := api.CallOptions{
		System: s.Manifest().Name, Operation: operationRef,
		Params: params, Headers: headers, Body: data,
		Yes: yes, DryRun: dryRun,
		Confirm:    confirmTTY(cmd),
		AuthConfig: authCfg, Timeout: apiTimeout(),
	}
	result, err := api.Call(cmd.Context(), s, opts)
	if err != nil {
		return apiErrorToExit(cmd, err)
	}
	return renderCallResult(cmd, result)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func init() {
	// 动态命令装配在包初始化时完成（内置系统由 blank-import 顺序保证先注册）。
	// 装配前冻结注册表：此后任何 Register 都会显式报错而非静默丢命令。
	DefaultRegistry := api.DefaultRegistry
	DefaultRegistry.Freeze()
	registerDynamicAPICommands()
	// 之后为新增的动态命令安装兜底错误函数（per-command 幂等，可重复调用）
	setAPIErrorFuncs(apiCmd)
}

// CatalogAlias / CatalogOperationAlias 保留类型别名，便于将来替换数据源。
type CatalogAlias = api.CatalogAlias
