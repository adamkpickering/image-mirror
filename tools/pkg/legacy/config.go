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

// Returns true if any of the entries' Images fields contains
// the passed image.
func (c Config) Contains(image string) bool {
	for _, configEntry := range c {
		for _, configImage := range configEntry.Images {
			if configImage == image {
				return true
			}
		}
	}
	return false
}
