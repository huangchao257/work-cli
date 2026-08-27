package cli

import (
	"os"

	"gopkg.in/yaml.v3"

	"github.com/huangchao257/work-cli/internal/platform"
)

// apiTimeoutOptions 对应 ~/.work/config.yaml 的 api 段。
type apiTimeoutOptions struct {
	API struct {
		Timeout string `yaml:"timeout"`
	} `yaml:"api"`
}

func loadAPITimeoutFromConfig() (apiTimeoutOptions, error) {
	var options apiTimeoutOptions
	path, err := platform.ConfigFilePath()
	if err != nil {
		return options, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return options, nil
	}
	_ = yaml.Unmarshal(data, &options)
	return options, nil
}
