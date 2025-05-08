package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rancher/image-mirror/internal/autoupdate"
	"github.com/rancher/image-mirror/internal/config"
	"github.com/rancher/image-mirror/internal/git"
	"github.com/rancher/image-mirror/internal/legacy"
	"github.com/rancher/image-mirror/internal/paths"
	"github.com/rancher/image-mirror/internal/regsync"

	"github.com/urfave/cli/v3"
)

var dryRun bool

func main() {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config-path",
				Aliases:     []string{"c"},
				Value:       "config.yaml",
				Usage:       "Path to config.yaml file",
				Destination: &paths.ConfigYaml,
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "autoupdate",
				Usage:  "Update config.yaml according to autoupdate.yaml",
				Action: autoUpdate,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:        "dry-run",
						Aliases:     []string{"n"},
						Usage:       "Only print what would be done",
						Destination: &dryRun,
					},
					&cli.StringFlag{
						Name:        "entry",
						Aliases:     []string{"e"},
						Usage:       "Autoupdate specific entry instead of all",
						Destination: &entryName,
					},
				},
			},
			{
				Name:   "format",
				Usage:  "Enforce formatting on certain files",
				Action: formatFiles,
			},
			{
				Name:   "generate-regsync",
				Usage:  "Generate regsync.yaml",
				Action: generateRegsyncYaml,
			},
			{
				Name:   "migrate-images-list",
				Usage:  "Migrate images from images-list to config.yaml",
				Action: migrateImagesList,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "images-list-path",
						Value:       "images-list",
						Usage:       "Path to images list file",
						Destination: &paths.ImagesList,
					},
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

// generateRegsyncYaml regenerates the regsync config file from the current state
// of config.yaml.
func generateRegsyncYaml(_ context.Context, _ *cli.Command) error {
	configYaml, err := config.Parse(paths.ConfigYaml)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", paths.ConfigYaml, err)
	}

	regsyncYaml, err := configYaml.ToRegsyncConfig()
	if err != nil {
		return err
	}

	if err := regsync.WriteConfig(paths.RegsyncYaml, regsyncYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", paths.RegsyncYaml, err)
	}

	return nil
}

func migrateImagesList(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 2 {
		return fmt.Errorf("must pass source and target image")
	}
	sourceImage := cmd.Args().Get(0)
	targetImage := cmd.Args().Get(1)

	configYaml, err := config.Parse(paths.ConfigYaml)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	accumulator := config.NewImageAccumulator()
	accumulator.AddImages(configYaml.Images...)

	imagesListComment, legacyImages, err := legacy.ParseImagesList(paths.ImagesList)
	if err != nil {
		return fmt.Errorf("failed to parse images list: %w", err)
	}

	configJson, err := legacy.ParseConfig(paths.ConfigJson)
	if err != nil {
		return fmt.Errorf("failed to parse %q: %w", paths.ConfigJson, err)
	}

	if configJson.Contains(sourceImage) {
		fmt.Printf("warning: %s refers to image with source %q\n", paths.ConfigJson, sourceImage)
	}

	legacyImagesToKeep := make([]legacy.ImagesListEntry, 0, len(legacyImages))
	for _, legacyImage := range legacyImages {
		if legacyImage.Source == sourceImage && legacyImage.Target == targetImage {
			newImage, err := convertImageListEntryToImage(legacyImage)
			if err != nil {
				return fmt.Errorf("failed to convert %q: %w", legacyImage, err)
			}
			accumulator.AddImages(newImage)
		} else {
			legacyImagesToKeep = append(legacyImagesToKeep, legacyImage)
			continue
		}
	}

	// set config.Images to accumulated images and write config
	configYaml.Images = accumulator.Images()
	if err := config.Write(paths.ConfigYaml, configYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", paths.ConfigYaml, err)
	}

	// write kept legacy images
	if err := legacy.WriteImagesList(paths.ImagesList, imagesListComment, legacyImagesToKeep); err != nil {
		return fmt.Errorf("failed to write %s: %w", paths.ImagesList, err)
	}

	return nil
}

func convertImageListEntryToImage(imageListEntry legacy.ImagesListEntry) (*config.Image, error) {
	parts := strings.Split(imageListEntry.Target, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("failed to split %q into 2 parts", imageListEntry.Target)
	}
	targetImageName := parts[len(parts)-1]

	image, err := config.NewImage(imageListEntry.Source, []string{imageListEntry.Tag})
	if err != nil {
		return nil, fmt.Errorf("failed to create new Image: %w", err)
	}
	image.SetTargetImageName(targetImageName)

	return image, nil
}

func formatFiles(_ context.Context, _ *cli.Command) error {
	configYaml, err := config.Parse(paths.ConfigYaml)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", paths.ConfigYaml, err)
	}
	if err := config.Write(paths.ConfigYaml, configYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", paths.ConfigYaml, err)
	}

	autoUpdateYaml, err := autoupdate.Parse(paths.AutoUpdateYaml)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", paths.AutoUpdateYaml, err)
	}
	if err := autoupdate.Write(paths.AutoUpdateYaml, autoUpdateYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", paths.AutoUpdateYaml, err)
	}

	return nil
}

func autoUpdate(ctx context.Context, _ *cli.Command) error {
	if !dryRun {
		if clean, err := git.IsWorkingTreeClean(); err != nil {
			return fmt.Errorf("failed to get status of working tree: %w", err)
		} else if !clean {
			return errors.New("working tree or index has changes")
		}
	}

	configYaml, err := config.Parse(paths.ConfigYaml)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", paths.ConfigYaml, err)
	}

	autoUpdateEntries, err := autoupdate.Parse(paths.AutoUpdateYaml)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", paths.AutoUpdateYaml, err)
	}

	for _, autoUpdateEntry := range autoUpdateEntries {
		if err := autoUpdateEntry.AutoUpdate(ctx, configYaml, dryRun); err != nil {
			fmt.Printf("%s: error: %s\n", autoUpdateEntry.Name, err)
			continue
		}
	}

	return nil
}
