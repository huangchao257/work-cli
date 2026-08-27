package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/huangchao257/work-cli/internal/openapi"
)

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

const petstoreSpec = `
openapi: 3.0.3
info: {title: Petstore, version: "1.0"}
servers:
  - url: https://petstore.example.com
paths:
  /pets:
    get:
      operationId: listPets
      tags: [pets]
      parameters:
        - {name: limit, in: query, schema: {type: integer, default: 10}}
      responses: {"200": {description: ok}}
    post:
      operationId: createPet
      tags: [pets]
      requestBody:
        required: true
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Pet'}
      responses: {"201": {description: created}}
  /pets/{id}:
    parameters:
      - {name: id, in: path, required: true, schema: {type: string}}
    get:
      operationId: getPet
      tags: [pets]
      responses: {"200": {description: ok}, "404": {description: missing}}
components:
  schemas:
    Pet:
      type: object
      required: [name]
      properties:
        name: {type: string}
`

func importPetstore(t *testing.T, name string) {
	t.Helper()
	specPath := filepath.Join(t.TempDir(), "petstore.yaml")
	if err := os.WriteFile(specPath, []byte(petstoreSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(context.Background(), ImportOptions{Name: name, Spec: specPath}); err != nil {
		t.Fatal(err)
	}
}

func TestImportGeneratesCatalogAndConfig(t *testing.T) {
	isolateHome(t)
	importPetstore(t, "petstore")

	cfg, exists, err := LoadSystemConfig("petstore")
	if err != nil || !exists {
		t.Fatalf("LoadSystemConfig: %v exists=%v", err, exists)
	}
	if cfg.Auth.Kind != AuthNone || cfg.SpecFile != "openapi.yaml" {
		t.Fatalf("unexpected cfg: %#v", cfg)
	}
	systems, warnings := ImportedSystems()
	if len(warnings) != 0 || len(systems) != 1 {
		t.Fatalf("systems=%d warnings=%v", len(systems), warnings)
	}
	catalog, err := systems[0].Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Operations) != 3 {
		t.Fatalf("operations = %d", len(catalog.Operations))
	}
	if systems[0].BaseURL() != "https://petstore.example.com" {
		t.Fatalf("BaseURL = %q", systems[0].BaseURL())
	}
}

func TestImportRejectsReservedAndInvalidNames(t *testing.T) {
	isolateHome(t)
	for _, name := range []string{"list", "call", "bad name", "a/b"} {
		if _, err := Import(context.Background(), ImportOptions{Name: name, Spec: "x.yaml"}); err == nil {
			t.Fatalf("Import(%q) should fail", name)
		}
	}
}

func TestImportRejectsSwagger(t *testing.T) {
	isolateHome(t)
	specPath := filepath.Join(t.TempDir(), "swagger.json")
	if err := os.WriteFile(specPath, []byte(`{"swagger":"2.0","info":{"title":"x","version":"1"},"paths":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(context.Background(), ImportOptions{Name: "x", Spec: specPath}); err == nil {
		t.Fatal("swagger import should fail")
	}
}

func TestRefreshSkipsSystemsWithoutSourceURL(t *testing.T) {
	isolateHome(t)
	importPetstore(t, "petstore")
	results, err := Refresh(context.Background(), RefreshOptions{Name: "petstore"})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Updated || !strings.Contains(results[0].Reason, "HTTPS") {
		t.Fatalf("unexpected result: %#v", results[0])
	}
}

func TestRemoveSystemIdempotent(t *testing.T) {
	isolateHome(t)
	importPetstore(t, "petstore")
	if err := RemoveSystem("petstore"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveSystem("petstore"); err != nil {
		t.Fatalf("second remove should be idempotent: %v", err)
	}
	if SystemExists("petstore") {
		t.Fatal("system should be removed")
	}
}

func TestSystemConfigYAMLRoundTrip(t *testing.T) {
	cfg := &SystemConfig{
		Name: "demo", Description: "示例",
		Auth: AuthConfig{Kind: AuthBearer, CredentialEnv: "DEMO_TOKEN"},
		Shortcuts: map[string]ShortcutDef{
			"list": {Target: "listPets", Description: "列出", Params: map[string]string{"limit": "10"}, Risk: "read"},
		},
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var decoded SystemConfig
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Auth.Kind != AuthBearer || decoded.Shortcuts["list"].Target != "listPets" {
		t.Fatalf("round trip mismatch: %#v", decoded)
	}
}

func TestCallReadOperationWithHTTPServer(t *testing.T) {
	isolateHome(t)
	var capturedQuery, capturedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("limit")
		capturedAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	spec := strings.Replace(petstoreSpec, "https://petstore.example.com", server.URL, 1)
	specPath := filepath.Join(t.TempDir(), "petstore.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(context.Background(), ImportOptions{Name: "petstore", Spec: specPath}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PETSTORE_TOKEN", "secret-token")

	systems, _ := ImportedSystems()
	result, err := Call(context.Background(), systems[0], CallOptions{
		System: "petstore", Operation: "listPets",
		Params:     map[string]string{"query.limit": "2"},
		AuthConfig: AuthConfig{Kind: AuthBearer, CredentialEnv: "PETSTORE_TOKEN"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Status != 200 {
		t.Fatalf("result = %#v", result)
	}
	if capturedQuery != "2" {
		t.Fatalf("query limit = %q", capturedQuery)
	}
	if capturedAuth != "Bearer secret-token" {
		t.Fatalf("auth = %q", capturedAuth)
	}
}

func TestCallMissingCredentialFailsWithHint(t *testing.T) {
	isolateHome(t)
	importPetstore(t, "petstore")
	// 空串语义等价于未设置（credential 以 TrimSpace 判空），且 t.Setenv 自动恢复
	t.Setenv("PETSTORE_TOKEN", "")

	systems, _ := ImportedSystems()
	_, err := Call(context.Background(), systems[0], CallOptions{
		System: "petstore", Operation: "listPets",
		AuthConfig: AuthConfig{Kind: AuthBearer, CredentialEnv: "PETSTORE_TOKEN"},
	})
	if err == nil || !strings.Contains(err.Error(), "PETSTORE_TOKEN") {
		t.Fatalf("expected credential hint error, got %v", err)
	}
}

func TestCallWriteRequiresConfirmationFailClosed(t *testing.T) {
	isolateHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("write request should not be sent")
	}))
	defer server.Close()

	spec := strings.Replace(petstoreSpec, "https://petstore.example.com", server.URL, 1)
	specPath := filepath.Join(t.TempDir(), "petstore.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(context.Background(), ImportOptions{Name: "petstore", Spec: specPath}); err != nil {
		t.Fatal(err)
	}

	systems, _ := ImportedSystems()
	_, err := Call(context.Background(), systems[0], CallOptions{
		System: "petstore", Operation: "createPet", Body: `{"name":"rex"}`,
	})
	if err == nil {
		t.Fatal("write call without --yes should fail closed")
	}
	if _, ok := err.(*ConfirmationRequiredError); !ok {
		t.Fatalf("expected ConfirmationRequiredError, got %T", err)
	}
}

func TestCallDryRunNeverSends(t *testing.T) {
	isolateHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("dry-run should not send request")
	}))
	defer server.Close()

	spec := strings.Replace(petstoreSpec, "https://petstore.example.com", server.URL, 1)
	specPath := filepath.Join(t.TempDir(), "petstore.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(context.Background(), ImportOptions{Name: "petstore", Spec: specPath}); err != nil {
		t.Fatal(err)
	}
	systems, _ := ImportedSystems()
	result, err := Call(context.Background(), systems[0], CallOptions{
		System: "petstore", Operation: "createPet", Body: `{"name":"rex"}`, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.DryRun || result.Status != 0 {
		t.Fatalf("dry-run result = %#v", result)
	}
}

func TestCallResolvesMethodPath(t *testing.T) {
	isolateHome(t)
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pets" {
			gotMethod = r.Method
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	spec := strings.Replace(petstoreSpec, "https://petstore.example.com", server.URL, 1)
	specPath := filepath.Join(t.TempDir(), "petstore.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(context.Background(), ImportOptions{Name: "petstore", Spec: specPath}); err != nil {
		t.Fatal(err)
	}
	systems, _ := ImportedSystems()
	result, err := Call(context.Background(), systems[0], CallOptions{
		System: "petstore", Operation: "GET /pets",
	})
	if err != nil || !result.OK {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	// METHOD PATH 解析必须命中 GET 而非同 path 的其他方法
	if gotMethod != http.MethodGet || result.Method != "GET" {
		t.Fatalf("method = %q (server saw %q), want GET", result.Method, gotMethod)
	}
}

func TestCallMissingRequiredPathParam(t *testing.T) {
	isolateHome(t)
	importPetstore(t, "petstore")
	systems, _ := ImportedSystems()
	_, err := Call(context.Background(), systems[0], CallOptions{
		System: "petstore", Operation: "getPet",
	})
	if err == nil || !strings.Contains(err.Error(), "缺少必填参数") {
		t.Fatalf("expected missing param error, got %v", err)
	}
}

func TestCallHTTPErrorEnvelope(t *testing.T) {
	isolateHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	spec := strings.Replace(petstoreSpec, "https://petstore.example.com", server.URL, 1)
	specPath := filepath.Join(t.TempDir(), "petstore.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(context.Background(), ImportOptions{Name: "petstore", Spec: specPath}); err != nil {
		t.Fatal(err)
	}
	systems, _ := ImportedSystems()
	result, err := Call(context.Background(), systems[0], CallOptions{
		System: "petstore", Operation: "getPet", Params: map[string]string{"path.id": "missing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || result.Status != 404 || result.Error == nil || result.Error.Subtype != "http_404" {
		t.Fatalf("result = %#v", result)
	}
}

func TestRegistryRegisterAndFreeze(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(&staticSystem{name: "demo"}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(&staticSystem{name: "demo"}); err == nil {
		t.Fatal("duplicate register should fail")
	}
	if err := registry.Register(&staticSystem{name: "call"}); err == nil {
		t.Fatal("reserved name should fail")
	}
	registry.Freeze()
	if err := registry.Register(&staticSystem{name: "another"}); err == nil {
		t.Fatal("frozen registry should reject")
	}
	if _, ok := registry.ByName("demo"); !ok {
		t.Fatal("demo should be registered")
	}
}

// staticSystem 是测试用的最小 System 实现。
type staticSystem struct {
	name string
}

func (s *staticSystem) Manifest() Manifest { return Manifest{Name: s.name, Source: "builtin"} }

func (s *staticSystem) Catalog(ctx context.Context) (*openapi.Catalog, error) {
	doc, err := openapi.LoadBytes([]byte(petstoreSpec))
	if err != nil {
		return nil, err
	}
	return doc.Index()
}

func (s *staticSystem) Document(ctx context.Context) (*openapi.Document, error) {
	return openapi.LoadBytes([]byte(petstoreSpec))
}

func (s *staticSystem) BaseURL() string { return "" }
