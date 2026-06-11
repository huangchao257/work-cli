package engine

type Result struct {
	Success       bool     `json:"success"`
	Name          string   `json:"name,omitempty"`
	Kind          string   `json:"kind,omitempty"`
	Version       string   `json:"version,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	InstalledIDEs []string `json:"installed_ides,omitempty"`
	SkippedIDEs   []string `json:"skipped_ides,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	FilesWritten  []string `json:"files_written,omitempty"`
	Commands      []string `json:"commands,omitempty"`
	DryRun        bool     `json:"dry_run,omitempty"`
	Message       string   `json:"message,omitempty"`
}

type ListResult struct {
	Items []ListItem `json:"items"`
}

type ListItem struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Version        string `json:"version"`
	Scope          string `json:"scope"`
	Ref            string `json:"ref"`
	InstalledAt    string `json:"installed_at"`
	IDEs           []string `json:"ides,omitempty"`
	InstallCommand string `json:"install_command,omitempty"`
}
