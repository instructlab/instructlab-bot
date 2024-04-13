package util

import (
	"context"
	"fmt"

	"github.com/google/go-github/v60/github"
)

func PostPullRequestError(ctx context.Context, client *github.Client, params PullRequestStatusParams) error {
	botComment := github.IssueComment{
		Body: &params.JobErr,
	}

	// Post the job error as a new comment
	if _, _, err := client.Issues.CreateComment(ctx, params.RepoOwner, params.RepoName, params.PrNum, &botComment); err != nil {
		return fmt.Errorf("failed to comment the error on pull request: %w", err)
	}

	// This message is short due to limited viewable width in the status box
	errStatusMsg := fmt.Sprintf("See bot message for error details for job id: %s", params.JobID)

	status := &github.RepoStatus{
		State:       github.String(params.State),
		Description: github.String(errStatusMsg),
		Context:     github.String(params.StatusCtx),
	}

	// Update the PR status to an error state
	_, _, err := client.Repositories.CreateStatus(ctx, params.RepoOwner, params.RepoName, params.PrSha, status)
	if err != nil {
		return err
	}
	return nil
}
