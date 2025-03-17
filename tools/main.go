package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/rancher/image-mirror/pkg/config"
	"github.com/rancher/image-mirror/pkg/legacy"
	"github.com/rancher/image-mirror/pkg/regsync"
	"github.com/urfave/cli/v2"
)

const regsyncYamlPath = "regsync.yaml"
const configJsonPath = "retrieve-image-tags/config.json"

var configYamlPath string

func main() {
	log.SetFlags(0)

	app := cli.NewApp()
	app.Commands = []*cli.Command{
		{
			Name:   "generate-regsync",
			Usage:  "Generate regsync.yaml",
			Action: generateRegsyncYaml,
			Flags: []cli.Flag{
				&cli.PathFlag{
					Name:        "config-path",
					Aliases:     []string{"c"},
					Value:       "config.yaml",
					Usage:       "Path to config.yaml file",
					Destination: &configYamlPath,
				},
			},
		},
		{
			Name:   "migrate-images-list",
			Usage:  "Moves images-list entries not mentioned in config.json to config.yaml",
			Action: migrateImagesList,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// generateRegsyncYaml regenerates the regsync config file from the current state
// of config.yaml.
func generateRegsyncYaml(ctx *cli.Context) error {
	cfg, err := config.Parse(configYamlPath)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", configYamlPath, err)
	}

	regsyncYaml := regsync.Config{
		Creds: make([]regsync.ConfigCred, 0, len(cfg.Repositories)),
		Sync:  make([]regsync.ConfigSync, 0),
	}
	for _, targetRepository := range cfg.Repositories {
		credEntry := regsync.ConfigCred{
			Registry: fmt.Sprintf(`{{ env "%s_ENDPOINT" }}`, targetRepository.EnvVarPrefix),
			User:     fmt.Sprintf(`{{ env "%s_USERNAME" }}`, targetRepository.EnvVarPrefix),
			Pass:     fmt.Sprintf(`{{ env "%s_PASSWORD" }}`, targetRepository.EnvVarPrefix),
		}
		regsyncYaml.Creds = append(regsyncYaml.Creds, credEntry)
	}
	for _, image := range cfg.Images {
		for _, repo := range cfg.Repositories {
			if !repo.Target {
				continue
			}
			syncEntries, err := convertConfigImageToRegsyncImages(repo, image)
			if err != nil {
				return fmt.Errorf("failed to convert Image with SourceImage %q: %w", image.SourceImage, err)
			}
			regsyncYaml.Sync = append(regsyncYaml.Sync, syncEntries...)
		}
	}

	if err := regsync.WriteConfig(regsyncYamlPath, regsyncYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", regsyncYamlPath, err)
	}

	return nil
}

// convertConfigImageToRegsyncImages converts image into one ConfigSync (i.e. an
// image for regsync to sync) for each tag present in image. repo provides the
// target repository for each ConfigSync.
func convertConfigImageToRegsyncImages(repo config.Repository, image *config.Image) ([]regsync.ConfigSync, error) {
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

func migrateImagesList(ctx *cli.Context) error {
	imagesListPath := ctx.Args().Get(0)
	if imagesListPath == "" {
		return fmt.Errorf("must pass path to images list file as argument")
	}

	configYaml, err := config.Parse(configYamlPath)
	if err != nil {
		return fmt.Errorf("failed to parse %q: %w", configYamlPath, err)
	}
	accumulator := config.NewImageAccumulator()
	for _, existingImage := range configYaml.Images {
		accumulator.AddImage(*existingImage)
	}

	legacyImages, err := legacy.ParseImagesList(imagesListPath)
	if err != nil {
		return fmt.Errorf("failed to parse %q: %w", imagesListPath, err)
	}

	configJson, err := legacy.ParseConfig(configJsonPath)
	if err != nil {
		return fmt.Errorf("failed to parse %q: %w", configJsonPath, err)
	}

	for _, legacyImage := range legacyImages {
		// if image in config.json, skip
		if configJson.Contains(legacyImage.Source) {
			continue
		}
		newImage, err := convertImageListEntryToImage(legacyImage)
		if err != nil {
			return fmt.Errorf("failed to convert %q: %w", legacyImage, err)
		}
		accumulator.AddImage(newImage)
	}

	// set config.Images to accumulated images and write config
	configYaml.Images = accumulator.Images()
	if err := config.Write(configYamlPath, configYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", configYamlPath, err)
	}

	return nil
}

func convertImageListEntryToImage(imageListEntry legacy.ImagesListEntry) (config.Image, error) {
	parts := strings.Split(imageListEntry.Target, "/")
	if len(parts) < 2 {
		return config.Image{}, fmt.Errorf("failed to split %q into 2 parts", imageListEntry.Target)
	}
	targetImageName := parts[len(parts)-1]

	image := config.Image{
		SourceImage: imageListEntry.Source,
		Tags: []string{
			imageListEntry.Tag,
		},
	}
	image.SetTargetImageName(targetImageName)

	return image, nil
}
