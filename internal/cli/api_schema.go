package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/api"
	"github.com/huangchao257/work-cli/internal/openapi"
	"github.com/huangchao257/work-cli/internal/output"
)

var (
	apiSchemaCompact bool
	apiSchemaAll     bool
)

var apiSchemaCmd = &cobra.Command{
	Use:   "schema <system> [operation|cli-path]",
	Short: "渐进式查看系统与操作契约",
	Long: `不发送请求，只展示系统接口目录。

  work api schema <system>              系统概览（按 tag 分组的操作列表）
  work api schema <system> <operation>  单操作详情（参数/请求体/响应码）
  work api schema <system> --all        输出完整目录（默认截断长列表）

--compact 使用正向字段白名单投影，适合 Agent 上下文。`,
	Example: `  work api schema demo
  work api schema demo createPet --compact
  work api schema demo --all --json`,
	Args:          cobra.RangeArgs(1, 2),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := defaultAPIDeps()
		s, cfg, err := deps.findSystem(args[0])
		if err != nil {
			return apiFail(cmd, err)
		}
		catalog, err := s.Catalog(cmd.Context())
		if err != nil {
			return apiFail(cmd, fmt.Errorf("读取系统目录失败: %w", err))
		}
		shortcuts, _ := api.BuildShortcuts(s, cfg)

		if len(args) == 2 {
			return renderOperationSchema(cmd, catalog, args[1], apiSchemaCompact)
		}
		return renderSystemSchema(cmd, catalog, shortcuts, apiSchemaCompact, apiSchemaAll)
	},
}

func init() {
	apiSchemaCmd.Flags().BoolVar(&apiSchemaCompact, "compact", false, "紧凑正向投影输出")
	apiSchemaCmd.Flags().BoolVar(&apiSchemaAll, "all", false, "输出完整目录（默认列表截断）")
}

// resolveSchemaTarget 按 operationId、METHOD PATH 或 cli-path 查找操作。
func resolveSchemaTarget(catalog *openapi.Catalog, ref string) (*openapi.CatalogOperation, bool) {
	ref = strings.TrimSpace(ref)
	if op, ok := catalog.FindByID(ref); ok {
		return op, true
	}
	parts := strings.Fields(ref)
	if len(parts) == 2 {
		method, path := strings.ToUpper(parts[0]), parts[1]
		if op, ok := catalog.FindByHTTP(method, path); ok {
			return op, true
		}
	}
	if op, ok := catalog.FindByCLIPath(strings.Fields(strings.ReplaceAll(ref, "/", " "))); ok {
		return op, true
	}
	return nil, false
}

func renderSystemSchema(cmd *cobra.Command, catalog *openapi.Catalog, shortcuts []api.Shortcut, compact, all bool) error {
	type schemaOperation struct {
		ID         string   `json:"id,omitempty"`
		CLIPath    string   `json:"cli_path,omitempty"`
		Method     string   `json:"method"`
		Path       string   `json:"path"`
		Summary    string   `json:"summary,omitempty"`
		Risk       string   `json:"risk,omitempty"`
		Deprecated bool     `json:"deprecated,omitempty"`
		Warnings   []string `json:"warnings,omitempty"`
	}
	type schemaShortcut struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Target      string `json:"target,omitempty"`
	}
	type schemaPayload struct {
		Title      string            `json:"title"`
		Version    string            `json:"version"`
		OpenAPI    string            `json:"openapi"`
		Operations []schemaOperation `json:"operations"`
		Shortcuts  []schemaShortcut  `json:"shortcuts,omitempty"`
		Warnings   []string          `json:"warnings,omitempty"`
		Truncated  bool              `json:"truncated,omitempty"`
	}

	payload := schemaPayload{
		Title: catalog.Title, Version: catalog.Version, OpenAPI: catalog.OpenAPI,
		Operations: []schemaOperation{}, Warnings: catalog.Warnings,
	}
	const maxList = 60
	for i, op := range catalog.Operations {
		if !all && i >= maxList {
			payload.Truncated = true
			break
		}
		entry := schemaOperation{
			ID: op.ID, Method: op.Method, Path: op.Path, Summary: op.Summary,
			Risk: op.Risk, Deprecated: op.Deprecated, Warnings: op.Warnings,
		}
		if op.Dynamic {
			entry.CLIPath = strings.Join(op.CLIPath, " ")
		}
		if compact {
			entry.Warnings = nil
			entry.Deprecated = false
		}
		payload.Operations = append(payload.Operations, entry)
	}
	for _, sc := range shortcuts {
		payload.Shortcuts = append(payload.Shortcuts, schemaShortcut{
			Name: sc.Name, Description: sc.Description, Target: sc.Target,
		})
	}

	if asJSON {
		return output.PrintJSON(cmd.OutOrStdout(), payload)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s（%s，OpenAPI %s）\n", payload.Title, payload.Version, payload.OpenAPI)
	if len(payload.Shortcuts) > 0 {
		fmt.Fprintln(w, "\n快捷方式（L1）:")
		for _, sc := range payload.Shortcuts {
			fmt.Fprintf(w, "  %-24s %s\n", sc.Name, sc.Description)
		}
	}
	fmt.Fprintln(w, "\n操作（L2 cli-path / L3 METHOD PATH）:")
	fmt.Fprintf(w, "  %-40s %-7s %-24s %s\n", "cli-path / operationId", "method", "path", "summary")
	for _, op := range payload.Operations {
		label := op.ID
		if op.CLIPath != "" {
			label = op.CLIPath
		}
		if op.Deprecated {
			label += "（已废弃）"
		}
		fmt.Fprintf(w, "  %-40s %-7s %-24s %s\n", label, op.Method, op.Path, op.Summary)
	}
	if payload.Truncated {
		fmt.Fprintf(w, "\n… 已截断，使用 --all 查看全部 %d 个操作\n", len(catalog.Operations))
	}
	for _, warning := range payload.Warnings {
		fmt.Fprintf(w, "⚠ %s\n", warning)
	}
	return nil
}

