package hooks

type Manifest struct {
	Type        string        `yaml:"type"`
	Name        string        `yaml:"name"`
	Version     string        `yaml:"version"`
	Description string        `yaml:"description"`
	Env         []EnvVar      `yaml:"env"`
	Telemetry   TelemetrySpec `yaml:"telemetry"`
	Resources   HookResources `yaml:"resources"`
	Targets     []string      `yaml:"targets"`
}

type EnvVar struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type TelemetrySpec struct {
	Preset string   `yaml:"preset"`
	Events []string `yaml:"events"`
	Redact []string `yaml:"redact"`
}

type HookResources struct {
	Hooks []HookResource `yaml:"hooks"`
}

type HookResource struct {
	ID     string `yaml:"id"`
	Source string `yaml:"source"`
}
