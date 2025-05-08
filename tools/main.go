package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rancher/image-mirror/internal/autoupdate"
	"github.com/rancher/image-mirror/internal/config"
	"github.com/rancher/image-mirror/internal/legacy"
	"github.com/rancher/image-mirror/internal/regsync"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v71/github"
	"github.com/urfave/cli/v3"
)

const regsyncYamlPath = "regsync.yaml"
const configJsonPath = "retrieve-image-tags/config.json"
const autoUpdateYamlPath = "autoupdate.yaml"

var configYamlPath string
var imagesListPath string

func main() {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config-path",
				Aliases:     []string{"c"},
				Value:       "config.yaml",
				Usage:       "Path to config.yaml file",
				Destination: &configYamlPath,
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "auto-update",
				Usage:  "Update config.yaml according to autoupdate.yaml",
				Action: autoUpdate,
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
						Destination: &imagesListPath,
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
	cfg, err := config.Parse(configYamlPath)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", configYamlPath, err)
	}

	regsyncYaml := regsync.Config{
		Creds: make([]regsync.ConfigCred, 0, len(cfg.Repositories)),
		Defaults: regsync.ConfigDefaults{
			UserAgent: "rancher-image-mirror",
		},
		Sync: make([]regsync.ConfigSync, 0),
	}
	for _, targetRepository := range cfg.Repositories {
		credEntry := regsync.ConfigCred{
			Pass:          targetRepository.Password,
			Registry:      targetRepository.Registry,
			ReqConcurrent: targetRepository.ReqConcurrent,
			User:          targetRepository.Username,
		}
		regsyncYaml.Creds = append(regsyncYaml.Creds, credEntry)
	}
	for _, image := range cfg.Images {
		if image.DoNotMirror {
			continue
		}
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

func migrateImagesList(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 2 {
		return fmt.Errorf("must pass source and target image")
	}
	sourceImage := cmd.Args().Get(0)
	targetImage := cmd.Args().Get(1)

	configYaml, err := config.Parse(configYamlPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	accumulator := config.NewImageAccumulator()
	accumulator.AddImages(configYaml.Images...)

	imagesListComment, legacyImages, err := legacy.ParseImagesList(imagesListPath)
	if err != nil {
		return fmt.Errorf("failed to parse images list: %w", err)
	}

	configJson, err := legacy.ParseConfig(configJsonPath)
	if err != nil {
		return fmt.Errorf("failed to parse %q: %w", configJsonPath, err)
	}

	if configJson.Contains(sourceImage) {
		fmt.Printf("warning: %s refers to image with source %q\n", configJsonPath, sourceImage)
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
	if err := config.Write(configYamlPath, configYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", configYamlPath, err)
	}

	// write kept legacy images
	if err := legacy.WriteImagesList(imagesListPath, imagesListComment, legacyImagesToKeep); err != nil {
		return fmt.Errorf("failed to write %s: %w", imagesListPath, err)
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
	configYaml, err := config.Parse(configYamlPath)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", configYamlPath, err)
	}
	if err := config.Write(configYamlPath, configYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", configYamlPath, err)
	}

	autoUpdateYaml, err := autoupdate.Parse(autoUpdateYamlPath)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", autoUpdateYamlPath, err)
	}
	if err := autoupdate.Write(autoUpdateYamlPath, autoUpdateYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", autoUpdateYamlPath, err)
	}

	return nil
}

func autoUpdate(ctx context.Context, _ *cli.Command) error {
	if clean, err := isWorkingTreeClean(); err != nil {
		return fmt.Errorf("failed to get status of working tree: %w", err)
	} else if !clean {
		return errors.New("working tree or index has changes")
	}

	configYaml, err := config.Parse(configYamlPath)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", configYamlPath, err)
	}

	autoUpdateConfig, err := autoupdate.Parse(autoUpdateYamlPath)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", autoUpdateYamlPath, err)
	}

	for _, entry := range autoUpdateConfig {
		latestImages, err := entry.GetLatestImages()
		if err != nil {
			fmt.Printf("Failed to get latest images for %s: %s\n", entry.Name, err)
			continue
		}

		accumulator := config.NewImageAccumulator()
		accumulator.AddImages(configYaml.Images...)

		needToUpdate := false
		for _, latestImage := range latestImages {
			if !accumulator.Contains(latestImage) {
				needToUpdate = true
				break
			}
		}
		if !needToUpdate {
			fmt.Printf("No updates found for %s\n", entry.Name)
		}

		// this might need to be changed depending on the update strategy
		tagName := latestImages[0].Tags[0]
		branchName := fmt.Sprintf("auto-update/%s/%s", entry.Name, tagName)

		ghClient := github.NewClient(nil)
		pullRequests, _, err := ghClient.PullRequests.List(ctx, "rancher", "image-mirror", &github.PullRequestListOptions{
			Head:  branchName,
			State: "all",
		})
		if err != nil {
			fmt.Printf("Failed to list pull requests for %s: %s", entry.Name, err)
			continue
		}
		if len(pullRequests) == 1 {
			fmt.Printf("Found existing PR for %s tag %s: %s", entry.Name, tagName, *pullRequests[0].URL)
			continue
		} else if len(pullRequests) > 1 {
			pullRequestString := ""
			for _, pullRequest := range pullRequests {
				pullRequestString = pullRequestString + "\n\t" + *pullRequest.URL
			}
			fmt.Printf("Warning: found multiple existing PRs for %s tag %s:%s", entry.Name, tagName, pullRequestString)
			continue
		}

		// TODO: this happens later
		accumulator.AddImages(latestImages...)

		configYaml.Images = accumulator.Images()
		if err := config.Write(configYamlPath, configYaml); err != nil {
			return fmt.Errorf("failed to write %s: %w", configYamlPath, err)
		}
	}

	return nil
}

func isWorkingTreeClean() (bool, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return false, fmt.Errorf("failed to open git repository: %w", err)
	}
	workingTree, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to get working tree: %w", err)
	}
	workingTreeStatus, err := workingTree.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get status of workingtree: %w", err)
	}
	return workingTreeStatus.IsClean(), nil
}
