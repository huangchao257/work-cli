// Package config 提供 ~/.work/config.yaml 的权威读写入口。
//
// 基于 gopkg.in/yaml.v3 的 yaml.Node 进行点分路径导航、设值与删除，
// 保留原有注释与键顺序。本包不重构既有 selfupdate/source 各自的配置读取，
// 作为新的统一读写入口，后续可渐进合并。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/huangchao257/work-cli/internal/configcache"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/usage"
)

func errUsage(format string, a ...any) error {
	return usage.Newf(format, a...)
}

// Path 返回 ~/.work/config.yaml 的绝对路径。
func Path() (string, error) {
	return platform.ConfigFilePath()
}

// Load 读取并解析配置文件，返回根 mapping node。
// 文件不存在时返回空 mapping node（不报错），便于 get/list 输出空。
func Load() (*yaml.Node, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := configcache.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyMapping(), nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root := doc.Content[0]
		if root.Kind == yaml.MappingNode {
			return root, nil
		}
	}
	// 空文件或顶层非 mapping：返回空 mapping，避免丢失已有数据时误判。
	return emptyMapping(), nil
}

// Save 将根 mapping node 写回配置文件，自动创建父目录，权限 0600，UTF-8/LF。
func Save(root *yaml.Node) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	data, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("编码配置失败: %w", err)
	}
	if err := os.WriteFile(p, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	configcache.Invalidate(p)
	return nil
}

// Get 按点分路径取值。scalar 返回原值；mapping/sequence 返回 YAML 文本。
// 找不到键时返回 ("", false, nil)，不报错。
// api_key 类键自动脱敏：仅返回 [已设置] 或 [未设置]。
func Get(key string) (string, bool, error) {
	if err := validateKey(key); err != nil {
		return "", false, err
	}
	root, err := Load()
	if err != nil {
		return "", false, err
	}
	cur := root
	parts := strings.Split(key, ".")
	for i, seg := range parts {
		v, _ := findValue(cur, seg)
		if v == nil {
			return "", false, nil
		}
		if i == len(parts)-1 {
			val := renderValue(v)
			return redactSecretValue(key, val), true, nil
		}
		if v.Kind != yaml.MappingNode {
			// 路径穿越非 mapping 节点：视为键不存在。
			return "", false, nil
		}
		cur = v
	}
	return "", false, nil
}

// redactSecretValue 对含 api_key 后缀的键值进行脱敏，返回 [已设置] 或 [未设置]。
func redactSecretValue(key, val string) string {
	if strings.HasSuffix(key, ".api_key") || strings.HasSuffix(key, ".API_KEY") {
		if strings.TrimSpace(val) == "" {
			return "[未设置]"
		}
		return "[已设置]"
	}
	return val
}

// Set 按点分路径设值，按需创建中间 mapping 节点。
// value 做类型推断：true/false→bool，整数→int，[a,b] 或逗号分隔→sequence，否则字符串。
// dryRun 为 true 时仅校验键合法，不写盘。
func Set(key, value string, dryRun bool) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if dryRun {
		return nil
	}
	root, err := Load()
	if err != nil {
		return err
	}
	if err := setNode(root, key, value); err != nil {
		return err
	}
	return Save(root)
}

// Unset 按点分路径删除叶子键。键不存在时为幂等无操作。
// dryRun 为 true 时仅校验键合法，不写盘。
func Unset(key string, dryRun bool) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if dryRun {
		return nil
	}
	root, err := Load()
	if err != nil {
		return err
	}
	if err := unsetNode(root, key); err != nil {
		return err
	}
	return Save(root)
}

// List 将配置展平为点分键→值字符串。
// scalar 叶子取原值；sequence 叶子取 YAML 文本；mapping 节点递归展平。
func List() (map[string]string, error) {
	root, err := Load()
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	flatten(root, "", out)
	return out, nil
}

// --- 内部辅助 ---

func emptyMapping() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

