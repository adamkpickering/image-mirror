package autoupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/rancher/image-mirror/internal/config"
	"github.com/rancher/image-mirror/internal/git"
	"github.com/rancher/image-mirror/internal/paths"
	"github.com/rancher/image-mirror/internal/regsync"

	"github.com/google/go-github/v71/github"
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

	for _, entry := range config {
		if err := entry.Validate(); err != nil {
			return nil, fmt.Errorf("entry %q failed validation: %w", entry.Name, err)
		}
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

func (entry ConfigEntry) Validate() error {
	if entry.Name == "" {
		return errors.New("must specify Name")
	}
	if entry.GithubLatestRelease == nil {
		return errors.New("must specify an autoupdate strategy")
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

func (entry ConfigEntry) AutoUpdate(ctx context.Context, configYaml config.Config, dryRun bool) error {
	ghClient := github.NewClient(nil)
	if !dryRun {
		githubToken := os.Getenv("GITHUB_TOKEN")
		if githubToken == "" {
			return errors.New("must define GITHUB_TOKEN")
		}
		ghClient = ghClient.WithAuthToken(githubToken)
	}

	value := os.Getenv("GITHUB_REPOSITORY")
	if value == "" {
		return errors.New("must define GITHUB_REPOSITORY")
	}
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return errors.New("must define GITHUB_REPOSITORY in form <owner>/<repo>")
	}
	githubOrg := parts[0]
	githubRepo := parts[1]

	latestImages, err := entry.GetLatestImages()
	if err != nil {
		return fmt.Errorf("failed to get latest images for %s: %w", entry.Name, err)
	}

	accumulator := config.NewImageAccumulator()
	accumulator.AddImages(configYaml.Images...)

	imagesToUpdate := make([]*config.Image, 0, len(entry.GithubLatestRelease.Images))
	for _, latestImage := range latestImages {
		if !accumulator.Contains(latestImage) {
			imagesToUpdate = append(imagesToUpdate, latestImage)
		}
	}
	if len(imagesToUpdate) == 0 {
		fmt.Printf("%s: no updates found\n", entry.Name)
		return nil
	}

	tagName := latestImages[0].Tags[0]
	branchName := fmt.Sprintf("autoupdate/%s/%s", entry.Name, tagName)

	// When filtering pull requests by head branch, the github API
	// requires that the head branch must be in the format <owner>:<branch>.
	// In the case of branches pushed using GITHUB_TOKEN, owner is
	// "rancher". When running in a personal repo, setting GITHUB_TOKEN
	// to a PAT makes owner the same as the user's github username.
	headBranch := githubOrg + ":" + branchName
	requestContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pullRequests, _, err := ghClient.PullRequests.List(requestContext, githubOrg, githubRepo, &github.PullRequestListOptions{
		Head:  headBranch,
		State: "all",
	})
	if err != nil {
		return fmt.Errorf("failed to list pull requests: %w", err)
	}
	if len(pullRequests) == 1 {
		fmt.Printf("%s: found existing PR with head branch %s: %s\n", entry.Name, headBranch, pullRequests[0].GetHTMLURL())
		return nil
	} else if len(pullRequests) > 1 {
		pullRequestString := ""
		for _, pullRequest := range pullRequests {
			pullRequestString = pullRequestString + "\n- " + pullRequest.GetHTMLURL()
		}
		fmt.Printf("%s: warning: found multiple existing PRs with head branch %s:%s\n", entry.Name, headBranch, pullRequestString)
		return nil
	}

	if dryRun {
		fmt.Printf("%s: would make PR under branch %s\n", entry.Name, branchName)
		return nil
	}

	if err := git.CreateAndCheckout(branchName); err != nil {
		return fmt.Errorf("failed to create and checkout branch %s: %w", branchName, err)
	}
	for _, imageToUpdate := range imagesToUpdate {
		// We can reuse the accumulator here because we are making a sequence
		// of commits, each of which makes an addition from imagesToUpdate.
		accumulator.AddImages(imageToUpdate)
		configYaml.Images = accumulator.Images()
		if err := config.Write(paths.ConfigYaml, configYaml); err != nil {
			return fmt.Errorf("failed to write %s: %w", paths.ConfigYaml, err)
		}

		regsyncYaml, err := configYaml.ToRegsyncConfig()
		if err != nil {
			return fmt.Errorf("failed to generate regsync config for commit for image %s: %w", imageToUpdate.SourceImage, err)
		}
		if err := regsync.WriteConfig(paths.RegsyncYaml, regsyncYaml); err != nil {
			return fmt.Errorf("failed to write regsync config for commit for image %s: %w", imageToUpdate.SourceImage, err)
		}

		msg := fmt.Sprintf("Add tag %s for image %s", tagName, imageToUpdate.SourceImage)
		if err := git.Commit(msg); err != nil {
			return fmt.Errorf("failed to commit changes for image %s: %w", imageToUpdate.SourceImage, err)
		}
	}
	if err := git.PushBranch(branchName, "origin"); err != nil {
		return fmt.Errorf("failed to push branch %s: %w", branchName, err)
	}

	title := fmt.Sprintf("[autoupdate] Add tag `%s` for `%s`", tagName, entry.Name)
	body := fmt.Sprintf("This PR was created by the autoupdate workflow.\n\nIt adds the tag `%s` for the following images:", tagName)
	for _, imageToUpdate := range imagesToUpdate {
		body = body + "\n- `" + imageToUpdate.SourceImage + "`"
	}
	baseBranch := "master"
	maintainerCanModify := true
	newPullRequest := &github.NewPullRequest{
		Base:                &baseBranch,
		Head:                &branchName,
		Title:               &title,
		Body:                &body,
		MaintainerCanModify: &maintainerCanModify,
	}

	requestContext, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pullRequest, _, err := ghClient.PullRequests.Create(requestContext, githubOrg, githubRepo, newPullRequest)
	if err != nil {
		return fmt.Errorf("failed to create pull request for tag %s: %w", tagName, err)
	}
	fmt.Printf("%s: created pull request for tag %s: %s\n", entry.Name, tagName, pullRequest.GetHTMLURL())

	return nil
}
