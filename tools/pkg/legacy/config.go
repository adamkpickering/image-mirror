package legacy

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config map[string]ConfigEntry

type ConfigEntry struct {
	HelmCharts        map[string]interface{}
	ImageDenyList     []string
	Images            []string
	ImagesFilePath    string
	Latest            bool
	VersionConstraint string
	VersionFilter     string
	VersionSource     string
}

func ParseConfig(fileName string) (Config, error) {
	contents, err := os.ReadFile(fileName)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read: %w", err)
	}

	config := Config{}
	if err := json.Unmarshal(contents, &config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal as JSON: %w", err)
	}

	return config, nil
}
