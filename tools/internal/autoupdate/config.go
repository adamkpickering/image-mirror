package autoupdate

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

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

func Write(filePath string, config []ConfigEntry) error {
	slices.SortStableFunc(config, func(a, b ConfigEntry) int {
		return strings.Compare(a.Name, b.Name)
	})

	contents, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	if err := os.WriteFile(filePath, contents, 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (entry ConfigEntry) GetLatestImages() ([]*config.Image, error) {
	switch {
	case entry.GithubLatestRelease != nil:
		return entry.GithubLatestRelease.GetLatestImages()
	default:
		return nil, errors.New("did not find update strategy")
	}
}
