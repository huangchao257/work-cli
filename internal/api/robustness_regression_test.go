package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huangchao257/work-cli/internal/usage"
)

// --- R4-C1: 重定向策略（跨 host 拒绝，凭据不外带） ---

func TestCallRejectsCrossHostRedirect(t *testing.T) {
	isolateHome(t)
	t.Setenv("R4_TOKEN", "secret-token")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 第一跳由测试主动发起（必然到达）；此处发出跨协议跨 host 重定向
		http.Redirect(w, r, "https://evil.invalid.example/x", http.StatusFound)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	s := newSingleOpSystem("redir", t, server.URL)
	_, err := Call(context.Background(), s, CallOptions{
		System: "redir", Operation: "op",
		AuthConfig: AuthConfig{Kind: AuthBearer, CredentialEnv: "R4_TOKEN"},
		Yes:        true,
	})
	if err == nil {
		t.Fatal("cross-host redirect should fail")
	}
	// httptest 是 http://，重定向到 https://evil... 同时触发协议变更与跨 host；
	// 任一拒绝都证明凭据不会被带去新 host（Go 默认行为是直接跟随）。
	if !strings.Contains(err.Error(), "跨 host") && !strings.Contains(err.Error(), "协议") {
		t.Fatalf("error should mention redirect rejection: %v", err)
	}
}

// newSingleOpSystem 构造单 op、走真实网络栈的系统。
func newSingleOpSystem(name string, t *testing.T, baseURL string) System {
	t.Helper()
	spec := fmt.Sprintf(`
openapi: 3.0.3
info: {title: %s, version: "1"}
servers: [{url: %s}]
paths:
  /op:
    get:
      operationId: op
      responses: {"200": {description: ok}}
`, name, baseURL)
	doc, err := openapiLoadBytes(spec)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := doc.Index()
	if err != nil {
		t.Fatal(err)
	}
	return &singleOpSystem{baseURL: baseURL, catalog: catalog}
}

type singleOpSystem struct {
	baseURL string
	catalog *catalogType
}

func (s *singleOpSystem) Manifest() Manifest                                { return Manifest{Name: "redir", Source: "builtin"} }
func (s *singleOpSystem) BaseURL() string                                   { return s.baseURL }
func (s *singleOpSystem) Catalog(ctx context.Context) (*catalogType, error) { return s.catalog, nil }
func (s *singleOpSystem) Document(ctx context.Context) (*docType, error)    { return nil, nil }

// --- R4-C1b: 307 重定向重发请求体（含凭据 body）也应被拦 ---

func TestCallRejects307CrossHostRedirect(t *testing.T) {
	isolateHome(t)
	var reached bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Host, "evil") {
			reached = true
		}
		http.Redirect(w, r, "https://evil.invalid.example/x", http.StatusTemporaryRedirect)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	s := newSingleOpSystem("redir", t, server.URL)
	_, err := Call(context.Background(), s, CallOptions{
		System: "redir", Operation: "op", Body: `{"name":"rex"}`, Yes: true,
	})
	if err == nil || (!strings.Contains(err.Error(), "跨 host") && !strings.Contains(err.Error(), "协议")) {
		t.Fatalf("307 cross-host redirect should be rejected, err=%v", err)
	}
	if reached {
		t.Fatal("request body reached the redirect target host")
	}
}

// --- R4-M3: 非法 x-work-risk fail-closed ---

func TestIllegalWorkRiskFailsClosed(t *testing.T) {
	doc, err := openapiLoadBytes(`
openapi: 3.0.3
info: {title: BadRisk, version: "1"}
servers: [{url: https://example.com}]
paths:
  /zap:
    delete:
      operationId: zapAll
      x-work-risk: dangerou
      responses: {"200": {description: ok}}
`)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := doc.Index()
	if err != nil {
		t.Fatal(err)
	}
	op, ok := catalog.FindByID("zapAll")
	if !ok {
		t.Fatal("zapAll not found")
	}
	if op.Risk != "dangerous" {
		t.Fatalf("illegal x-work-risk should degrade to dangerous, got %q", op.Risk)
	}
	found := false
	for _, w := range op.Warnings {
		if strings.Contains(w, "dangerou") {
			found = true
		}
	}
	if !found {
		t.Fatalf("illegal risk should produce warning: %#v", op.Warnings)
	}
	// 语义验证：门禁强度不得低于 dangerous（AssessRisk 可直接读目录值）
	if level := AssessRisk(op); level != RiskDangerous {
		t.Fatalf("AssessRisk = %v, want dangerous", level)
	}
}

