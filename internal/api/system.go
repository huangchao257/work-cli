// Package api 提供系统接口 CLI 化的注册表、调用编排与安全门禁。
package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/huangchao257/work-cli/internal/openapi"
)

// Manifest 描述一个可调用系统的稳定元数据。
type Manifest struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description"`
	Version     string `json:"version,omitempty" yaml:"version"`
	Source      string `json:"source,omitempty" yaml:"source"` // builtin | imported
	SourceURL   string `json:"source_url,omitempty" yaml:"source_url,omitempty"`
	BaseURL     string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// System 是一个可调用系统的统一视图，内置插件与导入系统都实现它。
type System interface {
	Manifest() Manifest
	// Catalog 返回归一化后的命令目录。导入系统从持久化目录读取；内置系统可从内嵌规范构建。
	Catalog(ctx context.Context) (*openapi.Catalog, error)
	// Document 返回完整 OpenAPI 文档，供 schema 查询使用；可为 nil（仅导入系统提供）。
	Document(ctx context.Context) (*openapi.Document, error)
	// BaseURL 返回调用的根地址，空串表示仅离线系统（如 demo）。
	BaseURL() string
}

// Authenticator 可选实现：向请求附加凭据。默认实现见 auth.go。
type Authenticator interface {
	// Authenticate 基于 system/auth 配置向 req 附加凭据；失败返回环境错误。
	Authenticate(ctx context.Context, req *http.Request) error
	// AuthStatus 返回脱敏后的凭据状态（如 "bearer: [已设置]"）。
	AuthStatus() string
}

// TransportProvider 可选实现：系统自带传输（demo mock、未来的重试/审计传输）。
type TransportProvider interface {
	Transport() http.RoundTripper
}

// Shortcuts 可选实现：返回系统的快捷操作（L1）。
type Shortcuts interface {
	Shortcuts() []Shortcut
}

// Shortcut 是一个意图级快捷操作。自定义 Handler 可编排多个调用；
// Handler 为 nil 时 Target 必须指向目录中的单个 operation。
type Shortcut struct {
	Name        string                                                                                    `json:"name" yaml:"name"`
	Description string                                                                                    `json:"description,omitempty" yaml:"description"`
	Target      string                                                                                    `json:"target,omitempty" yaml:"target"` // operationId
	Params      map[string]string                                                                         `json:"params,omitempty" yaml:"params"`
	Risk        string                                                                                    `json:"risk,omitempty" yaml:"risk"`
	Handler     func(ctx context.Context, s System, call CallFunc, params map[string]string) (any, error) `json:"-" yaml:"-"`
}

// CallFunc 是传给 shortcut handler 的调用函数，便于组合多个 operation。
type CallFunc func(ctx context.Context, opts CallOptions) (*CallResult, error)

// Registry 维护已注册系统。DefaultRegistry 供编译期插件在 init() 注册。
type Registry struct {
	mu       sync.RWMutex
	systems  map[string]System
	frozen   bool
	onFreeze []func()
}

// DefaultRegistry 是进程级注册表。
var DefaultRegistry = NewRegistry()

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{systems: map[string]System{}}
}

// ReservedSystemNames 是不能用作系统名的命令保留字。
var ReservedSystemNames = map[string]bool{
	"list": true, "info": true, "import": true, "refresh": true,
	"remove": true, "schema": true, "call": true, "help": true,
}

// Register 注册系统。名字非法、保留字冲突或重名时返回错误；冻结后拒绝注册。
func (r *Registry) Register(s System) error {
	m := s.Manifest()
	if m.Name == "" {
		return fmt.Errorf("系统名不能为空")
	}
	if ReservedSystemNames[m.Name] {
		return fmt.Errorf("系统名 %q 是保留字", m.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return fmt.Errorf("注册表已冻结，无法注册系统 %q", m.Name)
	}
	if _, exists := r.systems[m.Name]; exists {
		return fmt.Errorf("系统 %q 已注册", m.Name)
	}
	r.systems[m.Name] = s
	return nil
}

// Freeze 冻结注册表并执行回调（如动态命令装配）。重复调用幂等。
func (r *Registry) Freeze() {
	r.mu.Lock()
	if r.frozen {
		r.mu.Unlock()
		return
	}
	r.frozen = true
	callbacks := r.onFreeze
	r.onFreeze = nil
	r.mu.Unlock()
	for _, fn := range callbacks {
		fn()
	}
}

// OnFreeze 注册冻结回调（用于动态命令装配）。已冻结时立即执行。
func (r *Registry) OnFreeze(fn func()) {
	r.mu.Lock()
	if !r.frozen {
		r.onFreeze = append(r.onFreeze, fn)
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()
	fn()
}

// ByName 按名称查找系统。
func (r *Registry) ByName(name string) (System, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.systems[name]
	return s, ok
}

// Names 返回已注册系统名（排序后）。
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.systems))
	for name := range r.systems {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Systems 返回已注册系统（按名排序）。拷贝在单个读锁临界区内完成，
// 避免锁外读 map 与并发 Register 触发不可恢复的 runtime fatal。
func (r *Registry) Systems() []System {
	r.mu.RLock()
	names := make([]string, 0, len(r.systems))
	for name := range r.systems {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]System, 0, len(names))
	for _, name := range names {
		out = append(out, r.systems[name])
	}
	r.mu.RUnlock()
	return out
}
