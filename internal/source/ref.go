package source

import (
	"fmt"
	"strings"
)

type Kind int

const (
	KindRegistry Kind = iota
	KindGit
	KindLocal
)

type Ref struct {
	Kind   Kind
	Raw    string
	Name   string
	GitURL string
	GitRef string
	Local  string
}

func ParseRef(raw string) (Ref, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Ref{}, fmt.Errorf("安装引用不能为空")
	}
	if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "../") {
		return Ref{Kind: KindLocal, Raw: raw, Local: raw}, nil
	}
	if strings.HasPrefix(raw, "git:") {
		rest := strings.TrimPrefix(raw, "git:")
		at := strings.LastIndex(rest, "@")
		if at <= 0 {
			return Ref{}, fmt.Errorf("git 引用格式应为 git:host/org/repo@ref")
		}
		url := "https://" + rest[:at]
		ref := rest[at+1:]
		return Ref{Kind: KindGit, Raw: raw, GitURL: url, GitRef: ref}, nil
	}
	return Ref{Kind: KindRegistry, Raw: raw, Name: raw}, nil
}
