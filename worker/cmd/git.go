package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.uber.org/zap"
)

// gitOperations handles the git operations for a job and returns the head hash for the PR
func (w *Worker) gitOperations(logger *zap.SugaredLogger, taxonomyDir string, prNumber string) (string, error) {
	logger.Debug("Opening taxonomy git repo")

	var r *git.Repository
	var err error

	// Check if the taxonomy directory exists, clone it if it does not
	if _, err := os.Stat(taxonomyDir); os.IsNotExist(err) {
		logger.Debugf("Taxonomy directory does not exist, cloning from %s", GitRemote)
		r, err = git.PlainClone(taxonomyDir, false, &git.CloneOptions{
			URL: GitRemote,
			Auth: &githttp.BasicAuth{
				Username: GithubUsername,
				Password: GithubToken,
			},
		})
		if err != nil {
			return "", fmt.Errorf("could not clone taxonomy git repo: %v", err)
		}
	} else {
		// Open the existing taxonomy directory as a git repository
		r, err = git.PlainOpen(taxonomyDir)
		if err != nil {
			return "", fmt.Errorf("could not open taxonomy git repo: %v", err)
		}
	}

	// Fetch updates from the remote repository
	if err := fetchUpdates(r, logger); err != nil {
		return "", err
	}

	// Checkout the main branch
	if err := checkoutBranch(r, "main", logger); err != nil {
		return "", err
	}

	prBranchName := fmt.Sprintf("pr-%s", prNumber)
	// Delete the PR branch if it exists
	if err := deleteBranch(r, prBranchName, logger); err != nil {
		return "", err
	}

	// Fetch and checkout the PR branch
	if err := fetchAndCheckoutPRBranch(r, prNumber, prBranchName, logger); err != nil {
		return "", err
	}

	// Get the current head commit hash
	head, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("could not get HEAD: %v", err)
	}

	return head.Hash().String(), nil
}

// fetchUpdates fetches updates from the remote repository
func fetchUpdates(r *git.Repository, sugar *zap.SugaredLogger) error {
	for attempt := 1; attempt <= gitMaxRetries; attempt++ {
		sugar.Debug("Fetching from origin")
		err := r.Fetch(&git.FetchOptions{
			RemoteName: "origin",
			Auth: &githttp.BasicAuth{
				Username: GithubUsername,
				Password: GithubToken,
			},
		})
		if err == nil || err == git.NoErrAlreadyUpToDate {
			return nil
		}
		if attempt < gitMaxRetries {
			sugar.Infof("Retrying fetching updates, attempt %d/%d", attempt+1, gitMaxRetries)
			time.Sleep(gitRetryDelay)
		} else {
			return fmt.Errorf("could not fetch from origin: %v", err)
		}
	}
	return nil
}

// checkoutBranch checks out the specified branch
func checkoutBranch(r *git.Repository, branchName string, logger *zap.SugaredLogger) error {
	wt, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("could not get worktree: %v", err)
	}

	for attempt := 1; attempt <= gitMaxRetries; attempt++ {
		err := wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.ReferenceName(fmt.Sprintf("refs/remotes/origin/%s", branchName)),
		})
		if err == nil {
			return nil
		}
		if attempt < gitMaxRetries {
			logger.Infof("Retrying checkout of branch '%s', attempt %d/%d", branchName, attempt+1, gitMaxRetries)
			time.Sleep(gitRetryDelay)
		} else {
			return fmt.Errorf("could not checkout branch '%s' after retries: %v", branchName, err)
		}
	}
	return nil
}

// deleteBranch deletes the specified branch if it exists
func deleteBranch(r *git.Repository, branchName string, logger *zap.SugaredLogger) error {
	_, err := r.Branch(branchName)
	if err == nil {
		err = r.DeleteBranch(branchName)
		if err != nil {
			return fmt.Errorf("could not delete branch '%s': %v", branchName, err)
		}
	}
	return nil
}

// fetchAndCheckoutPRBranch fetches and checks out the PR branch
func fetchAndCheckoutPRBranch(r *git.Repository, prNumber, prBranchName string, logger *zap.SugaredLogger) error {
	logger = logger.With("pr_branch_name", prBranchName)
	logger.Debug("Fetching PR branch")
	refspec := gitconfig.RefSpec(fmt.Sprintf("refs/pull/%s/head:refs/heads/%s", prNumber, prBranchName))
	err := r.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		RefSpecs:   []gitconfig.RefSpec{refspec},
		Auth: &githttp.BasicAuth{
			Username: "instructlab-bot",
			Password: GithubToken,
		},
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("could not fetch PR branch: %v", err)
	}

	wt, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("could not get worktree: %v", err)
	}

	logger.Debug("Checking out PR branch")
	if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(prBranchName)}); err != nil {
		return fmt.Errorf("could not checkout PR branch: %v", err)
	}

	return nil
}

// deleteTaxonomyDir deletes the taxonomy directory and its contents
func deleteTaxonomyDir(taxonomyDir string) error {
	if err := os.RemoveAll(taxonomyDir); err != nil {
		return fmt.Errorf("could not delete taxonomy directory: %v", err)
	}
	return nil
}
