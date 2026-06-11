package state

import "time"

type File struct {
	Bundles []BundleRecord `json:"bundles"`
}

type BundleRecord struct {
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	Version        string          `json:"version"`
	Scope          string          `json:"scope"`
	Ref            string          `json:"ref"`
	InstalledAt    time.Time       `json:"installed_at"`
	IDEs           []string        `json:"ides,omitempty"`
	Resources      BundleResources `json:"resources,omitempty"`
	InstallCommand string          `json:"install_command,omitempty"`
}

type BundleResources struct {
	Skills []string `json:"skills"`
	Rules  []string `json:"rules"`
	MCP    []string `json:"mcp"`
}
