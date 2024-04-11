package util

import (
	"context"

	"github.com/google/go-github/v60/github"
)

const (
	Success = "success"
	Failure = "failure"
	Error   = "error"
	Pending = "pending"
	InstructLabMaintainersTeamUrl = "https://github.com/instruct-lab/community/blob/main/MAINTAINERS.md"
	TriageReadinessStatus = "Triage Readiness Status"
	PrecheckStatus = "Precheck Status"
	GenerateLocalStatus = "Generate Local Status"
	GenerateSDGStatus = "Generate SDG Status"

)

func PostPullRequestStatus(ctx context.Context, client *github.Client, state string, msg string, statusCtx string,
	targetUrl string, repoOwner string, repoName string, prSha string) error {

	status := &github.RepoStatus{
		State:       github.String(state),		// Status state: success, failure, error, or pending
		Description: github.String(msg), 		// Status description
		Context:     github.String(statusCtx), 		// Status context
		TargetURL:   github.String(targetUrl),		// Target URL to redirect
	}

	_, _, err := client.Repositories.CreateStatus(ctx, repoOwner, repoName, prSha, status)
	if err != nil {
		return err
	}
	return nil
}
