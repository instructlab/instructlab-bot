package util

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v60/github"
)

const (
	CheckComplete                 = "completed"
	CheckQueued                   = "queued"
	CheckInProgress               = "in_progress"
	CheckStatusSuccess            = "success"
	CheckStatusFailure            = "failure"
	CheckStatusError              = "error"
	CheckStatusPending            = "pending"

	TriageReadinessCheck = "Triage Readiness"
	PrecheckCheck        = "Precheck"
	GenerateLocalCheck   = "Generate Local"
	GenerateSDGCheck     = "Generate SDG Status"
)

type PullRequestStatusParams struct {
	Status       string
	Conclusion   string
	CheckName    string
	CheckSummary string
	CheckDetails string
	Comment      string

	JobType string
	JobID   string
	JobErr  string

	RepoOwner string
	RepoName  string
	PrNum     int
	PrSha     string
}

func PostPullRequestErrorComment(ctx context.Context, client *github.Client,  params PullRequestStatusParams, err error) error {
	params.Comment = fmt.Sprintf("Beep, boop 🤖  Sorry, An error has occurred : %v", err)
	return PostPullRequestComment(ctx, client, params)
}

func PostPullRequestComment(ctx context.Context, client *github.Client, params PullRequestStatusParams) error {
	comment := &github.IssueComment{
		Body: &params.Comment,
	}
	if _, _, err := client.Issues.CreateComment(ctx, params.RepoOwner, params.RepoName, int(params.PrNum), comment); err != nil {
		return fmt.Errorf("failed to comment on the pending job: %w", err)
	}
	return nil
}

func PostPullRequestCheck(ctx context.Context, client *github.Client, params PullRequestStatusParams) error {

	checkRequest := github.CreateCheckRunOptions{
		Name:       params.CheckName,
		HeadSHA:    params.PrSha,
		Status:     github.String(params.Status),
		StartedAt:  &github.Timestamp{Time: time.Now()}, // Optional
		Output: &github.CheckRunOutput{
			Title:   github.String(params.CheckName),
			Summary: github.String(params.CheckSummary),
			Text:    github.String(params.CheckDetails),
		},
	}

	if params.Conclusion != "" {
		checkRequest.Conclusion = github.String(params.Conclusion)
		checkRequest.CompletedAt = &github.Timestamp{Time: time.Now().Add(time.Duration(40))}

	}

	_, _, err := client.Checks.CreateCheckRun(ctx, params.RepoOwner, params.RepoName, checkRequest)
	if err != nil {
		return err
	}
	return nil
}
