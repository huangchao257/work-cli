// Package demo 是内嵌的离线示例系统：演示编译期 System 插件、
// mock 传输与自定义快捷方式，可离线验证 work api 全链路。
package demo

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/huangchao257/work-cli/internal/api"
	"github.com/huangchao257/work-cli/internal/openapi"
)

//go:embed openapi.yaml
var specFS embed.FS

const specName = "openapi.yaml"

// System 是 demo 系统实现。
type System struct {
	once     sync.Once
	document *openapi.Document
	catalog  *openapi.Catalog
	loadErr  error
}

// New 构造 demo 系统。
func New() *System { return &System{} }

func (s *System) load() {
	s.once.Do(func() {
		data, err := specFS.ReadFile(specName)
		if err != nil {
			s.loadErr = err
			return
		}
		doc, err := openapi.LoadBytes(data)
		if err != nil {
			s.loadErr = err
			return
		}
		catalog, err := doc.Index()
		if err != nil {
			s.loadErr = err
			return
		}
		s.document = doc
		s.catalog = catalog
	})
}

func (s *System) Manifest() api.Manifest {
	return api.Manifest{
		Name:        "demo",
		Description: "离线示例系统（mock 传输，不访问网络）",
		Version:     "1.0.0",
		Source:      "builtin",
	}
}

func (s *System) Catalog(ctx context.Context) (*openapi.Catalog, error) {
	s.load()
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.catalog, nil
}

func (s *System) Document(ctx context.Context) (*openapi.Document, error) {
	s.load()
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.document, nil
}

// BaseURL 返回占位地址：demo 使用 mock 传输，不真正访问网络。
func (s *System) BaseURL() string { return "https://demo.invalid" }

// Transport 实现 TransportProvider：返回确定性 mock 传输。
func (s *System) Transport() http.RoundTripper { return &mockTransport{} }

// Shortcuts 实现 Shortcuts：提供超越单 operation 别名的组合快捷方式。
func (s *System) Shortcuts() []api.Shortcut {
	return []api.Shortcut{
		{
			Name:        "+seed",
			Description: "创建示例宠物并返回列表（组合 createPet + listPets）",
			Risk:        "write",
			Params:      map[string]string{"name": "demo-pet"},
			Handler: func(ctx context.Context, sys api.System, call api.CallFunc, params map[string]string) (any, error) {
				name := params["name"]
				if strings.TrimSpace(name) == "" {
					name = "demo-pet"
				}
				created, err := call(ctx, api.CallOptions{
					System: "demo", Operation: "createPet", Yes: true,
					Body: fmt.Sprintf(`{"name":%q}`, name),
				})
				if err != nil {
					return nil, err
				}
				listed, err := call(ctx, api.CallOptions{
					System: "demo", Operation: "listPets", Yes: true,
					Params: map[string]string{"query.limit": "5"},
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"created": created.Data,
					"pets":    listed.Data,
				}, nil
			},
		},
		{
			Name:        "+top",
			Description: "列出前 5 个宠物（listPets 预设参数）",
			Target:      "listPets",
			Risk:        "read",
			Params:      map[string]string{"query.limit": "5"},
		},
	}
}

// mockTransport 是确定性离线传输：按 method/path 返回固定响应。
type mockTransport struct{}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var status int
	var body string
	contentType := "application/json"

	switch {
	case req.Method == http.MethodGet && req.URL.Path == "/status":
		status, body = http.StatusOK, `{"status":"ok","system":"demo"}`
	case req.Method == http.MethodGet && req.URL.Path == "/pets":
		limit := req.URL.Query().Get("limit")
		if limit == "" {
			limit = "10"
		}
		status = http.StatusOK
		if req.URL.Query().Get("status") != "" && req.URL.Query().Get("status") != "available" {
			body = fmt.Sprintf(`{"pets":[],"filter":%q,"limit":%q}`, req.URL.Query().Get("status"), limit)
		} else {
			body = fmt.Sprintf(`{"pets":[{"id":"p-1","name":"mochi","tag":"cat"},{"id":"p-2","name":"rex","tag":"dog"}],"limit":%q}`, limit)
		}
	case req.Method == http.MethodPost && req.URL.Path == "/pets":
		status, body = http.StatusCreated, `{"id":"p-new","created":true}`
	case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/pets/"):
		id := strings.TrimPrefix(req.URL.Path, "/pets/")
		if id == "404" {
			status, body = http.StatusNotFound, `{"error":"pet not found"}`
		} else {
			status, body = http.StatusOK, fmt.Sprintf(`{"id":%q,"name":"pet-%s"}`, id, id)
		}
	case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/pets/"):
		id := strings.TrimPrefix(req.URL.Path, "/pets/")
		if id == "404" {
			status, body = http.StatusNotFound, `{"error":"pet not found"}`
		} else {
			status, body = http.StatusNoContent, ``
		}
	case req.Method == http.MethodGet && req.URL.Path == "/error":
		status, body = http.StatusInternalServerError, `{"error":"internal"}`
	default:
		status, body = http.StatusNotFound, `{"error":"no mock for `+req.Method+` `+req.URL.Path+`"}`
	}

	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       ioNopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func init() {
	// 编译期注册到默认注册表；由 internal/cli/api_builtin_demo.go blank-import 触发。
	_ = api.DefaultRegistry.Register(New())
}
