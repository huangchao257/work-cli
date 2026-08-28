package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveMaven(meta registryResponse, registryURL string, cache string) (string, error) {
	if meta.Version == "" {
		return "", fmt.Errorf("maven 类型缺少 version 字段")
	}
	if err := validatePathComponent(meta.Name); err != nil {
		return "", fmt.Errorf("registry 返回非法 bundle 名称: %w", err)
	}
	if err := validatePathComponent(meta.Version); err != nil {
		return "", fmt.Errorf("registry 返回非法版本号: %w", err)
	}
	dest := filepath.Join(cache, "registry", meta.Name, meta.Version)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	coords := meta.DownloadURL
	if coords == "" {
		coords = meta.Name
	}
	parts := strings.Split(coords, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("maven 坐标格式无效 %q，期望 groupId:artifactId:version", coords)
	}
	groupID := parts[0]
	artifactID := parts[1]
	version := parts[2]
	groupPath := strings.ReplaceAll(groupID, ".", "/")
	jarURL := strings.TrimRight(registryURL, "/") + "/" + groupPath + "/" + artifactID + "/" + version + "/" + artifactID + "-" + version + ".jar"
	jarPath := filepath.Join(cache, "registry", meta.Name, meta.Version+".jar")
	if strings.TrimSpace(meta.Checksum) == "" {
		return "", fmt.Errorf("registry 响应缺少 checksum（sha256），拒绝安装")
	}
	// 任一步失败都清理 dest：半成品目录残留会被 Stat 永久命中，无法自愈。
	if err := func() (err error) {
		defer func() {
			if err != nil {
				_ = os.RemoveAll(dest)
			}
		}()
		if err := os.MkdirAll(filepath.Dir(jarPath), 0o755); err != nil {
			return fmt.Errorf("创建缓存目录失败: %w", err)
		}
		if err := downloadFile(jarURL, jarPath); err != nil {
			return fmt.Errorf("下载 maven 归档失败: %w", err)
		}
		if err := verifyChecksumRequired(jarPath, meta.Checksum); err != nil {
			return err
		}
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return fmt.Errorf("创建缓存目录失败: %w", err)
		}
		if err := unzip(jarPath, dest); err != nil {
			return fmt.Errorf("解压 maven 归档失败: %w", err)
		}
		return nil
	}(); err != nil {
		return "", err
	}
	return dest, nil
}
