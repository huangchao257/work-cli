package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/huangchao257/work-cli/internal/openapi"
	"github.com/huangchao257/work-cli/internal/usage"
)

// CallOptions 是一次调用的完整输入。
type CallOptions struct {
	System     string
	Operation  string            // operationId 或 METHOD PATH
	Params     map[string]string // --set location.name=value 与 --params 合并后的最终值
	Headers    map[string]string // 自定义 header
	Body       string            // --data 原始 JSON（已读取）
	Yes        bool
	DryRun     bool
	Confirm    func(summary ConfirmSummary) (bool, error)
	AuthConfig AuthConfig
	Timeout    string
	Client     *http.Client // 测试注入；生产为空
}

// ConfirmSummary 是确认提示展示的调用摘要。
type ConfirmSummary struct {
	System    string
	Operation string
	Method    string
	Path      string
	Risk      RiskLevel
	Body      string
}

// CallResult 是调用结果信封。
type CallResult struct {
	OK          bool           `json:"ok"`
	System      string         `json:"system"`
	Operation   string         `json:"operation"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Status      int            `json:"status"`
	ContentType string         `json:"content_type,omitempty"`
	DurationMs  int64          `json:"duration_ms"`
	DryRun      bool           `json:"dry_run,omitempty"`
	Warnings    []string       `json:"warnings,omitempty"`
	Data        any            `json:"data,omitempty"`
	Error       *CallErrorInfo `json:"error,omitempty"`
}

// CallErrorInfo 是失败时的结构化错误。
type CallErrorInfo struct {
	Type      string         `json:"type"`
	Subtype   string         `json:"subtype,omitempty"`
	Message   string         `json:"message"`
	Hint      string         `json:"hint,omitempty"`
	Retryable bool           `json:"retryable,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

// resolveOperation 解析 operation 引用（operationId 或 METHOD PATH）。
func resolveOperation(catalog *openapi.Catalog, ref string) (*openapi.CatalogOperation, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, usage.Newf("operation 不能为空")
	}
	parts := strings.Fields(ref)
	if len(parts) == 2 {
		method, path := strings.ToUpper(parts[0]), parts[1]
		if isHTTPMethod(method) {
			if op, ok := catalog.FindByHTTP(method, path); ok {
				return op, nil
			}
			return nil, usage.Newf("目录中不存在 %s %s，运行 work api schema 查看可用操作", method, path)
		}
	}
	if op, ok := catalog.FindByID(ref); ok {
		return op, nil
	}
	return nil, usage.Newf("未找到 operation %q，运行 work api schema 查看可用操作", ref)
}

func isHTTPMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE":
		return true
	}
	return false
}

