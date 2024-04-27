package handlers

import (
	"context"
	"encoding/json"

	"github.com/google/go-github/v60/github"
	"github.com/instructlab/instructlab-bot/gobot/common"
	"github.com/instructlab/instructlab-bot/gobot/util"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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

	if event.GetRepo().GetName() != common.RepoName {
		h.Logger.Warnf("Received unexpected event %s from %s/%s repo. Skipping the event.",
			eventType, event.GetOrganization().GetLogin(), event.GetRepo().GetName())
		return nil
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
		CheckName: common.BotReadyStatus,
		RepoOwner: repoOwner,
		RepoName:  repoName,
		PrNum:     prNum,
		PrSha:     prSha,
	}

	// Check if the bot readiness status already exists
	enable, err := util.StatusExist(ctx, client, params, common.BotReadyStatus)
	if err != nil {
		h.Logger.Errorf("Failed to check bot enable status: %v", err)
		return nil
	}
	if enable {
		return nil
	}

	err = util.PostBotWelcomeMessage(ctx, client, repoOwner, repoName, prNum, prSha, h.BotUsername, h.Maintainers)
	if err != nil {
		h.Logger.Errorf("Failed to post bot welcome message on PR %s/%s#%d: %v", repoOwner, repoName, prNum, err)
		return err
	}
	return nil
}
