package util

import (
	"context"
	"fmt"

	"github.com/google/go-github/v60/github"
)

const (
	Success                       = "success"
	Failure                       = "failure"
	Error                         = "error"
	Pending                       = "pending"
	InstructLabMaintainersTeamUrl = "https://github.com/instruct-lab/community/blob/main/MAINTAINERS.md"
	TriageReadinessStatus         = "Triage Readiness Status"
	PrecheckStatus                = "Precheck Status"
	GenerateLocalStatus           = "Generate Local Status"
	GenerateSDGStatus             = "Generate SDG Status"
)

type PullRequestStatusParams struct {
	State      string
	MsgStatus  string
	StatusCtx  string
	TargetURL  string
	RepoOwner  string
	RepoName   string
	PrSha      string
	PrNum      int
	JobType    string
	JobID      string
	MsgComment string
	JobErr     string
}

func PostPullRequestStatus(ctx context.Context, client *github.Client, params PullRequestStatusParams) error {
	// Post the job ID and job type as a new comment for the new pending job
	if params.State == "pending" {
		comment := &github.IssueComment{
			Body: &params.MsgComment,
		}
		if _, _, err := client.Issues.CreateComment(ctx, params.RepoOwner, params.RepoName, int(params.PrNum), comment); err != nil {
			return fmt.Errorf("failed to comment on the pending job: %w", err)
		}
	}

	status := &github.RepoStatus{
		State:       github.String(params.State),     // Status state: success, failure, error, or pending
		Description: github.String(params.MsgStatus), // Status description
		Context:     github.String(params.StatusCtx), // Status context
		TargetURL:   github.String(params.TargetURL), // Target URL to redirect
	}

	_, _, err := client.Repositories.CreateStatus(ctx, params.RepoOwner, params.RepoName, params.PrSha, status)
	if err != nil {
		return err
	}

	return nil
}
