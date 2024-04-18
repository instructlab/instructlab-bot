package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v60/github"
	"github.com/instructlab/instructlab-bot/gobot/util"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	TriageReadinessMsg = "PR is ready for evaluation."
)

type PullRequestHandler struct {
	githubapp.ClientCreator
	Logger         *zap.SugaredLogger
	RequiredLabels []string
	BotUsername    string
	Maintainers    []string
}

func (h *PullRequestHandler) Handles() []string {
	return []string{"pull_request"}
}

func (h *PullRequestHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	var event github.PullRequestEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "failed to parse issue comment event payload")
	}

	if event.GetAction() != "labeled" {
		return nil
	}

	installID := githubapp.GetInstallationIDFromEvent(&event)
	repo := event.GetRepo()
	repoOwner := repo.GetOwner().GetLogin()
	repoName := repo.GetName()
	prNum := event.GetPullRequest().GetNumber()
	prSha := event.GetPullRequest().GetHead().GetSHA()

	h.Logger.Infof("Checking for required labels: %v", h.RequiredLabels)
	if len(h.RequiredLabels) == 0 {
		return nil
	}

	labelFound, err := util.CheckRequiredLabel(event.GetPullRequest().Labels, h.RequiredLabels)
	if err != nil {
		h.Logger.Errorf("Failed to check required labels: %v", err)
	}

	if !labelFound {
		h.Logger.Infof("Required labels not found on PR, skipping posting welcome message #%d", prNum)
		return nil
	}

	client, err := h.NewInstallationClient(installID)
	if err != nil {
		h.Logger.Errorf("Failed to create installation client: %v", err)
		return err
	}

	params := util.PullRequestStatusParams{
		CheckName: util.TriageReadinessCheck,
		RepoOwner: repoOwner,
		RepoName:  repoName,
		PrNum:     prNum,
		PrSha:     prSha,
	}

	// Check if the triage readiness check already exists
	enable, err := util.CheckBotEnableStatus(ctx, client, params)
	if err != nil {
		h.Logger.Errorf("Failed to check bot enable status: %v", err)
		return nil
	}
	if enable {
		return nil
	}

	detailsMsg := fmt.Sprintf("Beep, boop 🤖, Hi, I'm %s and I'm going to help you"+
		" with your pull request. Thanks for you contribution! 🎉\n\n", h.BotUsername)
	detailsMsg += fmt.Sprintf("I support the following commands:\n\n"+
		"* `%s precheck` -- Check existing model behavior using the questions in this proposed change.\n"+
		"* `%s generate` -- Generate a sample of synthetic data using the synthetic data generation backend infrastructure.\n"+
		"* `%s generate-local` -- Generate a sample of synthetic data using a local model.\n"+
		"> [!NOTE] \n > **Results or Errors of these commands will be posted as a pull request check in the Checks section below**\n\n",
		h.BotUsername, h.BotUsername, h.BotUsername)

	if len(h.Maintainers) > 0 {
		detailsMsg += fmt.Sprintf("> [!NOTE] \n > **Currently only maintainers belongs to [%v] teams are allowed to run these commands**.\n", h.Maintainers)
	}

	params.Status = util.CheckComplete
	params.Conclusion = util.CheckStatusSuccess
	params.CheckSummary = TriageReadinessMsg
	params.CheckDetails = detailsMsg
	params.Comment = detailsMsg

	err = util.PostPullRequestComment(ctx, client, params)
	if err != nil {
		h.Logger.Errorf("Failed to post comment on PR %s/%s#%d: %v", params.RepoOwner, params.RepoName, params.PrNum, err)
		return err
	}

	err = util.PostPullRequestCheck(ctx, client, params)
	if err != nil {
		h.Logger.Errorf("Failed to post check on PR %s/%s#%d: %v", params.RepoOwner, params.RepoName, params.PrNum, err)
		return err
	}
	return nil
}
