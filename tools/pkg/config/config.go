package config

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

type Config struct {
	Images       []Image
	Repositories []Repository
}

type Image struct {
	// The source image without any tags. For example: rancher/rancher
	// or ghcr.io/banzaicloud/fluentd.
	SourceImage string
	// Used to specify the desired name of the image on the target. If it
	// is not specified, defaults to mirrored-<repoName>-<imageName>.
	TargetImageName string
	// The tags that we want to mirror.
	Tags []string
}

type Repository struct {
	// BaseUrl is used exclusively for building the target image ref
	// for a given image for a repository. For example, a target
	// image name of "mirrored-rancher-cis-operator" and a BaseUrl
	// of "docker.io/rancher" produce a target image ref of
	// "docker.io/rancher/mirrored-rancher-cis-operator".
	BaseUrl string
	// Whether the repository should have images mirrored to it.
	Target bool
	// Password is what goes into the "pass" field of regsync.yaml
	// for this repository. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	Password string
	// Registry is what goes into the "registry" field of regsync.yaml
	// for this repository. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	Registry string
	// Username is what goes into the "user" field of regsync.yaml
	// for this repository. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	Username string
}

func Parse(fileName string) (Config, error) {
	contents, err := os.ReadFile(fileName)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read: %w", err)
	}

	config := Config{}
	if err := yaml.Unmarshal(contents, &config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal as JSON: %w", err)
	}

	return config, nil
}
