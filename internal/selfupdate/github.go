package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
)

const DefaultRepo = "huangchao257/work-cli"

type releaseInfo struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

type assetRef struct {
	Tag     string
	Version string
	Name    string
	URL     string
}

func fetchLatestRelease(ctx context.Context, client *http.Client, repo string) (*releaseInfo, error) {
	if client == nil {
		client = http.DefaultClient
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "work-cli")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取最新版本失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var info releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("解析 Release 信息失败: %w", err)
	}
	if info.TagName == "" {
		return nil, fmt.Errorf("Release 缺少 tag_name")
	}
	return &info, nil
}

func resolveAsset(info *releaseInfo, version string) (*assetRef, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	ver := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if ver == "" {
		ver = strings.TrimPrefix(strings.TrimSpace(info.TagName), "v")
	}

	ext := "tar.gz"
	if osName == "windows" {
		ext = "zip"
	}
	want := fmt.Sprintf("work_%s_%s_%s.%s", ver, osName, arch, ext)

	for _, asset := range info.Assets {
		if asset.Name == want {
			return &assetRef{
				Tag:     info.TagName,
				Version: ver,
				Name:    asset.Name,
				URL:     asset.URL,
			}, nil
		}
	}
	return nil, fmt.Errorf("未找到当前平台 (%s/%s) 的安装包: %s", osName, arch, want)
}

func fetchReleaseByTag(ctx context.Context, client *http.Client, repo, tag string) (*releaseInfo, error) {
	if client == nil {
		client = http.DefaultClient
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, fmt.Errorf("版本号不能为空")
	}
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "work-cli")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取版本 %s 失败: %w", tag, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var info releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("解析 Release 信息失败: %w", err)
	}
	if info.TagName == "" {
		info.TagName = tag
	}
	return &info, nil
}
