package git

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

func IsWorkingTreeClean() (bool, error) {
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
		return false, fmt.Errorf("failed to get status of working tree: %w", err)
	}
	return workingTreeStatus.IsClean(), nil
}

func CreateAndCheckout(branchName string) error {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	masterReference, err := repo.Reference("refs/heads/master", true)
	if err != nil {
		return fmt.Errorf("failed to get master reference: %w", err)
	}
	opts := &git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
		Hash:   masterReference.Hash(),
	}
	if err := worktree.Checkout(opts); err != nil {
		return fmt.Errorf("failed to check out branch: %w", err)
	}
	return nil
}

func Commit(msg string) error {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	if _, err := worktree.Commit(msg, &git.CommitOptions{All: true}); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	return nil
}

func PushBranch(branchName, remote string) error {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}
	refSpec := fmt.Sprintf("refs/heads/%s:refs/heads/%s", branchName, branchName)
	opts := &git.PushOptions{
		RemoteName: remote,
		RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec(refSpec)},
	}
	if err := repo.Push(opts); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}
	return nil
}
