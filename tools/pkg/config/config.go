package config

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"
)

type Config struct {
	Images       []*Image
	Repositories []Repository
}

// Image should not be instantiated directly. Instead, use NewImage().
type Image struct {
	// The source image without any tags.
	SourceImage            string
	defaultTargetImageName string
	// Used to specify the desired name of the target image if it differs
	// from default. This field would be private if it was convenient for
	// marshalling to JSON/YAML, but it is not. This field should not be
	// accessed directly - instead, use the TargetImageName() and
	// SetTargetImageName() methods.
	SpecifiedTargetImageName string `json:"TargetImageName,omitempty"`
	// The tags that we want to mirror.
	Tags []string
}

type Repository struct {
	BaseUrl      string
	EnvVarPrefix string
	// Whether the repository should have images mirrored to it.
	Target bool
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

	for _, image := range config.Images {
		if err := image.setDefaults(); err != nil {
			return Config{}, fmt.Errorf("failed to set defaults for image %q: %w", image, err)
		}
	}

	return config, nil
}

func Write(fileName string, config Config) error {
	config.Sort()

	contents, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to unmarshal as JSON: %w", err)
	}

	if err := os.WriteFile(fileName, contents, 0o644); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	return nil
}

func (config *Config) Sort() {
	for _, image := range config.Images {
		image.Sort()
	}
	slices.SortStableFunc(config.Images, compareImages)
	slices.SortStableFunc(config.Repositories, compareRepositories)
}

func compareImages(a, b *Image) int {
	if sourceImageValue := strings.Compare(a.SourceImage, b.SourceImage); sourceImageValue != 0 {
		return sourceImageValue
	}
	return strings.Compare(a.TargetImageName(), b.TargetImageName())
}

func compareRepositories(a, b Repository) int {
	if sourceImageValue := strings.Compare(a.BaseUrl, b.BaseUrl); sourceImageValue != 0 {
		return sourceImageValue
	}
	return strings.Compare(a.EnvVarPrefix, b.EnvVarPrefix)
}

func NewImage(sourceImage string, tags []string) (*Image, error) {
	image := &Image{
		SourceImage: sourceImage,
		Tags:        tags,
	}
	if err := image.setDefaults(); err != nil {
		return nil, err
	}
	return image, nil
}
func (image *Image) Sort() {
	slices.Sort(image.Tags)
}

func (image *Image) setDefaults() error {
	parts := strings.Split(image.SourceImage, "/")
	if len(parts) < 2 {
		return fmt.Errorf("source image split into %d parts (>=2 parts expected)", len(parts))
	}
	repoName := parts[len(parts)-2]
	imageName := parts[len(parts)-1]
	image.defaultTargetImageName = "mirrored-" + repoName + "-" + imageName
	return nil
}

func (image Image) TargetImageName() string {
	if image.SpecifiedTargetImageName != "" {
		return image.SpecifiedTargetImageName
	}
	return image.defaultTargetImageName
}

func (image *Image) SetTargetImageName(value string) {
	if value == image.defaultTargetImageName {
		image.SpecifiedTargetImageName = ""
	} else {
		image.SpecifiedTargetImageName = value
	}
}
