package git

import (
	"fmt"
	"os/exec"
)

func IsWorkingTreeClean() (bool, error) {
	cmd := exec.Command("git", "diff", "--quiet")
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		} else {
			return false, fmt.Errorf("failed to run git diff: %w", err)
		}
	}
	return true, nil
}

func CreateAndCheckout(branchName string) error {
	cmd := exec.Command("git", "checkout", "-b", branchName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run git checkout: %w", err)
	}
	return nil
}

func Commit(msg string) error {
	cmd := exec.Command("git", "commit", "--all", "--message", msg)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run git commit: %w", err)
	}
	return nil
}

func PushBranch(branchName, remote string) error {
	cmd := exec.Command("git", "push", remote, branchName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run git push: %w", err)
	}
	return nil
}
