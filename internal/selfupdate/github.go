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
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
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

// gitHubAPI 创建对 GitHub API 的通用 HTTP 请求，设置必要的头部。
// 返回 HTTP 响应与错误。调用方负责关闭 resp.Body。
func gitHubAPI(ctx context.Context, client *http.Client, apiPath, repo string) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", repo, apiPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "work-cli")
	return client.Do(req)
}

// fetchReleaseResponse 调用 gitHubAPI 并解析返回的 releaseInfo。
func fetchReleaseResponse(ctx context.Context, client *http.Client, apiPath, repo string, errMsg string) (*releaseInfo, error) {
	resp, err := gitHubAPI(ctx, client, apiPath, repo)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errMsg, err)
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
	return &info, nil
}

func fetchLatestRelease(ctx context.Context, client *http.Client, repo, channel string) (*releaseInfo, error) {
	switch channel {
	case "beta":
		return fetchLatestPrerelease(ctx, client, repo)
	default:
		return fetchStableRelease(ctx, client, repo)
	}
}

func fetchStableRelease(ctx context.Context, client *http.Client, repo string) (*releaseInfo, error) {
	info, err := fetchReleaseResponse(ctx, client, "releases/latest", repo, "获取最新版本失败")
	if err != nil {
		return nil, err
	}
	if info.TagName == "" {
		return nil, fmt.Errorf("Release 缺少 tag_name 字段")
	}
	return info, nil
}

// fetchLatestPrerelease 从 releases 列表 API 获取最新的 pre-release。
func fetchLatestPrerelease(ctx context.Context, client *http.Client, repo string) (*releaseInfo, error) {
	resp, err := gitHubAPI(ctx, client, "releases?per_page=20", repo)
	if err != nil {
		return nil, fmt.Errorf("获取 beta 版本失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var releases []releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("解析 Release 列表失败: %w", err)
	}
	for _, r := range releases {
		if r.Prerelease && r.TagName != "" {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("未找到 beta 版本")
}

// archAliases 将常见的 GOARCH 变体映射到标准名称，用于资产匹配。
var archAliases = map[string][]string{
	"amd64": {"x86_64", "amd64", "x64"},
	"arm64": {"aarch64", "arm64", "aarch64_be", "armv8"},
	"386":   {"i386", "i686", "386", "x86"},
}

// resolveAsset 从 releaseInfo 中选择当前平台的下载资产。
// 优先精确匹配，若无精确匹配则尝试 GOARCH 别名与扩展名变体。
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

	// 候选扩展名列表（优先精确扩展，其次尝试另一种）
	altExt := "zip"
	if ext == "zip" {
		altExt = "tar.gz"
	}
	extCandidates := []string{ext, altExt}

	// 候选架构名列表（当前 arch + 别名）
	archCandidates := []string{arch}
	if aliases, ok := archAliases[arch]; ok {
		for _, a := range aliases {
			if a != arch {
				archCandidates = append(archCandidates, a)
			}
		}
	}

	// 遍历所有资产，按优先级匹配：精确 ext+arch > 精确 ext+别名arch > 替代 ext+arch > 替代 ext+别名arch
	for _, e := range extCandidates {
		for _, a := range archCandidates {
			want := fmt.Sprintf("work_%s_%s_%s.%s", ver, osName, a, e)
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
		}
	}

	return nil, fmt.Errorf("未找到当前平台 (%s/%s) 的安装包", osName, arch)
}

func fetchReleaseByTag(ctx context.Context, client *http.Client, repo, tag string) (*releaseInfo, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, fmt.Errorf("版本号不能为空")
	}
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	info, err := fetchReleaseResponse(ctx, client, "releases/tags/"+tag, repo, fmt.Sprintf("获取版本 %s 失败", tag))
	if err != nil {
		return nil, err
	}
	if info.TagName == "" {
		info.TagName = tag
	}
	return info, nil
}