// findValue 在 mapping node 的 Content（[key,value,...] 配对）中查找键，
// 返回值节点与其在 Content 中的下标（值位置），未找到返回 (nil, -1)。
func findValue(m *yaml.Node, key string) (*yaml.Node, int) {
	if m.Kind != yaml.MappingNode {
		return nil, -1
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1], i + 1
		}
	}
	return nil, -1
}

func validateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errUsage("键不能为空")
	}
	for _, seg := range strings.Split(key, ".") {
		if seg == "" {
			return errUsage("键路径包含空段: %s", key)
		}
	}
	// 拒绝控制字符与前后空白，避免 YAML 键名异常
	for _, r := range key {
		if r < 0x20 && r != '\t' {
			return errUsage("键包含非法控制字符: %q", key)
		}
	}
	if key != strings.TrimSpace(key) {
		return errUsage("键首尾不能有空白: %q", key)
	}
	return nil
}

func setNode(root *yaml.Node, key, value string) error {
	cur := root
	parts := strings.Split(key, ".")
	for i, seg := range parts {
		isLeaf := i == len(parts)-1
		if isLeaf {
			vn := buildValueNode(value)
			if old, idx := findValue(cur, seg); idx >= 0 {
				// 保留旧值节点上的注释（行内/脚注/头注释），仅替换值。
				vn.HeadComment = old.HeadComment
				vn.LineComment = old.LineComment
				vn.FootComment = old.FootComment
				cur.Content[idx] = vn
			} else {
				kn := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: seg}
				cur.Content = append(cur.Content, kn, vn)
			}
			return nil
		}
		// 中间段：取或创建子 mapping
		child, _ := findValue(cur, seg)
		if child == nil {
			nm := emptyMapping()
			kn := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: seg}
			cur.Content = append(cur.Content, kn, nm)
			cur = nm
			continue
		}
		if child.Kind != yaml.MappingNode {
			return errUsage("键路径冲突: %s 已存在且非映射", seg)
		}
		cur = child
	}
	return nil
}

func unsetNode(root *yaml.Node, key string) error {
	cur := root
	parts := strings.Split(key, ".")
	for i, seg := range parts {
		v, idx := findValue(cur, seg)
		if idx < 0 {
			return nil // 幂等：键不存在
		}
		if i == len(parts)-1 {
			// 移除 key/value 配对（idx-1 为键，idx 为值）
			cur.Content = append(cur.Content[:idx-1], cur.Content[idx+1:]...)
			return nil
		}
		if v.Kind != yaml.MappingNode {
			return errUsage("键路径冲突: %s 已存在且非映射", seg)
		}
		cur = v
	}
	return nil
}

func flatten(n *yaml.Node, prefix string, out map[string]string) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		key := k.Value
		if prefix != "" {
			key = prefix + "." + key
		}
		switch v.Kind {
		case yaml.MappingNode:
			flatten(v, key, out)
		case yaml.SequenceNode:
			out[key] = redactSecretValue(key, renderValue(v))
		default:
			out[key] = redactSecretValue(key, v.Value)
		}
	}
}

// renderValue 渲染节点为字符串：scalar 取原值，其它序列化为 YAML 文本（去尾换行）。
func renderValue(v *yaml.Node) string {
	if v.Kind == yaml.ScalarNode {
		return v.Value
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		return v.Value
	}
	return strings.TrimRight(string(out), "\n")
}

// buildValueNode 按类型推断构造值节点。
// 仅 [a,b] 流式序列格式转换为 sequence node；其他值按 bool/int/str 推断。
func buildValueNode(raw string) *yaml.Node {
	raw = strings.TrimSpace(raw)
	// 形如 [a,b] 的流式序列：用 yaml 解析为 sequence node。
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		var doc yaml.Node
		if err := yaml.Unmarshal([]byte(raw), &doc); err == nil {
			if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 && doc.Content[0].Kind == yaml.SequenceNode {
				return doc.Content[0]
			}
		}
	}
	return scalarNode(raw)
}

// scalarNode 构造类型化标量节点（bool/int/str）。
func scalarNode(raw string) *yaml.Node {
	if raw == "true" || raw == "false" {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: raw}
	}
	if _, err := strconv.Atoi(raw); err == nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: raw}
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: raw}
}