// --- R4-M4: 响应超 4 MiB 截断带 warning 且原文摘录 ---

func TestOversizedResponseTruncatedWithWarning(t *testing.T) {
	isolateHome(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// >4MiB 且整体是合法 JSON 前缀但被截断后必解析失败
		_, _ = w.Write([]byte(`{"pad":"` + strings.Repeat("a", 5<<20) + `"}`))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	s := newSingleOpSystem("redir", t, server.URL)
	result, err := Call(context.Background(), s, CallOptions{System: "redir", Operation: "op"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "截断") {
			found = true
		}
	}
	if !found {
		t.Fatalf("truncation warning missing: %#v", result.Warnings)
	}
	if text, ok := result.Data.(string); ok && len(text) > 8<<10 {
		t.Fatalf("oversized raw text should be excerpted, got %d chars", len(text))
	}
}

// --- R4-M1: catalog.json 二次消费校验（坏条目降级、好条目保留） ---

func TestSanitizeCatalogDegradesBadEntries(t *testing.T) {
	isolateHome(t)
	dir, err := systemDir("tampered")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	bad := `{"title":"T","version":"1","openapi":"3.0.3","operations":[
		{"id":"bad","method":"GET","path":"/a","cli_path":[],"dynamic":true},
		{"id":"good","method":"GET","path":"/b","cli_path":["pets","list"],"dynamic":true},
		{"id":"ws","method":"GET","path":"/c","cli_path":["a b","x"],"dynamic":true}
	]}`
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &SystemConfig{Name: "tampered", SpecFile: "openapi.yaml"}
	s := NewConfigSystem(cfg)
	catalog, err := s.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	badOp, badOk := catalog.FindByID("bad")
	wsOp, wsOk := catalog.FindByID("ws")
	goodOp, goodOk := catalog.FindByID("good")
	if !badOk || !goodOk || !wsOk {
		t.Fatalf("entries missing: bad=%v good=%v ws=%v", badOk, goodOk, wsOk)
	}
	if badOp.Dynamic {
		t.Fatal("empty cli_path entry should be degraded to non-dynamic")
	}
	if wsOp.Dynamic {
		t.Fatal("whitespace-segment cli_path entry should be degraded")
	}
	if !goodOp.Dynamic {
		t.Fatal("good entry should remain dynamic")
	}
	if len(catalog.Warnings) < 2 {
		t.Fatalf("catalog warnings should cover both degraded entries: %#v", catalog.Warnings)
	}
}

// --- R4-M2: 自定义凭据 header 名在 dry-run 中强制脱敏 ---

func TestDryRunRedactsCustomCredentialHeader(t *testing.T) {
	isolateHome(t)
	t.Setenv("R4_TOKEN", "secret-token")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{}`)) })
	server := httptest.NewServer(handler)
	defer server.Close()

	s := newSingleOpSystem("redir", t, server.URL)
	result, err := Call(context.Background(), s, CallOptions{
		System: "redir", Operation: "op", DryRun: true,
		AuthConfig: AuthConfig{Kind: AuthAPIKey, Header: "X-Company-Token", CredentialEnv: "R4_TOKEN"},
	})
	if err != nil {
		t.Fatal(err)
	}
	invocation, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("dry-run data = %#v", result.Data)
	}
	headers := invocation["headers"].(map[string]string)
	if got := headers["X-Company-Token"]; got != "***" {
		t.Fatalf("X-Company-Token should be redacted, got %q", got)
	}
	if strings.Contains(fmt.Sprint(invocation), "secret-token") {
		t.Fatal("credential leaked into dry-run output")
	}
}

// --- R4-M2b: apikey 放 query 时 dry-run URL 强制脱敏 ---

func TestDryRunRedactsCredentialQuery(t *testing.T) {
	isolateHome(t)
	t.Setenv("R4_TOKEN", "secret-token")
	s := newSingleOpSystem("redir", t, "https://redir.invalid")
	result, err := Call(context.Background(), s, CallOptions{
		System: "redir", Operation: "op", DryRun: true,
		AuthConfig: AuthConfig{Kind: AuthAPIKey, Query: "sess", CredentialEnv: "R4_TOKEN"},
	})
	if err != nil {
		t.Fatal(err)
	}
	invocation := result.Data.(map[string]any)
	urlText := invocation["url"].(string)
	if strings.Contains(urlText, "secret-token") {
		t.Fatalf("credential leaked in URL: %s", urlText)
	}
	if !strings.Contains(urlText, "sess=") {
		t.Fatalf("query site missing in URL: %s", urlText)
	}
}

// --- R4-m1: spec_file 路径穿越拒绝 ---

func TestDocumentRejectsTraversalSpecFile(t *testing.T) {
	isolateHome(t)
	cfg := &SystemConfig{Name: "trav", SpecFile: "../../../etc/passwd"}
	s := NewConfigSystem(cfg)
	if _, err := s.Document(context.Background()); err == nil {
		t.Fatal("traversal spec_file should be rejected")
	} else if !strings.Contains(err.Error(), "spec_file") {
		t.Fatalf("error should mention spec_file: %v", err)
	}
}

// --- R6: 手编 catalog 的 cli_path 冲突与导入期同语义消解 ---

func TestSanitizeCatalogResolvesDuplicateCLIPaths(t *testing.T) {
	isolateHome(t)
	dir, err := systemDir("duppath")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	dup := `{"title":"D","version":"1","openapi":"3.0.3","operations":[
		{"id":"first","method":"GET","path":"/a","cli_path":["pets","list"],"dynamic":true},
		{"id":"second","method":"GET","path":"/b","cli_path":["pets","list"],"dynamic":true}
	]}`
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(dup), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewConfigSystem(&SystemConfig{Name: "duppath", SpecFile: "openapi.yaml"})
	catalog, err := s.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first, _ := catalog.FindByID("first")
	second, _ := catalog.FindByID("second")
	if first.Dynamic || second.Dynamic {
		t.Fatalf("duplicate cli_path should degrade both: first=%v second=%v", first.Dynamic, second.Dynamic)
	}
	found := false
	for _, w := range catalog.Warnings {
		if strings.Contains(w, "冲突") {
			found = true
		}
	}
	if !found {
		t.Fatalf("duplicate cli_path should warn: %#v", catalog.Warnings)
	}
}

// --- R6: 重复 operationId warning ---

func TestDuplicateOperationIDWarns(t *testing.T) {
	doc, err := openapiLoadBytes(`
openapi: 3.0.3
info: {title: Dup, version: "1"}
servers: [{url: https://dup.invalid}]
paths:
  /one:
    get:
      operationId: getPet
      responses: {"200": {description: ok}}
  /two:
    get:
      operationId: getPet
      responses: {"200": {description: ok}}
`)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := doc.Index()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range catalog.Warnings {
		if strings.Contains(w, "getPet") && strings.Contains(w, "重复") {
			found = true
		}
	}
	if !found {
		t.Fatalf("duplicate operationId should warn: %#v", catalog.Warnings)
	}
}

// --- R6: Windows 保留设备名拒绝 ---

func TestImportRejectsWindowsReservedNames(t *testing.T) {
	isolateHome(t)
	for _, name := range []string{"con", "NUL", "Aux", "com1", "LPT9"} {
		if err := validateSystemName(name); err == nil {
			t.Fatalf("validateSystemName(%q) should reject Windows reserved name", name)
		}
	}
	for _, name := range []string{"console", "com10", "nulx", "a-con"} {
		if err := validateSystemName(name); err != nil {
			t.Fatalf("validateSystemName(%q) should pass: %v", name, err)
		}
	}
}

// --- R6: 配置型 shortcut 悬空 target 警告 ---

func TestValidateShortcutTargetsWarnsOnDanglingTarget(t *testing.T) {
	doc, err := openapiLoadBytes(`
openapi: 3.0.3
info: {title: S, version: "1"}
servers: [{url: https://s.invalid}]
paths:
  /x:
    get:
      operationId: realOp
      responses: {"200": {description: ok}}
`)
	if err != nil {
		t.Fatal(err)
	}
	catalog, _ := doc.Index()
	cfg := &SystemConfig{Shortcuts: map[string]ShortcutDef{
		"dangling": {Target: "noSuchOp", Risk: "read"},
		"fine":     {Target: "realOp", Risk: "read"},
	}}
	warnings := ValidateShortcutTargets(catalog, cfg)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "noSuchOp") {
		t.Fatalf("warnings = %#v, want single dangling-target warning", warnings)
	}
}

// --- R6-D: 自定义 Authenticator 未上报注入点时 dry-run 保守全遮蔽 ---

func TestDryRunRedactsAllWhenAuthenticatorOpaque(t *testing.T) {
	isolateHome(t)
	spec := `openapi: 3.0.3
info: {title: Opaque, version: "1"}
servers: [{url: https://opaque.invalid}]
paths:
  /op:
    get:
      operationId: op
      responses: {"200": {description: ok}}
`
	doc, err := openapiLoadBytes(spec)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := doc.Index()
	if err != nil {
		t.Fatal(err)
	}
	s := &opaqueAuthSystem{catalog: catalog}
	result, err := Call(context.Background(), s, CallOptions{
		System: "opaque", Operation: "op", DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	invocation, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("dry-run data = %#v", result.Data)
	}
	headers := invocation["headers"].(map[string]string)
	sig, has := headers["X-Sig-Value"]
	if !has || sig != "***" {
		t.Fatalf("opaque authenticator secret should be redacted, got %q (headers=%v)", sig, headers)
	}
	if strings.Contains(fmt.Sprint(invocation), "raw-secret-abc123") {
		t.Fatal("opaque authenticator credential leaked into dry-run output")
	}
}

// opaqueAuthSystem 实现自定义 Authenticator（注入不明位置的自定义 header）但不实现上报接口。
type opaqueAuthSystem struct {
	catalog *catalogType
}

func (s *opaqueAuthSystem) Authenticate(ctx context.Context, req *http.Request) error {
	req.Header.Set("X-Sig-Value", "raw-secret-abc123")
	return nil
}
func (s *opaqueAuthSystem) AuthStatus() string { return "custom: [已设置]" }
func (s *opaqueAuthSystem) Manifest() Manifest {
	return Manifest{Name: "opaque", Source: "builtin"}
}
func (s *opaqueAuthSystem) BaseURL() string                                   { return "https://opaque.invalid" }
func (s *opaqueAuthSystem) Catalog(ctx context.Context) (*catalogType, error) { return s.catalog, nil }
func (s *opaqueAuthSystem) Document(ctx context.Context) (*docType, error)    { return nil, nil }

// --- R4-m7: header 值 CRLF 预检（参数错误而非环境错误） ---

func TestHeaderValueCRLFRejectedAsUsage(t *testing.T) {
	doc, err := openapiLoadBytes(`
openapi: 3.0.3
info: {title: H, version: "1"}
servers: [{url: https://h.invalid}]
paths:
  /h:
    get:
      operationId: h
      responses: {"200": {description: ok}}
`)
	if err != nil {
		t.Fatal(err)
	}
	catalog, _ := doc.Index()
	op, _ := catalog.FindByID("h")
	_, err = buildRequest("https://h.invalid", op, nil, "", map[string]string{"X-Evil": "v\r\nHost: evil"})
	if err == nil {
		t.Fatal("CRLF header value should be rejected")
	}
	var usageErr *usage.Error
	if !errors.As(err, &usageErr) {
		t.Fatalf("CRLF header should be usage error, got: %v", err)
	}
}
