package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/huangchao257/work-cli/internal/openapi"
	"github.com/huangchao257/work-cli/internal/usage"
)

// --- 缺口 1: 受限 header 门禁（凭据不可被命令行覆盖） ---

func TestCallRejectsRestrictedHeaderOverride(t *testing.T) {
	isolateHome(t)
	doc, err := openapiLoadBytes(`
openapi: 3.0.3
info: {title: R, version: "1"}
servers: [{url: https://r.invalid}]
paths:
  /r:
    get:
      operationId: r
      responses: {"200": {description: ok}}
`)
	if err != nil {
		t.Fatal(err)
	}
	catalog, _ := doc.Index()
	op, _ := catalog.FindByID("r")

	for header, via := range map[string]string{
		"Authorization": "flag",       // --header
		"authorization": "flag-lower", // 大小写归一
		" Cookie":       "param",      // --set header.Cookie（含空白）
		"Content-Type":  "param",
		"Host":          "param",
	} {
		var err error
		if strings.HasPrefix(via, "flag") {
			_, err = buildRequest("https://r.invalid", op, nil, "", map[string]string{header: "evil"})
		} else {
			_, err = buildRequest("https://r.invalid", op, map[string]string{"header." + strings.TrimSpace(header): "evil"}, "", nil)
		}
		if err == nil {
			t.Fatalf("restricted header %q should be rejected via %s", header, via)
		}
		var usageErr *usage.Error
		if !errors.As(err, &usageErr) {
			t.Fatalf("restricted header rejection should be usage error, got: %v", err)
		}
	}
}

// --- 缺口 3: 超时链（parseTimeout + client 装配） ---

func TestParseTimeoutDefaults(t *testing.T) {
	// 空 → 默认 30s
	dur, err := parseTimeout("")
	if err != nil || dur.Seconds() != 30 {
		t.Fatalf("parseTimeout('') = %v, %v; want 30s", dur, err)
	}
	if _, err := parseTimeout("10s"); err != nil {
		t.Fatalf("parseTimeout('10s') failed: %v", err)
	}
	if _, err := parseTimeout("abc"); err == nil {
		t.Fatal("parseTimeout('abc') should fail")
	}
	if _, err := parseTimeout("-5s"); err == nil {
		t.Fatal("parseTimeout('-5s') should fail")
	}
}

func TestCallTimeoutActuallyApplied(t *testing.T) {
	isolateHome(t)
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 服务端睡 2s；客户端超时 100ms 应先断
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer slow.Close()

	s := newSingleOpSystem("redir", t, slow.URL)
	_, err := Call(context.Background(), s, CallOptions{
		System: "redir", Operation: "op", Timeout: "100ms", Yes: true,
	})
	if err == nil {
		t.Fatal("slow server should trigger client timeout")
	}
	var info *CallErrorInfo
	if !errors.As(err, &info) || info.Type != "environment" {
		t.Fatalf("timeout should classify as environment error: %v", err)
	}
}

// --- 缺口 5: LoadURL 下载大小上限（注入 RoundTripper，无需真网络） ---

func TestLoadURLRejectsOversizedSpec(t *testing.T) {
	big := io.NopCloser(strings.NewReader(`{"openapi":"3.0.3","x":"` + strings.Repeat("a", 9<<20) + `"}`))
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200, Body: big,
			Header: http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	_, _, err := openapi.LoadURL(r_context(), "https://spec.invalid/openapi.json", client)
	if err == nil || !strings.Contains(err.Error(), "限制") {
		t.Fatalf("oversized spec should be rejected: %v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func r_context() context.Context { return context.Background() }