func renderOperationSchema(cmd *cobra.Command, catalog *openapi.Catalog, ref string, compact bool) error {
	op, ok := resolveSchemaTarget(catalog, ref)
	if !ok {
		return apiFail(cmd, fmt.Errorf("未找到操作 %q，运行 work api schema 查看全部操作", ref))
	}

	type paramView struct {
		Name        string   `json:"name"`
		In          string   `json:"in"`
		Required    bool     `json:"required,omitempty"`
		Type        string   `json:"type,omitempty"`
		Flag        string   `json:"flag,omitempty"`
		Enum        []string `json:"enum,omitempty"`
		Default     any      `json:"default,omitempty"`
		Description string   `json:"description,omitempty"`
	}
	type opView struct {
		ID               string      `json:"id,omitempty"`
		CLIPath          string      `json:"cli_path,omitempty"`
		Method           string      `json:"method"`
		Path             string      `json:"path"`
		Summary          string      `json:"summary,omitempty"`
		Description      string      `json:"description,omitempty"`
		Deprecated       bool        `json:"deprecated,omitempty"`
		Risk             string      `json:"risk"`
		Parameters       []paramView `json:"parameters,omitempty"`
		BodyRequired     bool        `json:"body_required,omitempty"`
		BodyContentTypes []string    `json:"body_content_types,omitempty"`
		Responses        []string    `json:"responses,omitempty"`
		Warnings         []string    `json:"warnings,omitempty"`
	}
	view := opView{
		ID: op.ID, Method: op.Method, Path: op.Path, Summary: op.Summary,
		Description: op.Description, Deprecated: op.Deprecated, Risk: op.Risk,
		BodyRequired: op.BodyRequired, BodyContentTypes: op.BodyContentTypes,
		Responses: op.Responses, Warnings: op.Warnings,
	}
	if op.Dynamic {
		view.CLIPath = strings.Join(op.CLIPath, " ")
	}
	for _, p := range op.Parameters {
		entry := paramView{
			Name: p.Name, In: p.In, Required: p.Required, Type: p.Type,
			Enum: p.Enum, Default: p.Default, Description: p.Description,
		}
		if p.FlagEnabled {
			entry.Flag = "--" + p.Flag
		}
		view.Parameters = append(view.Parameters, entry)
	}
	if compact {
		view.Description = ""
		view.Warnings = nil
		for i := range view.Parameters {
			view.Parameters[i].Description = ""
		}
	}

	if asJSON {
		return output.PrintJSON(cmd.OutOrStdout(), view)
	}
	w := cmd.OutOrStdout()
	title := view.ID
	if title == "" {
		title = view.Method + " " + view.Path
	}
	fmt.Fprintf(w, "%s（%s %s，风险 %s）\n", title, view.Method, view.Path, view.Risk)
	if view.CLIPath != "" {
		fmt.Fprintf(w, "命令: work api <system> %s\n", view.CLIPath)
	}
	if view.Summary != "" {
		fmt.Fprintf(w, "摘要: %s\n", view.Summary)
	}
	if len(view.Parameters) > 0 {
		fmt.Fprintln(w, "参数:")
		for _, p := range view.Parameters {
			flag := ""
			if p.Flag != "" {
				flag = " " + p.Flag
			}
			required := ""
			if p.Required {
				required = "（必填）"
			}
			fmt.Fprintf(w, "  %-28s %-8s %s%s\n", p.In+"."+p.Name+flag, p.Type, required, enumSuffix(p.Enum))
		}
	}
	if view.BodyRequired || len(view.BodyContentTypes) > 0 {
		fmt.Fprintf(w, "请求体: %s\n", strings.Join(view.BodyContentTypes, ", "))
	}
	if len(view.Responses) > 0 {
		fmt.Fprintf(w, "响应码: %s\n", strings.Join(view.Responses, ", "))
	}
	for _, warning := range view.Warnings {
		fmt.Fprintf(w, "⚠ %s\n", warning)
	}
	return nil
}

func enumSuffix(enum []string) string {
	if len(enum) == 0 {
		return ""
	}
	return " 可选值: " + strings.Join(enum, "|")
}
