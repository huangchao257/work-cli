package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/huangchao257/work-cli/internal/openapi"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/usage"
)

// defaultAuthenticator 实现 none/bearer/apikey 鉴权。
type defaultAuthenticator struct {
	cfg AuthConfig
}

// NewDefaultAuthenticator 用 AuthConfig 构造默认鉴权器。
func NewDefaultAuthenticator(cfg AuthConfig) Authenticator { return &defaultAuthenticator{cfg: cfg} }

func (a *defaultAuthenticator) Authenticate(ctx context.Context, req *http.Request) error {
	switch a.cfg.Kind {
	case "", AuthNone:
		return nil
	case AuthBearer:
		token, err := a.credential()
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	case AuthAPIKey:
		key, err := a.credential()
		if err != nil {
			return err
		}
		if q := a.cfg.Query; q != "" {
			query := req.URL.Query()
			query.Set(q, key)
			req.URL.RawQuery = query.Encode()
			return nil
		}
		header := a.cfg.Header
		if header == "" {
			header = "X-API-Key"
		}
		req.Header.Set(header, key)
		return nil
	default:
		return usage.Newf("不支持的鉴权类型 %q（支持 none/bearer/apikey）", a.cfg.Kind)
	}
}

func (a *defaultAuthenticator) credential() (string, error) {
	name := a.cfg.CredentialEnv
	if name == "" {
		return "", fmt.Errorf("系统配置缺少 credential_env，请运行 work api info 查看配置")
	}
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		hint := platform.EnvSetHint(name)
		if hint == "" {
			hint = fmt.Sprintf("请设置环境变量 %s", name)
		}
		return "", &CallErrorInfo{
			Type: "environment", Subtype: "missing_credential",
			Message: fmt.Sprintf("缺少凭据环境变量 %s", name), Hint: hint,
		}
	}
	return value, nil
}

func (a *defaultAuthenticator) AuthStatus() string {
	switch a.cfg.Kind {
	case "", AuthNone:
		return "none"
	case AuthBearer:
		return "bearer: " + a.credentialStatus()
	case AuthAPIKey:
		label := a.cfg.Header
		if label == "" {
			label = a.cfg.Query
		}
		if label == "" {
			label = "X-API-Key"
		}
		return fmt.Sprintf("apikey(%s): %s", label, a.credentialStatus())
	default:
		return fmt.Sprintf("未知类型 %q", a.cfg.Kind)
	}
}

func (a *defaultAuthenticator) credentialStatus() string {
	if a.cfg.CredentialEnv == "" {
		return "[未设置 credential_env]"
	}
	if strings.TrimSpace(os.Getenv(a.cfg.CredentialEnv)) == "" {
		return "[未设置]"
	}
	return "[已设置]"
}

// AuthorizeRequest 为请求附加凭据。系统实现了 Authenticator 用之，否则退回配置驱动。
func AuthorizeRequest(ctx context.Context, s System, authCfg AuthConfig, req *http.Request) error {
	if a, ok := s.(Authenticator); ok {
		return a.Authenticate(ctx, req)
	}
	return NewDefaultAuthenticator(authCfg).Authenticate(ctx, req)
}

// CredentialSites 报告鉴权层实际注入凭据的位置（header 名/query 参数名），
// 供 dry-run 脱敏使用——凭据遮蔽必须按注入点，不能按名字子串猜测。
// 自定义 Authenticator 未上报注入点时返回 shouldRedactAll=true：
// 无法确定凭据落点时对 dry-run 输出保守遮蔽自定义 header/query（fail-closed）。
func CredentialSites(s System, authCfg AuthConfig) (headerName, queryName string, shouldRedactAll bool) {
	if a, ok := s.(Authenticator); ok {
		if reporter, ok := a.(credentialSiteReporter); ok {
			headerName, queryName = reporter.credentialSites()
			return headerName, queryName, false
		}
		return "", "", true
	}
	switch authCfg.Kind {
	case AuthAPIKey:
		if authCfg.Query != "" {
			return "", authCfg.Query, false
		}
		header := authCfg.Header
		if header == "" {
			header = "X-API-Key"
		}
		return header, "", false
	case AuthBearer:
		return "Authorization", "", false
	default:
		return "", "", false
	}
}

// credentialSiteReporter 是自定义 Authenticator 可选实现的凭据位置上报。
type credentialSiteReporter interface {
	credentialSites() (headerName, queryName string)
}

// AuthStatusText 返回系统鉴权的脱敏状态。
func AuthStatusText(s System, authCfg AuthConfig) string {
	if a, ok := s.(Authenticator); ok {
		return a.AuthStatus()
	}
	return NewDefaultAuthenticator(authCfg).AuthStatus()
}

// resolveHTTPClient 组装 HTTP 客户端：系统可提供自定义 RoundTripper（demo mock 等）。
// 重定向策略：拒绝跨 host 与协议降级——带凭据的 API 调用被 302 到其他 host
// 几乎必然是配置错误或 SSRF 向量（Go 默认仅对非子域跨域剥离 Authorization，
// 兄弟子域/同 host 跨端口仍会原样携带；307/308 还会重发完整请求体）。
func resolveHTTPClient(s System, timeout string) (*http.Client, error) {
	transport := http.DefaultTransport
	if provider, ok := s.(TransportProvider); ok {
		if rt := provider.Transport(); rt != nil {
			transport = rt
		}
	}
	dur, err := parseTimeout(timeout)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: transport, Timeout: dur,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			first := via[0]
			if req.URL.Scheme != first.URL.Scheme {
				return fmt.Errorf("拒绝重定向变更协议: %s → %s", first.URL.Scheme, req.URL.Scheme)
			}
			if req.URL.Host != first.URL.Host {
				return fmt.Errorf("拒绝跨 host 重定向: %s → %s", first.URL.Host, req.URL.Host)
			}
			return nil
		},
	}, nil
}

// writeSystemSpec 将规范快照与 catalog 原子写入系统目录。
func writeSystemSpec(name string, spec []byte, specFile string, catalog *openapi.Catalog) error {
	dir, err := systemDir(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("创建系统目录失败: %w", err)
	}
	if err := atomicWriteFile(filepath.Join(dir, specFile), spec, 0o644); err != nil {
		return fmt.Errorf("写入规范快照失败: %w", err)
	}
	catalogData, err := jsonMarshal(catalog)
	if err != nil {
		return fmt.Errorf("编码 catalog.json 失败: %w", err)
	}
	if err := atomicWriteFile(filepath.Join(dir, "catalog.json"), append(catalogData, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入 catalog.json 失败: %w", err)
	}
	return nil
}

// readSystemSpecText 读取规范快照文本（schema 展示用）。
func readSystemSpecText(name, specFile string) (string, error) {
	dir, err := systemDir(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, specFile))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
