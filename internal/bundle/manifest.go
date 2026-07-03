// Package bundle 提供 Skills/Rules/MCP 资源套装（bundle.yaml）的解析与校验。

package bundle

type Manifest struct {
	Type        string       `yaml:"type"`
	Name        string       `yaml:"name"`
	Version     string       `yaml:"version"`
	Description string       `yaml:"description"`
	Env         []EnvVar     `yaml:"env"`
	Resources   Resources    `yaml:"resources"`
	Targets     []string     `yaml:"targets"`
	PostInstall *PostInstall `yaml:"post_install"`
}

// PostInstall runs after a successful bundle install (optional).
type PostInstall struct {
	WhenScope string `yaml:"when_scope"` // project | user | any (default project)
	Action    string `yaml:"action"`     // graph_init
}

type EnvVar struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type Resources struct {
	Skills []SkillResource `yaml:"skills"`
	Rules  []RuleResource  `yaml:"rules"`
	MCP    []MCPResource   `yaml:"mcp"`
}

type SkillResource struct {
	ID     string `yaml:"id"`
	Source string `yaml:"source"`
}

type RuleResource struct {
	ID     string   `yaml:"id"`
	Source string   `yaml:"source"`
	Apply  string   `yaml:"apply"`
	Globs  []string `yaml:"globs"`
}

type MCPResource struct {
	ID     string              `yaml:"id"`
	Source string              `yaml:"source"`
	Env    []map[string]string `yaml:"env"`
}
