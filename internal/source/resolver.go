package source

import (
	"fmt"
)

func Resolve(ref Ref) (string, error) {
	switch ref.Kind {
	case KindLocal:
		return ResolveLocal(ref.Local)
	case KindGit:
		cfg, err := LoadUserConfig()
		if err != nil {
			return "", err
		}
		cache, err := CacheDir(cfg)
		if err != nil {
			return "", err
		}
		return ResolveGit(ref.GitURL, ref.GitRef, cache)
	case KindRegistry:
		cfg, err := LoadUserConfig()
		if err != nil {
			return "", err
		}
		return ResolveRegistry(ref.Name, cfg)
	default:
		return "", fmt.Errorf("未知来源类型")
	}
}
