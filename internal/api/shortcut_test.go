package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// demoLikeSystem 用于 shortcut 测试的最小系统。
type shortcutTestSystem struct {
	staticSystem
}

func TestEffectiveShortcutRiskAtLeastUnderlying(t *testing.T) {
	system := &shortcutTestSystem{}
	catalog, err := system.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// createPet 底层是 write；声明 read 的 shortcut 应提升为 write
	sc := Shortcut{Name: "+cheap-create", Target: "createPet", Risk: "read"}
	if got := EffectiveShortcutRisk(catalog, sc); got != RiskWrite {
		t.Fatalf("risk = %v, want write", got)
	}
	// 未声明风险视同 read，但底层 write 仍提升
	sc = Shortcut{Name: "+create", Target: "createPet"}
	if got := EffectiveShortcutRisk(catalog, sc); got != RiskWrite {
		t.Fatalf("risk = %v, want write", got)
	}
	// handler 型 shortcut 只信声明；空声明视为 dangerous
	sc = Shortcut{Name: "+custom", Handler: func(ctx context.Context, s System, call CallFunc, params map[string]string) (any, error) {
		return nil, nil
	}}
	if got := EffectiveShortcutRisk(catalog, sc); got != RiskDangerous {
		t.Fatalf("handler risk = %v, want dangerous", got)
	}
}

func TestBuildShortcutsMergesSystemAndConfig(t *testing.T) {
	cfg := &SystemConfig{Shortcuts: map[string]ShortcutDef{
		"cfg-only": {Target: "listPets", Risk: "read"},
	}}
	system := &shortcutTestSystem{}
	shortcuts, err := BuildShortcuts(system, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(shortcuts) != 1 || shortcuts[0].Name != "+cfg-only" {
		t.Fatalf("shortcuts = %#v", shortcuts)
	}
}

// TestBuildShortcutsSystemOverridesConfigSameName 验证编译期 shortcut 与配置型同名时
// 内置优先（system 侧先注册占坑，config 侧被忽略并出 warning）。
func TestBuildShortcutsSystemOverridesConfigSameName(t *testing.T) {
	cfg := &SystemConfig{Shortcuts: map[string]ShortcutDef{
		"seed": {Target: "listPets", Risk: "read"},
	}}
	shortcuts, err := BuildShortcuts(&seedProviderSystem{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(shortcuts) != 1 || shortcuts[0].Name != "+seed" {
		t.Fatalf("shortcuts = %#v", shortcuts)
	}
	// 内置版不带 Params（config 版带）；若 config 覆盖了内置，Params 非空
	if len(shortcuts[0].Params) != 0 {
		t.Fatalf("builtin shortcut should win over same-name config: %#v", shortcuts[0])
	}
	// 覆盖发生时应记录 warning（取走即清空）
	found := false
	for _, w := range TakeShortcutWarnings() {
		if strings.Contains(w, "+seed") {
			found = true
		}
	}
	if !found {
		t.Fatal("same-name config shortcut should produce a warning")
	}
}

// seedProviderSystem 实现编译期 Shortcuts 接口（+seed 无参数版）。
type seedProviderSystem struct {
	shortcutTestSystem
}

func (s *seedProviderSystem) Shortcuts() []Shortcut {
	return []Shortcut{{Name: "+seed", Target: "listPets", Risk: "read"}}
}

// TestExecuteShortcutMergesParams 用真实 httptest 验证参数合并：
// 仅传预设 → query 含 limit=5；显式参数 → 覆盖预设。
func TestExecuteShortcutMergesParams(t *testing.T) {
	isolateHome(t)
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	system := &shortcutHTTPSystem{baseURL: server.URL}
	sc := Shortcut{Name: "+list", Target: "listPets", Params: map[string]string{"query.limit": "5"}, Risk: "read"}

	// 仅预设参数
	if _, err := ExecuteShortcut(context.Background(), system, sc, CallOptions{System: "test"}); err != nil {
		t.Fatal(err)
	}
	if gotQuery != "limit=5" {
		t.Fatalf("query = %q, want limit=5 (preset params missing)", gotQuery)
	}

	// 显式参数覆盖预设
	if _, err := ExecuteShortcut(context.Background(), system, sc, CallOptions{
		System: "test", Params: map[string]string{"query.limit": "2"},
	}); err != nil {
		t.Fatal(err)
	}
	if gotQuery != "limit=2" {
		t.Fatalf("query = %q, want limit=2 (explicit should override preset)", gotQuery)
	}
}

// shortcutHTTPSystem 是接真实网络栈的 shortcut 测试系统。
type shortcutHTTPSystem struct {
	staticSystem
	baseURL string
}

func (s *shortcutHTTPSystem) BaseURL() string { return s.baseURL }

func TestFindShortcut(t *testing.T) {
	shortcuts := []Shortcut{{Name: "+list"}}
	if _, ok := FindShortcut(shortcuts, "list"); !ok {
		t.Fatal("FindShortcut without + should match")
	}
	if _, ok := FindShortcut(shortcuts, "+list"); !ok {
		t.Fatal("FindShortcut with + should match")
	}
	if _, ok := FindShortcut(shortcuts, "missing"); ok {
		t.Fatal("missing shortcut should not match")
	}
}
