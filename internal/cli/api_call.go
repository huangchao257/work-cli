package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/api"
	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/output"
	"github.com/huangchao257/work-cli/internal/usage"
)

var (
	apiCallParams []string
	apiCallSet    []string
	apiCallData   string
	apiCallHeader []string
	apiCallYes    bool
)

var apiCallCmd = &cobra.Command{
	Use:   "call <system> <operation>",
	Short: "通用调用兜底（operationId 或 METHOD PATH）",
	Long: `L3 通用调用：按 operationId 或 METHOD PATH 直接调用系统接口。

参数：
  --params <JSON>     query 参数对象（与 --set/--param 合并，显式 flag 优先）
  --set k=v           单个参数（k 为 query.name / header.name 或裸 name=query）
  --data <json|-|@f>  请求体 JSON（- 读 stdin，@f 读相对当前目录的文件）
  --header k=v        附加自定义 header
  --yes               跳过写操作确认（非交互环境必须）

METHOD PATH 支持两种形式：引号整体传入（"GET /pets"）或拆成两个位置参数（GET /pets）。
示例：
  work api call demo "GET /pets" --params '{"limit":2}'
  work api call demo createPet --data '{"name":"rex"}' --yes`,
	Example: `  work api call demo "GET /pets" --params '{"limit":2}' --json
  work api call demo createPet --data '{"name":"rex"}' --dry-run`,
	Args:          cobra.RangeArgs(2, 3),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		operation := args[1]
		if len(args) == 3 {
			// work api call <system> <METHOD> <PATH>
			operation = strings.ToUpper(args[1]) + " " + args[2]
		}
		return runAPICall(cmd, args[0], operation)
	},
}

func init() {
	apiCallCmd.Flags().StringArrayVar(&apiCallParams, "params", nil, "query 参数 JSON 对象（可重复，后者覆盖前者）")
	apiCallCmd.Flags().StringArrayVar(&apiCallSet, "set", nil, "单个参数 k=v（k 可带 query./header. 前缀）")
	apiCallCmd.Flags().StringVar(&apiCallData, "data", "", "请求体 JSON（- stdin，@file 相对路径）")
	apiCallCmd.Flags().StringArrayVar(&apiCallHeader, "header", nil, "附加 header k=v")
	apiCallCmd.Flags().BoolVar(&apiCallYes, "yes", false, "跳过写操作确认")
}

// mergeCallParams 合并 --params JSON 与 --set 键值对，显式 --set 优先。
// 值只接受标量：数组/对象/null 无法经 map[string]string 正确序列化，
// 静默 fmt.Sprint 会产生 "[a b]" 之类的错误数据，直接报参数错误（usage，退出码 2）。
// 数字用 json.Number 解码保留原始字面量——经 any 解码成 float64 后
// fmt.Sprint 会把大整数变成科学计数法（1234567890123456789 → 1.2345678901234568e+18），
// ID 类参数被静默损坏。
func mergeCallParams(params []string, sets []string) (map[string]string, error) {
	merged := map[string]string{}
	for _, raw := range params {
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.UseNumber()
		var decoded map[string]any
		if err := dec.Decode(&decoded); err != nil {
			return nil, usage.Newf("--params 不是合法 JSON 对象: %v", err)
		}
		keys := make([]string, 0, len(decoded))
		for key := range decoded {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := decoded[key]
			switch value.(type) {
			case []any, map[string]any, nil:
				return nil, usage.Newf("--params 的 %q 值必须是标量（字符串/数字/布尔）；数组/对象参数请使用 --set 多次传键", key)
			}
			if num, ok := value.(json.Number); ok {
				merged[key] = num.String()
				continue
			}
			merged[key] = fmt.Sprint(value)
		}
	}
	for _, raw := range sets {
		key, value, found := strings.Cut(raw, "=")
		if !found || strings.TrimSpace(key) == "" {
			return nil, usage.Newf("--set 格式应为 k=v，收到 %q", raw)
		}
		merged[strings.TrimSpace(key)] = value
	}
	return merged, nil
}