// buildRequest 由 operation 与参数构造 HTTP 请求（未鉴权）。
func buildRequest(baseURL string, op *openapi.CatalogOperation, params map[string]string, body string, headers map[string]string) (*http.Request, error) {
	pathTemplate := op.Path
	placed := map[string]bool{}
	var missing []string

	// 先计算每个声明参数实际命中的来源键（前缀键优先，裸名兜底），
	// 保证 placed 与 params 键空间一致：命中的裸名键不再泄漏到 query。
	// 同一参数的前缀键与裸名键并存时，两者都标记为已放置（前缀键优先，裸名被忽略）。
	sourceKey := func(p openapi.CatalogParameter) (string, bool) {
		if _, ok := params[p.In+"."+p.Name]; ok {
			return p.In + "." + p.Name, true
		}
		if _, ok := params[p.Name]; ok {
			return p.Name, true
		}
		return "", false
	}
	// markPlaced 将命中的来源键与其同位置别名键（裸名/前缀形式）一并标记，
	// 防止未命中的别名落入"未声明键"循环覆盖已放置的值。
	markPlaced := func(p openapi.CatalogParameter, key string) {
		placed[key] = true
		placed[p.Name] = true
		placed[p.In+"."+p.Name] = true
	}

	// path 参数替换
	for _, p := range op.Parameters {
		if p.In != "path" {
			continue
		}
		key, ok := sourceKey(p)
		if !ok {
			if p.Required {
				missing = append(missing, p.Name)
			}
			continue
		}
		markPlaced(p, key)
		value := params[key]
		if value == "." || value == ".." || strings.Contains(value, "/../") || strings.HasPrefix(value, "../") || strings.HasSuffix(value, "/..") {
			return nil, usage.Newf("path 参数 %s 的值 %q 含路径穿越段，已拒绝", p.Name, value)
		}
		pathTemplate = strings.ReplaceAll(pathTemplate, "{"+p.Name+"}", url.PathEscape(value))
	}
	if len(missing) > 0 {
		return nil, usage.Newf("缺少必填参数: %s", strings.Join(missing, ", "))
	}

	query := url.Values{}
	header := http.Header{}
	for _, p := range op.Parameters {
		switch p.In {
		case "path":
			continue
		case "query":
			if key, ok := sourceKey(p); ok {
				query.Set(p.Name, params[key])
				markPlaced(p, key)
			} else if p.Required {
				return nil, usage.Newf("缺少必填参数: %s", p.Name)
			}
		case "header":
			if key, ok := sourceKey(p); ok {
				header.Set(p.Name, params[key])
				markPlaced(p, key)
			} else if p.Required {
				return nil, usage.Newf("缺少必填参数: %s", p.Name)
			}
		case "cookie":
			if key, ok := sourceKey(p); ok {
				header.Add("Cookie", p.Name+"="+params[key])
				markPlaced(p, key)
			} else if p.Required {
				return nil, usage.Newf("缺少必填参数: %s", p.Name)
			}
		}
	}
	// 显式参数中目录未声明的键：按位置前缀分发，无前缀默认 query。
	// 先按 key 排序并跳过与前缀键同名的裸名键，保证结果与 map 迭代顺序无关。
	undeclared := make([]string, 0, len(params))
	for key := range params {
		if placed[key] {
			continue
		}
		undeclared = append(undeclared, key)
	}
	sort.Strings(undeclared)
	for _, key := range undeclared {
		value := params[key]
		switch {
		case strings.HasPrefix(key, "query."):
			query.Set(strings.TrimPrefix(key, "query."), value)
		case strings.HasPrefix(key, "header."):
			if isRestrictedHeader(strings.TrimPrefix(key, "header.")) {
				return nil, usage.Newf("不允许通过参数 %s 覆盖受限 header", key)
			}
			header.Set(strings.TrimPrefix(key, "header."), value)
		case strings.HasPrefix(key, "path."):
			return nil, usage.Newf("参数 %s 不是该 operation 声明的 path 参数", key)
		}
	}
	// 裸名键最后处理：若同名前缀键（query./header.）已写入则跳过，避免非确定覆盖
	for _, key := range undeclared {
		if strings.Contains(key, ".") || placed[key] {
			continue
		}
		value := params[key]
		if _, exists := params["query."+key]; exists {
			continue
		}
		if _, exists := params["header."+key]; exists {
			continue
		}
		query.Set(key, value)
	}
	for key, value := range headers {
		if isRestrictedHeader(key) {
			return nil, usage.Newf("不允许通过 --header 覆盖受限 header: %s", key)
		}
		if err := validateHeaderValue(value); err != nil {
			return nil, usage.Newf("header %s 的值非法: %v", key, err)
		}
		header.Set(key, value)
	}

	var payload io.Reader
	contentType := ""
	if strings.TrimSpace(body) != "" {
		payload = strings.NewReader(body)
		contentType = pickContentType(op)
	}
	requestURL := strings.TrimSuffix(baseURL, "/") + pathTemplate
	if encoded := query.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}
	req, err := http.NewRequest(op.Method, requestURL, payload)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	for key, values := range header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

func pickContentType(op *openapi.CatalogOperation) string {
	for _, contentType := range op.BodyContentTypes {
		if strings.Contains(contentType, "json") {
			return contentType
		}
	}
	if len(op.BodyContentTypes) > 0 {
		return op.BodyContentTypes[0]
	}
	return "application/json"
}

var restrictedHeaders = map[string]bool{
	"host": true, "content-length": true, "connection": true, "transfer-encoding": true,
	"authorization": true, "cookie": true, "content-type": true,
}

func isRestrictedHeader(key string) bool {
	return restrictedHeaders[strings.ToLower(strings.TrimSpace(key))]
}

// validateHeaderValue 预检 header 值（拒绝 CRLF 注入），非法即参数错误而非环境错误。
func validateHeaderValue(value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("含换行符（禁止 header 注入）")
	}
	return nil
}

