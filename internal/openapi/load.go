package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const MaxSpecSize int64 = 8 << 20

// LoadBytes 解析 JSON 或 YAML 格式的 OpenAPI 3.0/3.1 文档。
func LoadBytes(data []byte) (*Document, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("OpenAPI 文档为空")
	}
	if int64(len(data)) > MaxSpecSize {
		return nil, fmt.Errorf("OpenAPI 文档超过 %d MiB 限制", MaxSpecSize>>20)
	}

	var doc Document
	trimmed := bytes.TrimSpace(data)
	var err error
	if len(trimmed) > 0 && trimmed[0] == '{' {
		err = json.Unmarshal(trimmed, &doc)
	} else {
		err = yaml.Unmarshal(trimmed, &doc)
	}
	if err != nil {
		return nil, fmt.Errorf("解析 OpenAPI 文档失败: %w", err)
	}
	if strings.TrimSpace(doc.Swagger) != "" || strings.HasPrefix(strings.TrimSpace(doc.OpenAPI), "2.") {
		return nil, fmt.Errorf("不支持 OpenAPI 2.0/Swagger，请转换为 OpenAPI 3.x")
	}
	version := strings.TrimSpace(doc.OpenAPI)
	if !strings.HasPrefix(version, "3.0") && !strings.HasPrefix(version, "3.1") {
		return nil, fmt.Errorf("仅支持 OpenAPI 3.0/3.1，当前版本为 %q", version)
	}
	if len(doc.Paths) == 0 {
		return nil, fmt.Errorf("OpenAPI 文档未定义 paths")
	}
	return &doc, nil
}

// LoadFile 从本地文件加载 OpenAPI 文档。
func LoadFile(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开 OpenAPI 文档失败: %w", err)
	}
	defer f.Close()
	data, err := readLimited(f)
	if err != nil {
		return nil, err
	}
	return LoadBytes(data)
}

// LoadURL 从 HTTPS URL 加载 OpenAPI 文档。client 为 nil 时使用 30 秒超时客户端。
func LoadURL(ctx context.Context, rawURL string, client *http.Client) (*Document, []byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, nil, fmt.Errorf("远程 OpenAPI URL 必须使用 HTTPS")
	}
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
			// 与 source/registry.go 的 newSecureHTTPClient 同理：拒绝协议降级重定向，
			// 防止 https 的规范 URL 被 302 到 http:// 绕过入口的 HTTPS-only 检查
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("重定向次数过多")
				}
				if req.URL.Scheme != "https" {
					return fmt.Errorf("拒绝重定向到非 HTTPS 协议: %s", req.URL.Scheme)
				}
				return nil
			},
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("创建 OpenAPI 请求失败: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("下载 OpenAPI 文档失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("下载 OpenAPI 文档失败: %s", resp.Status)
	}
	data, err := readLimited(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	doc, err := LoadBytes(data)
	return doc, data, err
}

func readLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxSpecSize+1))
	if err != nil {
		return nil, fmt.Errorf("读取 OpenAPI 文档失败: %w", err)
	}
	if int64(len(data)) > MaxSpecSize {
		return nil, fmt.Errorf("OpenAPI 文档超过 %d MiB 限制", MaxSpecSize>>20)
	}
	return data, nil
}
