package handlers

import (
	"context"

	"github.com/palantir/go-githubapp/githubapp"
	"go.uber.org/zap"
)

type PullRequestHandler struct {
	githubapp.ClientCreator
	Logger         *zap.SugaredLogger
	RequiredLabels []string
	BotUsername    string
}

func (h *PullRequestHandler) Handles() []string {
	return []string{"pull_request"}
}

func (h *PullRequestHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	return nil
}
