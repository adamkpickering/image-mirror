package config

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/rancher/image-mirror/internal/regsync"

	"sigs.k8s.io/yaml"
)

type Config struct {
	Images       []*Image
	Repositories []Repository
}

// Image should not be instantiated directly. Instead, use NewImage().
type Image struct {
	// If true, the Image is not mirrored i.e. it is not added to the
	// regsync config when the regsync config is generated.
	DoNotMirror bool `json:",omitempty"`
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
	// ReqConcurrent is what goes into the "reqConcurrent" field of
	// regsync.yaml for this repository. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	ReqConcurrent int
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

	for _, image := range config.Images {
		if err := image.setDefaults(); err != nil {
			return Config{}, fmt.Errorf("failed to set defaults for image %q: %w", image.SourceImage, err)
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

func (config *Config) ToRegsyncConfig() (regsync.Config, error) {
	regsyncYaml := regsync.Config{
		Creds: make([]regsync.ConfigCred, 0, len(config.Repositories)),
		Defaults: regsync.ConfigDefaults{
			UserAgent: "rancher-image-mirror",
		},
		Sync: make([]regsync.ConfigSync, 0),
	}
	for _, targetRepository := range config.Repositories {
		credEntry := regsync.ConfigCred{
			Pass:          targetRepository.Password,
			Registry:      targetRepository.Registry,
			ReqConcurrent: targetRepository.ReqConcurrent,
			User:          targetRepository.Username,
		}
		regsyncYaml.Creds = append(regsyncYaml.Creds, credEntry)
	}
	for _, image := range config.Images {
		if image.DoNotMirror {
			continue
		}
		for _, repo := range config.Repositories {
			if !repo.Target {
				continue
			}
			// Do not mirror images to the repository they come from
			if strings.HasPrefix(image.SourceImage, repo.BaseUrl) {
				continue
			}
			syncEntries, err := convertConfigImageToRegsyncImages(repo, image)
			if err != nil {
				return regsync.Config{}, fmt.Errorf("failed to convert Image with SourceImage %q: %w", image.SourceImage, err)
			}
			regsyncYaml.Sync = append(regsyncYaml.Sync, syncEntries...)
		}
	}
	return regsyncYaml, nil
}

// convertConfigImageToRegsyncImages converts image into one ConfigSync (i.e. an
// image for regsync to sync) for each tag present in image. repo provides the
// target repository for each ConfigSync.
func convertConfigImageToRegsyncImages(repo Repository, image *Image) ([]regsync.ConfigSync, error) {
	entries := make([]regsync.ConfigSync, 0, len(image.Tags))
	for _, tag := range image.Tags {
		sourceImage := image.SourceImage + ":" + tag
		targetImage := repo.BaseUrl + "/" + image.TargetImageName() + ":" + tag
		entry := regsync.ConfigSync{
			Source: sourceImage,
			Target: targetImage,
			Type:   "image",
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func compareImages(a, b *Image) int {
	if sourceImageValue := strings.Compare(a.SourceImage, b.SourceImage); sourceImageValue != 0 {
		return sourceImageValue
	}
	return strings.Compare(a.TargetImageName(), b.TargetImageName())
}

func compareRepositories(a, b Repository) int {
	return strings.Compare(a.BaseUrl, b.BaseUrl)
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

	if parts[0] == "dp.apps.rancher.io" {
		// AppCo images have only one significant part in their reference.
		// For example, in dp.apps.rancher.io/containers/openjdk,
		// dp.apps.rancher.io/containers is the repository and openjdk is
		// the significant part.
		imageName := parts[len(parts)-1]
		image.defaultTargetImageName = "appco-" + imageName
	} else {
		repoName := parts[len(parts)-2]
		imageName := parts[len(parts)-1]
		image.defaultTargetImageName = "mirrored-" + repoName + "-" + imageName
	}
	return nil
}

func (image *Image) TargetImageName() string {
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
