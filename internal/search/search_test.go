package search

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// findItem 在 items 中按 name 查找，返回条目与是否命中。
func findItem(items []Item, name string) (Item, bool) {
	for _, it := range items {
		if it.Name == name {
			return it, true
		}
	}
	return Item{}, false
}

// TestLoadBuiltinKnownItems 校验内置 catalog 中已知条目被正确解析，且 type 与清单类型一致。
func TestLoadBuiltinKnownItems(t *testing.T) {
	items, warnings := loadBuiltin()
	if len(items) == 0 {
		t.Fatalf("loadBuiltin 未返回任何条目（warnings: %v）", warnings)
	}

	// dev-kit 是 bundle 类型内置项，必须出现且 type 正确。
	it, ok := findItem(items, "dev-kit")
	if !ok {
		t.Fatalf("loadBuiltin 未返回 dev-kit，得到 %v", itemNames(items))
	}
	if it.Type != "bundle" {
		t.Errorf("dev-kit type 期望 bundle，得到 %q", it.Type)
	}
	if it.Version == "" {
		t.Errorf("dev-kit version 为空")
	}
	if it.Source != "builtin" {
		t.Errorf("dev-kit source 期望 builtin，得到 %q", it.Source)
	}

	// codegraph 是 cli 类型。
	if cg, ok := findItem(items, "codegraph"); ok {
		if cg.Type != "cli" {
			t.Errorf("codegraph type 期望 cli，得到 %q", cg.Type)
		}
	}

	// company-hooks 是 hooks 类型。
	if ch, ok := findItem(items, "company-hooks"); ok {
		if ch.Type != "hooks" {
			t.Errorf("company-hooks type 期望 hooks，得到 %q", ch.Type)
		}
	}
}

// itemNames 返回条目名称列表，便于错误信息阅读。
func itemNames(items []Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Name)
	}
	return out
}

// TestRunQueryFilterHitAndMiss 校验 query 子串过滤命中与不命中。
func TestRunQueryFilterHitAndMiss(t *testing.T) {
	all, _ := Run(Options{})
	if len(all.Items) == 0 {
		t.Skip("当前环境无内置 catalog，跳过过滤测试")
	}

	// 取一个肯定存在的名字做精确命中。
	first := all.Items[0].Name
	hit, _ := Run(Options{Query: first})
	if len(hit.Items) == 0 {
		t.Errorf("query=%q 期望至少命中一条，实际 0", first)
	}
	found := false
	for _, it := range hit.Items {
		if it.Name == first {
			found = true
		}
	}
	if !found {
		t.Errorf("query=%q 未在结果中包含原条目", first)
	}

	// 一个不可能命中的子串。
	miss, _ := Run(Options{Query: "___zzz_not_exist_qqq___"})
	if len(miss.Items) != 0 {
		t.Errorf("不可能命中的 query 期望 0 条，得到 %d", len(miss.Items))
	}
}

// TestRunQueryCaseInsensitive 校验 query 不区分大小写。
func TestRunQueryCaseInsensitive(t *testing.T) {
	all, _ := Run(Options{})
	if len(all.Items) == 0 {
		t.Skip("当前环境无内置 catalog，跳过大小写测试")
	}
	first := all.Items[0].Name
	upper := strings.ToUpper(first)
	hit, _ := Run(Options{Query: upper})
	if len(hit.Items) == 0 {
		t.Errorf("query 大写 %q 期望命中，实际 0", upper)
	}
}

// TestRunRemoteDisabledByDefault 校验未开启 Remote 时只返回本地条目。
func TestRunRemoteDisabledByDefault(t *testing.T) {
	res, _ := Run(Options{})
	for _, it := range res.Items {
		if it.Source == "registry" {
			t.Errorf("未启用 Remote 却出现 registry 条目: %v", it)
		}
	}
}

// TestRunRemoteEmptyURL 校验 registry.url 为空时 Remote 不报错，只产 warning。
func TestRunRemoteEmptyURL(t *testing.T) {
	res, err := Run(Options{Remote: true, RegistryURL: ""})
	if err != nil {
		t.Fatalf("Remote=true 且 url 为空不应返回 error: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Fatalf("期望至少一条 warning")
	}
	got := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "未配置 registry.url") {
			got = true
		}
	}
	if !got {
		t.Errorf("warning 未包含「未配置 registry.url」，得到 %v", res.Warnings)
	}
}

// TestFetchRegistryParsesResponse 用 httptest mock Registry，校验响应解析正确。
func TestFetchRegistryParsesResponse(t *testing.T) {
	payload := []map[string]string{
		{"name": "team-bundle", "type": "bundle", "version": "2.1.0", "description": "团队套装"},
		{"name": "team-cli", "type": "cli", "version": "0.3.1", "description": "团队 CLI"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bundles" {
			t.Errorf("请求路径期望 /bundles，得到 %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	items, warn := fetchRegistry(srv.URL)
	if warn != "" {
		t.Fatalf("不期望 warning，得到 %q", warn)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 条，得到 %d", len(items))
	}
	tb, ok := findItem(items, "team-bundle")
	if !ok {
		t.Fatalf("未找到 team-bundle")
	}
	if tb.Version != "2.1.0" || tb.Type != "bundle" || tb.Description != "团队套装" || tb.Source != "registry" {
		t.Errorf("team-bundle 解析异常: %+v", tb)
	}
}

// TestFetchRegistryServerError 校验服务端错误时返回 warning 而非 panic。
func TestFetchRegistryServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	items, warn := fetchRegistry(srv.URL)
	if len(items) != 0 {
		t.Errorf("服务端错误时不应返回条目，得到 %d", len(items))
	}
	if warn == "" {
		t.Errorf("服务端错误时期望 warning")
	}
}

// TestFetchRegistryMalformedJSON 校验响应体非法时返回 warning。
func TestFetchRegistryMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	items, warn := fetchRegistry(srv.URL)
	if len(items) != 0 {
		t.Errorf("非法响应不应返回条目")
	}
	if warn == "" {
		t.Errorf("非法响应期望 warning")
	}
}
