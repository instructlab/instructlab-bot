package util

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/instructlab/instructlab-bot/gobot/common"
)

type PullRequestStatusParams struct {
	Status       string
	Conclusion   string
	CheckName    string
	CheckSummary string
	CheckDetails string
	Comment      string
	StatusDesc   string

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

func PostPullRequestStatus(ctx context.Context, client *github.Client, params PullRequestStatusParams) error {
	status := &github.RepoStatus{
		State:       github.String(params.Conclusion),        // Status state: success, failure, error, or pending
		Description: github.String(params.StatusDesc),        // Status description
		Context:     github.String(params.CheckName),         // Status context
		TargetURL:   github.String(common.InstructLabBotUrl), // Target URL to redirect
	}
	_, _, err := client.Repositories.CreateStatus(ctx, params.RepoOwner, params.RepoName, params.PrSha, status)
	if err != nil {
		return err
	}
	return nil
}

func PostBotWelcomeMessage(ctx context.Context, client *github.Client, repoOwner string, repoName string, prNum int, prSha string, botName string, maintainers []string) error {
	params := PullRequestStatusParams{
		CheckName: common.BotReadyStatus,
		RepoOwner: repoOwner,
		RepoName:  repoName,
		PrNum:     prNum,
		PrSha:     prSha,
	}
	detailsMsg := fmt.Sprintf("Beep, boop 🤖, Hi, I'm %s and I'm going to help you"+
		" with your pull request. Thanks for you contribution! 🎉\n\n", botName)
	detailsMsg += fmt.Sprintf("I support the following commands:\n\n"+
		"* `%s precheck` -- Check existing model behavior using the questions in this proposed change.\n"+
		"* `%s generate` -- Generate a sample of synthetic data using the synthetic data generation backend infrastructure.\n"+
		"* `%s generate-local` -- Generate a sample of synthetic data using a local model.\n"+
		"* `%s help` -- Print this help message again.\n"+
		"> [!NOTE] \n > **Results or Errors of these commands will be posted as a pull request check in the Checks section below**\n\n",
		botName, botName, botName, botName)

	if len(maintainers) > 0 {
		detailsMsg += fmt.Sprintf("> [!NOTE] \n > **Currently only maintainers belongs to [%v] teams are allowed to run these commands**.\n", maintainers)
	}
	params.Status = common.CheckComplete
	params.Conclusion = common.CheckStatusSuccess
	params.CheckSummary = common.BotReadyStatusMsg
	params.CheckDetails = detailsMsg
	params.Comment = detailsMsg
	params.StatusDesc = common.BotReadyStatusMsg

	err := PostPullRequestComment(ctx, client, params)
	if err != nil {
		return err
	}

	err = PostPullRequestStatus(ctx, client, params)
	if err != nil {
		return err
	}
	return nil

}