// Call 执行一次系统调用：解析 operation → 构造请求 → 鉴权 → 风险门禁 → 发送。
func Call(ctx context.Context, s System, opts CallOptions) (*CallResult, error) {
	catalog, err := s.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	op, err := resolveOperation(catalog, opts.Operation)
	if err != nil {
		return nil, err
	}
	risk := AssessRisk(op)
	baseURL := s.BaseURL()
	if baseURL == "" {
		return nil, usage.Newf("系统 %s 未配置 base_url，无法发起远程调用", opts.System)
	}

	// dry-run：构造请求但不发送、不确认
	if opts.DryRun {
		req, err := buildRequest(baseURL, op, opts.Params, opts.Body, opts.Headers)
		if err != nil {
			return nil, err
		}
		if err := AuthorizeRequest(ctx, s, opts.AuthConfig, req); err != nil {
			return nil, err
		}
		headerSite, querySite, redactAll := CredentialSites(s, opts.AuthConfig)
		return &CallResult{
			OK: true, System: opts.System, Operation: displayOperation(op), Method: op.Method,
			Path: req.URL.Path, DryRun: true, Warnings: op.Warnings,
			Data: map[string]any{
				"method":  req.Method,
				"url":     redactURL(req, querySite, redactAll),
				"headers": redactHeaders(req, headerSite, redactAll),
				"body":    opts.Body,
			},
		}, nil
	}

	// 风险门禁
	if Confirmable(risk) && !opts.Yes {
		confirmed := false
		if opts.Confirm != nil {
			confirmed, err = opts.Confirm(ConfirmSummary{
				System: opts.System, Operation: displayOperation(op), Method: op.Method,
				Path: op.Path, Risk: risk, Body: opts.Body,
			})
			if err != nil {
				return nil, err
			}
		}
		if !confirmed {
			return nil, NewConfirmationRequired(opts.System, displayOperation(op), op.Method, op.Path, risk, "非交互环境且未提供 --yes")
		}
	}

	req, err := buildRequest(baseURL, op, opts.Params, opts.Body, opts.Headers)
	if err != nil {
		return nil, err
	}
	if err := AuthorizeRequest(ctx, s, opts.AuthConfig, req); err != nil {
		return nil, err
	}

	client := opts.Client
	if client == nil {
		client, err = resolveHTTPClient(s, opts.Timeout)
		if err != nil {
			return nil, err
		}
	}
	start := time.Now()
	resp, err := client.Do(req.WithContext(ctx))
	duration := time.Since(start).Milliseconds()
	if err != nil {
		return nil, networkError(err)
	}
	defer resp.Body.Close()

	result := &CallResult{
		System: opts.System, Operation: displayOperation(op), Method: op.Method,
		Path: op.Path, Status: resp.StatusCode, ContentType: resp.Header.Get("Content-Type"),
		DurationMs: duration, Warnings: op.Warnings,
	}
	bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if readErr != nil {
		result.Error = &CallErrorInfo{Type: "transport", Message: "读取响应失败: " + readErr.Error()}
		return result, nil
	}
	truncated := len(bodyBytes) > maxResponseBody
	if truncated {
		bodyBytes = bodyBytes[:maxResponseBody]
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("响应超过 %d MiB 上限已截断，完整数据请改用服务端分页/导出接口", maxResponseBody>>20))
	}
	result.OK = resp.StatusCode >= 200 && resp.StatusCode < 300
	result.Data = decodeBody(result.ContentType, bodyBytes)
	if !result.OK {
		result.Error = &CallErrorInfo{
			Type: "api", Subtype: fmt.Sprintf("http_%d", resp.StatusCode),
			Message: fmt.Sprintf("系统返回 %d", resp.StatusCode),
			Hint:    "检查参数与权限后重试",
			Details: map[string]any{"body": decodeBody(result.ContentType, bodyBytes)},
		}
	}
	return result, nil
}

const maxResponseBody = 4 << 20

// maxRawDataChars 是 decodeBody 失败退回原文时保留的最大字符数（约 2 KiB）。
// 防止截断的 JSON 解析失败后把数 MiB 原文整段塞进信封，淹没下游 Agent 上下文。
const maxRawDataChars = 2048

func displayOperation(op *openapi.CatalogOperation) string {
	if op.ID != "" {
		return op.ID
	}
	return op.Method + " " + op.Path
}

func decodeBody(contentType string, body []byte) any {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil
	}
	if strings.Contains(contentType, "json") {
		var decoded any
		if err := jsonUnmarshal(body, &decoded); err == nil {
			return decoded
		}
	}
	if len(trimmed) > maxRawDataChars {
		return trimmed[:maxRawDataChars] + "…（超长原文已截断）"
	}
	return trimmed
}

func (e *CallErrorInfo) Error() string { return e.Message }

func networkError(err error) error {
	return &CallErrorInfo{Type: "environment", Message: "请求失败: " + err.Error(), Hint: "检查网络与 base_url 配置", Retryable: true}
}

// redactURL 遮蔽 query 中的凭据。querySite 是鉴权层注入的 query 参数名（强制遮蔽），
// redactAll 时全部 query 值遮蔽（自定义鉴权未上报注入点，无法确定凭据落点）；
// 名字启发式仅作非注入点兜底（如用户显式 --set query.api_token=...），词表与 redactHeaders 一致。
func redactURL(req *http.Request, querySite string, redactAll bool) string {
	u := *req.URL
	if u.RawQuery == "" {
		return u.String()
	}
	values := u.Query()
	for key := range values {
		if redactAll || key == querySite || containsCredentialMarker(key) {
			values.Set(key, "***")
		}
	}
	u.RawQuery = values.Encode()
	return u.String()
}

// containsCredentialMarker 是脱敏兜底的名字启发式词表（headers/query 共用）。
func containsCredentialMarker(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "key") || strings.Contains(lower, "token")
}

// redactHeaders 遮蔽 header 中的凭据。headerSite 是鉴权层注入的 header 名（强制遮蔽），
// redactAll 时除 Content-Type 等基本 header 外全部遮蔽；名字启发式仅作非注入点兜底。
func redactHeaders(req *http.Request, headerSite string, redactAll bool) map[string]string {
	out := map[string]string{}
	for key, values := range req.Header {
		if redactAll {
			out[key] = "***"
			continue
		}
		if strings.EqualFold(key, headerSite) || strings.EqualFold(key, "Authorization") || containsCredentialMarker(key) {
			out[key] = "***"
			continue
		}
		out[key] = strings.Join(values, ", ")
	}
	return out
}
