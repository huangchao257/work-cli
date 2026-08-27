package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- R1: import --overwrite 延迟删除 ---

func TestImportOverwriteBadSpecKeepsOldSystem(t *testing.T) {
	home := isolateHome(t)
	specDir := t.TempDir()
	goodPath := filepath.Join(specDir, "good.yaml")
	badPath := filepath.Join(specDir, "bad.yaml")
	if err := os.WriteFile(goodPath, []byte(petstoreSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("not-a-spec"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Import(context.Background(), ImportOptions{Name: "keepme", Spec: goodPath}); err != nil {
		t.Fatal(err)
	}
	// 非 dry-run + 坏规范 + --overwrite：旧系统必须保留
	if _, err := Import(context.Background(), ImportOptions{Name: "keepme", Spec: badPath, Overwrite: true}); err == nil {
		t.Fatal("bad spec should fail")
	}
	if !SystemExists("keepme") {
		t.Fatal("old system was deleted despite validation failure")
	}
	// dry-run + --overwrite 同样不删
	if _, err := Import(context.Background(), ImportOptions{Name: "keepme", Spec: badPath, Overwrite: true, DryRun: true}); err == nil {
		t.Fatal("bad spec should fail even in dry-run")
	}
	if !SystemExists("keepme") {
		t.Fatal("old system was deleted in dry-run")
	}
	_ = home
}

func TestImportDuplicateWithoutOverwriteRejected(t *testing.T) {
	isolateHome(t)
	specPath := filepath.Join(t.TempDir(), "petstore.yaml")
	if err := os.WriteFile(specPath, []byte(petstoreSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(context.Background(), ImportOptions{Name: "dup", Spec: specPath}); err != nil {
		t.Fatal(err)
	}
	_, err := Import(context.Background(), ImportOptions{Name: "dup", Spec: specPath})
	if err == nil {
		t.Fatal("duplicate import without --overwrite should be rejected")
	}
	if !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("error should hint --overwrite, got: %v", err)
	}
}

// --- R1: 系统名点开头拒绝 ---

func TestImportRejectsDotPrefixedNames(t *testing.T) {
	isolateHome(t)
	specPath := filepath.Join(t.TempDir(), "petstore.yaml")
	if err := os.WriteFile(specPath, []byte(petstoreSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".", "..", ".hidden"} {
		if _, err := Import(context.Background(), ImportOptions{Name: name, Spec: specPath}); err == nil {
			t.Fatalf("Import(%q) should be rejected", name)
		}
	}
	// systems 目录不应有越界写入
	root, _ := SystemsDir()
	if _, err := os.Stat(filepath.Join(root, "system.yaml")); err == nil {
		t.Fatal("dot-name import wrote outside systems/<name>/")
	}
}

// --- R1: buildRequest 参数放置矩阵 ---

const dualSpec = `
openapi: 3.0.3
info: {title: Dual, version: "1"}
servers: [{url: http://127.0.0.1:19990}]
tags: [{name: t}]
paths:
  /q:
    get:
      operationId: dualQuery
      tags: [t]
      parameters:
        - {name: limit, in: query, schema: {type: integer}}
      responses: {"200": {description: ok}}
  /who:
    get:
      operationId: whoami
      tags: [t]
      parameters:
        - {name: session, in: cookie, required: true, schema: {type: string}}
      responses: {"200": {description: ok}}
`

func newDualSystem(t *testing.T, baseURL string) System {
	t.Helper()
	spec := strings.Replace(dualSpec, "http://127.0.0.1:19990", baseURL, 1)
	doc, err := openapiLoadBytes(spec)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := doc.Index()
	if err != nil {
		t.Fatal(err)
	}
	return &dualSystem{baseURL: baseURL, catalog: catalog}
}

type dualSystem struct {
	baseURL string
	catalog *catalogType
}

func (s *dualSystem) Manifest() Manifest                                { return Manifest{Name: "dual", Source: "builtin"} }
func (s *dualSystem) BaseURL() string                                   { return s.baseURL }
func (s *dualSystem) Catalog(ctx context.Context) (*catalogType, error) { return s.catalog, nil }
func (s *dualSystem) Document(ctx context.Context) (*docType, error)    { return nil, nil }

func TestCallCookieParamNotLeakedToQuery(t *testing.T) {
	isolateHome(t)
	var gotCookie, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	s := newDualSystem(t, server.URL)
	// 裸名键命中声明的 cookie 参数：进 Cookie header，不泄漏 query
	result, err := Call(context.Background(), s, CallOptions{
		System: "dual", Operation: "whoami",
		Params: map[string]string{"session": "s1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if gotCookie != "session=s1" {
		t.Fatalf("Cookie = %q", gotCookie)
	}
	if gotQuery != "" {
		t.Fatalf("query leaked session: %q", gotQuery)
	}
}

func TestCallPrefixKeyBeatsBareKey(t *testing.T) {
	isolateHome(t)
	var gotRawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	s := newDualSystem(t, server.URL)
	// 前缀键优先于裸名键（确定性），且裸名值不得以重复键形式泄漏
	result, err := Call(context.Background(), s, CallOptions{
		System: "dual", Operation: "dualQuery",
		Params: map[string]string{"query.limit": "5", "limit": "9"},
	})
	if err != nil || !result.OK {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	// 整串精确匹配：任何 limit=9 泄漏（如 Add 产生重复键）都会破坏该断言
	if gotRawQuery != "limit=5" {
		t.Fatalf("RawQuery = %q, want exactly limit=5", gotRawQuery)
	}
}

// --- R1: 嵌套 $ref 解析 ---

func TestChainedParameterRefResolves(t *testing.T) {
	doc, err := openapiLoadBytes(`
openapi: 3.0.3
info: {title: Ref, version: "1"}
servers: [{url: https://example.com}]
paths:
  /r:
    get:
      operationId: refOp
      parameters:
        - $ref: '#/components/parameters/Outer'
      responses: {"200": {description: ok}}
components:
  parameters:
    Outer:
      $ref: '#/components/parameters/Inner'
    Inner:
      name: limit
      in: query
      required: true
      schema: {type: integer, default: 10}
`)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := doc.Index()
	if err != nil {
		t.Fatal(err)
	}
	op, ok := catalog.FindByID("refOp")
	if !ok {
		t.Fatal("refOp not found")
	}
	if len(op.Parameters) != 1 || op.Parameters[0].Name != "limit" || !op.Parameters[0].Required {
		t.Fatalf("chained $ref not resolved: %#v", op.Parameters)
	}
	if op.Parameters[0].Default != 10 {
		t.Fatalf("default not resolved: %#v", op.Parameters[0])
	}
}

func TestCyclicParameterRefWarnsAndDrops(t *testing.T) {
	doc, err := openapiLoadBytes(`
openapi: 3.0.3
info: {title: Cycle, version: "1"}
servers: [{url: https://example.com}]
paths:
  /c:
    get:
      operationId: cycleOp
      parameters:
        - $ref: '#/components/parameters/A'
      responses: {"200": {description: ok}}
components:
  parameters:
    A:
      $ref: '#/components/parameters/B'
    B:
      $ref: '#/components/parameters/A'
`)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := doc.Index()
	if err != nil {
		t.Fatal(err)
	}
	op, _ := catalog.FindByID("cycleOp")
	if len(op.Parameters) != 0 {
		t.Fatalf("cyclic ref param should be dropped: %#v", op.Parameters)
	}
	found := false
	for _, w := range op.Warnings {
		if strings.Contains(w, "循环") {
			found = true
		}
	}
	if !found {
		t.Fatalf("cyclic ref should warn: %#v", op.Warnings)
	}
}
