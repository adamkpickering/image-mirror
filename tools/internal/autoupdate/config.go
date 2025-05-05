package autoupdate

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/rancher/image-mirror/internal/config"

	"sigs.k8s.io/yaml"
)

type ConfigEntry struct {
	Name                string
	GithubLatestRelease *GithubLatestRelease
}

func Parse(filePath string) ([]ConfigEntry, error) {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	config := make([]ConfigEntry, 0)
	if err := yaml.UnmarshalStrict(contents, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return config, nil
}

type GithubLatestRelease struct {
	Owner      string
	Repository string
	Images     []string
}