// readCallData 解析 --data：inline JSON、-（stdin）或 @相对路径。
// 值必须是合法 JSON（对齐设计文档「只做 required 参数与请求体 JSON 语法校验」）。
// 格式/路径错误为 usage（2）；IO 失败（stdin/文件不可读）为环境类。
func readCallData(spec string) (string, error) {
	spec = strings.TrimSpace(spec)
	var data string
	switch {
	case spec == "":
		return "", nil
	case spec == "-":
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", engine.NewEnvError(fmt.Sprintf("读取 stdin 失败: %v", err))
		}
		data = string(raw)
	case strings.HasPrefix(spec, "@"):
		path := strings.TrimPrefix(spec, "@")
		if path == "" {
			return "", usage.New("--data @file 需要文件名")
		}
		if filepathIsAbs(path) {
			return "", usage.New("--data 只接受当前目录下的相对路径")
		}
		if err := validateRelativePath(path); err != nil {
			return "", err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			// IO 失败（文件不可读/被占用）是环境问题，非参数问题 → 退出码 3
			return "", engine.NewEnvError(fmt.Sprintf("读取请求体文件失败: %v", err))
		}
		data = string(raw)
	default:
		data = spec
	}
	if data != "" {
		var probe any
		if err := json.Unmarshal([]byte(data), &probe); err != nil {
			return "", usage.Newf("--data 不是合法 JSON: %v", err)
		}
	}
	return data, nil
}

// validateRelativePath 拒绝 .. 上跳段：--data @file 只允许当前目录子树内文件。
func validateRelativePath(path string) error {
	cleaned := filepath.Clean(path)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return usage.Newf("--data 只接受当前目录下的相对路径（不允许 ..）: %s", path)
	}
	return nil
}

func parseCallHeaders(headers []string) (map[string]string, error) {
	out := map[string]string{}
	for _, raw := range headers {
		key, value, found := strings.Cut(raw, ":")
		if !found {
			key, value, found = strings.Cut(raw, "=")
		}
		if !found || strings.TrimSpace(key) == "" {
			return nil, usage.Newf("--header 格式应为 k=v，收到 %q", raw)
		}
		out[strings.TrimSpace(key)] = value
	}
	return out, nil
}

