package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v60/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type PullRequestHandler struct {
	githubapp.ClientCreator
	Logger        *zap.SugaredLogger
	RequiredLabel string
	BotUsername   string
}

func (h *PullRequestHandler) Handles() []string {
	return []string{"pull_request"}
}

func (h *PullRequestHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	var event github.PullRequestEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "failed to parse issue comment event payload")
	}

	if event.GetAction() != "opened" {
		return nil
	}

	installID := githubapp.GetInstallationIDFromEvent(&event)
	repo := event.GetRepo()
	repoOwner := repo.GetOwner().GetLogin()
	repoName := repo.GetName()
	prNum := event.GetPullRequest().GetNumber()

	client, err := h.NewInstallationClient(installID)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("Beep, boop 🤖 Hi, I'm %s and I'm going to help you"+
		" with your pull request. Thanks for you contribution! 🎉\n\n", h.BotUsername)
	if h.RequiredLabel != "" {
		msg += fmt.Sprintf("> [!NOTE]\n"+
			"> Before you are able to use the bot's commands, it must be triaged "+
			"and have the `%s` label applied to it.\n\n", h.RequiredLabel)
	}
	msg += fmt.Sprintf("I support the following commands:\n\n"+
		"* `%s precheck` -- Check existing model behavior using the questions in this proposed change.\n"+
		"* `%s generate` -- Generate a sample of synthetic data using the synthetic data generation backend infrastructure.\n"+
		"* `%s generate-local` -- Generate a sample of synthetic data using a local model.\n",
		h.BotUsername, h.BotUsername, h.BotUsername)
	botComment := github.IssueComment{
		Body: &msg,
	}

	if _, _, err := client.Issues.CreateComment(ctx, repoOwner, repoName, prNum, &botComment); err != nil {
		h.Logger.Error("Failed to comment on pull request: %w", err)
	}

	return nil
}
