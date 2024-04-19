package util

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
)

const (
	CheckComplete      = "completed"
	CheckQueued        = "queued"
	CheckInProgress    = "in_progress"
	CheckStatusSuccess = "success"
	CheckStatusFailure = "failure"
	CheckStatusError   = "error"
	CheckStatusPending = "pending"

	TriageReadinessCheck = "Triage Readiness Check"
	PrecheckCheck        = "Precheck Check"
	GenerateLocalCheck   = "Generate Local Check"
	GenerateSDGCheck     = "Generate SDG Check"

	TriageReadinessStatus = "Triage Readiness Status"
	PrecheckStatus        = "Precheck Status"
	GenerateLocalStatus   = "Generate Local Status"
	GenerateSDGStatus     = "Generate SDG Status"
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

func StatusExist(ctx context.Context, client *github.Client, params PullRequestStatusParams, statusName string) (bool, error) {
	repoStatus, response, err := client.Repositories.ListStatuses(ctx, params.RepoOwner, params.RepoName, params.PrSha, nil)
	if err != nil {
		return false, err
	}

	if response.StatusCode == http.StatusOK {
		for _, status := range repoStatus {
			if strings.HasSuffix(status.GetURL(), params.PrSha) {
				if status.GetContext() == statusName {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func PostPullRequestErrorComment(ctx context.Context, client *github.Client, params PullRequestStatusParams, err error) error {
	params.Comment = fmt.Sprintf("Beep, boop 🤖  Sorry, An error has occurred : %v", err)
	return PostPullRequestComment(ctx, client, params)
}

func PostPullRequestComment(ctx context.Context, client *github.Client, params PullRequestStatusParams) error {
	comment := &github.IssueComment{
		Body: &params.Comment,
	}
	if _, _, err := client.Issues.CreateComment(ctx, params.RepoOwner, params.RepoName, int(params.PrNum), comment); err != nil {
		return err
	}
	return nil
}

func PostPullRequestCheck(ctx context.Context, client *github.Client, params PullRequestStatusParams) error {

	checkRequest := github.CreateCheckRunOptions{
		Name:      params.CheckName,
		HeadSHA:   params.PrSha,
		Status:    github.String(params.Status),
		StartedAt: &github.Timestamp{Time: time.Now()}, // Optional
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