// runAPICall 是 L3 调用入口（也被动态命令复用）。
func runAPICall(cmd *cobra.Command, systemName, operation string) error {
	deps := defaultAPIDeps()
	s, cfg, err := deps.findSystem(systemName)
	if err != nil {
		return apiFail(cmd, err)
	}
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
	// 包级 flag 变量读后即清，防同进程多次 Execute 时 --yes/--data 残留
	// 绕过后续调用的确认门禁（pflag 不重置未出现的 flag）
	resetCallFlagState()

	opts := api.CallOptions{
		System: systemName, Operation: operation,
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

// resetCallFlagState 清零调用类包级 flag 变量（读取后调用）。
func resetCallFlagState() {
	apiCallYes = false
	apiCallData = ""
	apiCallParams = nil
	apiCallSet = nil
	apiCallHeader = nil
}

// confirmTTY 构造交互式确认函数；非 TTY 返回 nil（fail-closed 由 Call 层兜底）。
// 确认提示写 stderr：--json 模式下 stdout 的 JSON 契约不能被交互文本污染。
func confirmTTY(cmd *cobra.Command) func(summary api.ConfirmSummary) (bool, error) {
	return func(summary api.ConfirmSummary) (bool, error) {
		stat, err := os.Stdin.Stat()
		if err != nil || stat.Mode()&os.ModeCharDevice == 0 {
			return false, nil
		}
		w := cmd.ErrOrStderr()
		fmt.Fprintf(w, "即将执行 %s %s（%s，风险 %s）\n", summary.Method, summary.Path, summary.Operation, summary.Risk)
		if summary.Body != "" {
			fmt.Fprintf(w, "请求体: %s\n", summary.Body)
		}
		fmt.Fprintf(w, "确认执行？[y/N] ")
		var answer string
		if _, err := fmt.Scanln(&answer); err != nil {
			return false, nil
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		return answer == "y" || answer == "yes", nil
	}
}

// apiErrorToExit 将调用错误映射为退出码并输出错误信封。
// 分类优先级：usage（→2）> environment（→3）> 其余（→1）。
func apiErrorToExit(cmd *cobra.Command, err error) error {
	return apiFail(cmd, err)
}

// isEnvironmentError 按 CallErrorInfo.Type 分类判断环境错误，而非脆弱的消息子串匹配。
func isEnvironmentError(err error) bool {
	var info *api.CallErrorInfo
	if errors.As(err, &info) {
		return info.Type == "environment"
	}
	// 凭据缺失等默认鉴权器错误（无结构化类型）按消息兜底
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "环境变量") || strings.Contains(msg, "export ")
}

// printAPIError 输出统一错误信封（JSON 上 stderr，human 上 stderr）。
// 透传 CallErrorInfo / ConfirmationRequiredError 的结构化分类与 hint。
func printAPIError(cmd *cobra.Command, err error) {
	w := cmd.ErrOrStderr()
	if asJSON {
		errType, subtype, hint := errorEnvelopeFields(err)
		payload := map[string]any{
			"ok": false,
			"error": map[string]any{
				"type":    errType,
				"message": err.Error(),
			},
		}
		if subtype != "" {
			payload["error"].(map[string]any)["subtype"] = subtype
		}
		if hint != "" {
			payload["error"].(map[string]any)["hint"] = hint
		}
		_ = output.PrintJSON(w, payload)
		return
	}
	fmt.Fprintf(w, "✗ %s\n", err.Error())
	if _, _, hint := errorEnvelopeFields(err); hint != "" {
		fmt.Fprintf(w, "  下一步: %s\n", hint)
	}
}

// errorEnvelopeFields 从错误链提取 (type, subtype, hint)。
func errorEnvelopeFields(err error) (string, string, string) {
	var info *api.CallErrorInfo
	if errors.As(err, &info) {
		if info.Hint != "" {
			return info.Type, info.Subtype, info.Hint
		}
		return info.Type, info.Subtype, ""
	}
	var confirmErr *api.ConfirmationRequiredError
	if errors.As(err, &confirmErr) {
		return "cli", "confirmation_required", confirmErr.Hint
	}
	return "cli", "", ""
}

// renderCallResult 输出调用结果信封。HTTP 非 2xx 输出完整信封后返回退出码 1。
func renderCallResult(cmd *cobra.Command, result *api.CallResult) error {
	if asJSON {
		if err := output.PrintJSON(cmd.OutOrStdout(), result); err != nil {
			return err
		}
	} else {
		w := cmd.OutOrStdout()
		status := fmt.Sprintf("%d", result.Status)
		if result.DryRun {
			status = "dry-run"
		}
		mark := "✓"
		if result.Error != nil {
			mark = "✗"
		}
		fmt.Fprintf(w, "%s %s %s（%s，%dms）\n", mark, result.Method, result.Path, status, result.DurationMs)
		for _, warning := range result.Warnings {
			fmt.Fprintf(w, "⚠ %s\n", warning)
		}
		if result.DryRun {
			if invocation, ok := result.Data.(map[string]any); ok {
				fmt.Fprintf(w, "  URL: %s\n", invocation["url"])
				if body, ok := invocation["body"].(string); ok && strings.TrimSpace(body) != "" {
					fmt.Fprintf(w, "  Body: %s\n", body)
				}
			}
			return nil
		}
	}
	if result.Error != nil {
		if !asJSON {
			fmt.Fprintf(cmd.ErrOrStderr(), "✗ %s（%s）\n", result.Error.Message, result.Error.Subtype)
		}
		return exitErr(1, fmt.Errorf("%s", result.Error.Message))
	}
	if !asJSON {
		printHumanData(cmd.OutOrStdout(), result.Data)
	}
	return nil
}

func printHumanData(w io.Writer, data any) {
	switch value := data.(type) {
	case nil:
		return
	case string:
		fmt.Fprintln(w, value)
	case []any:
		for _, item := range value {
			fmt.Fprintln(w, item)
		}
	default:
		if encoded, err := json.MarshalIndent(value, "", "  "); err == nil {
			fmt.Fprintln(w, string(encoded))
		}
	}
}

func filepathIsAbs(path string) bool {
	return filepath.IsAbs(path) || filepathIsWindowsAbs(path)
}

func filepathIsWindowsAbs(path string) bool {
	if len(path) < 3 {
		return false
	}
	return path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}
