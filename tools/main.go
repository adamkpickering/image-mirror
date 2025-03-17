package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/rancher/image-mirror/pkg/config"
	"github.com/rancher/image-mirror/pkg/regsync"
	"github.com/urfave/cli/v2"
)

const regsyncYamlPath = "regsync.yaml"

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
			Registry: targetRepository.Registry,
			User:     targetRepository.Username,
			Pass:     targetRepository.Password,
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
func convertConfigImageToRegsyncImages(repo config.Repository, image config.Image) ([]regsync.ConfigSync, error) {
	targetImageName := image.TargetImageName
	if targetImageName == "" {
		parts := strings.Split(image.SourceImage, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("source image split into %d parts (>=2 parts expected)", len(parts))
		}
		repoName := parts[len(parts)-2]
		imageName := parts[len(parts)-1]
		targetImageName = "mirrored-" + repoName + "-" + imageName
	}

	entries := make([]regsync.ConfigSync, 0, len(image.Tags))
	for _, tag := range image.Tags {
		sourceImage := image.SourceImage + ":" + tag
		targetImage := repo.BaseUrl + "/" + targetImageName + ":" + tag
		entry := regsync.ConfigSync{
			Source: sourceImage,
			Target: targetImage,
			Type:   "image",
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
