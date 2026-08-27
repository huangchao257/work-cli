package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoadBytesJSONAndYAML(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want string
	}{
		{"json-30", `{"openapi":"3.0.3","info":{"title":"Pets","version":"1"},"paths":{"/pets":{"get":{"operationId":"listPets","responses":{"200":{"description":"ok"}}}}}}`, "3.0.3"},
		{"yaml-31", "openapi: 3.1.0\ninfo:\n  title: Pets\n  version: '1'\npaths:\n  /pets:\n    get:\n      operationId: listPets\n      responses:\n        '200':\n          description: ok\n", "3.1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := LoadBytes([]byte(tt.spec))
			if err != nil {
				t.Fatal(err)
			}
			if doc.OpenAPI != tt.want {
				t.Fatalf("OpenAPI = %q, want %q", doc.OpenAPI, tt.want)
			}
		})
	}
}

func TestLoadBytesRejectsSwaggerAndMissingPaths(t *testing.T) {
	for _, spec := range []string{
		`{"swagger":"2.0","info":{"title":"x","version":"1"},"paths":{"/x":{}}}`,
		`{"openapi":"3.0.3","info":{"title":"x","version":"1"},"paths":{}}`,
	} {
		if _, err := LoadBytes([]byte(spec)); err == nil {
			t.Fatalf("LoadBytes(%s) should fail", spec)
		}
	}
}

func TestLoadURLRequiresHTTPS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	if _, _, err := LoadURL(context.Background(), server.URL, server.Client()); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS error, got %v", err)
	}
}

func TestBaseURLReplacesServerVariables(t *testing.T) {
	doc := &Document{Servers: []Server{{
		URL: "https://{region}.example.com/{version}/",
		Variables: map[string]ServerVariable{
			"region":  {Default: "cn"},
			"version": {Default: "v1"},
		},
	}}}
	if got := doc.BaseURL(); got != "https://cn.example.com/v1" {
		t.Fatalf("BaseURL() = %q", got)
	}
}
