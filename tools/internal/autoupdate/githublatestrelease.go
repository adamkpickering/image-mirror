package autoupdate

import (
	"context"
	"fmt"

	"github.com/rancher/image-mirror/internal/config"

	"github.com/google/go-github/v71/github"
)

type GithubLatestRelease struct {
	Owner      string
	Repository string
	Images     []string
}

func (strat *GithubLatestRelease) GetLatestImages() ([]*config.Image, error) {
	client := github.NewClient(nil)
	release, _, err := client.Repositories.GetLatestRelease(context.Background(), strat.Owner, strat.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}
	latestTag := *release.TagName

	images := make([]*config.Image, 0, len(strat.Images))
	for _, sourceImage := range strat.Images {
		image, err := config.NewImage(sourceImage, []string{latestTag})
		if err != nil {
			return nil, fmt.Errorf("failed to construct image from source image %q and tag %q: %w", sourceImage, latestTag, err)
		}
		images = append(images, image)
	}

	return images, nil
}
