package handlers

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v60/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type PRCommentHandler struct {
	githubapp.ClientCreator
	Logger        *zap.SugaredLogger
	RedisHostPort string
}

type PRComment struct {
	repoOwner string
	repoName  string
	prNum     int
	author    string
	body      string
	installID int64
}

func (h *PRCommentHandler) Handles() []string {
	return []string{"issue_comment"}
}

func (h *PRCommentHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	var event github.IssueCommentEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "failed to parse issue comment event payload")
	}

	if !event.GetIssue().IsPullRequest() {
		return nil
	}

	if event.GetAction() != "created" {
		return nil
	}

	repo := event.GetRepo()
	prComment := PRComment{
		repoOwner: repo.GetOwner().GetLogin(),
		repoName:  repo.GetName(),
		prNum:     event.GetIssue().GetNumber(),
		author:    event.GetComment().GetUser().GetLogin(),
		body:      event.GetComment().GetBody(),
		installID: githubapp.GetInstallationIDFromEvent(&event),
	}

	client, err := h.NewInstallationClient(prComment.installID)
	if err != nil {
		return err
	}

	words := strings.Fields(strings.TrimSpace(prComment.body))
	if len(words) < 2 {
		return nil
	}
	if words[0] != "@instruct-lab-bot" {
		return nil
	}
	switch words[1] {
	case "generate":
		err = h.generateCommand(ctx, client, &prComment)
		if err != nil {
			h.reportError(ctx, client, &prComment, err)
		}
		return err
	default:
		return h.unknownCommand(ctx, client, &prComment)
	}
}

func (h *PRCommentHandler) reportError(ctx context.Context, client *github.Client, prComment *PRComment, err error) {
	h.Logger.Errorf("Error processing command on %s/%s#%d by %s: %v",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author, err)

	msg := "Beep, boop 🤖  Sorry, An error has occurred."
	botComment := github.IssueComment{
		Body: &msg,
	}

	if _, _, err := client.Issues.CreateComment(ctx, prComment.repoOwner, prComment.repoName, prComment.prNum, &botComment); err != nil {
		h.Logger.Error("Failed to comment on pull request: %w", err)
	}
}

func (h *PRCommentHandler) generateCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("Generate command received on %s/%s#%d by %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)

	r := redis.NewClient(&redis.Options{
		Addr:     h.RedisHostPort,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	jobNumber, err := r.Incr(ctx, "jobs").Result()
	if err != nil {
		return err
	}

	err = r.Set(ctx, "jobs:"+strconv.FormatInt(jobNumber, 10)+":pr_number", prComment.prNum, 0).Err()
	if err != nil {
		return err
	}

	err = r.Set(ctx, "jobs:"+strconv.FormatInt(jobNumber, 10)+":installation_id", prComment.installID, 0).Err()
	if err != nil {
		return err
	}

	err = r.Set(ctx, "jobs:"+strconv.FormatInt(jobNumber, 10)+":repo_owner", prComment.repoOwner, 0).Err()
	if err != nil {
		return err
	}

	err = r.Set(ctx, "jobs:"+strconv.FormatInt(jobNumber, 10)+":repo_name", prComment.repoName, 0).Err()
	if err != nil {
		return err
	}

	err = r.LPush(ctx, "generate", strconv.FormatInt(jobNumber, 10)).Err()
	if err != nil {
		return err
	}
	msg := "Beep, boop 🤖  Generating test data for your pull request.\n\n" +
		"This will take several minutes...\n\n" +
		"Your job ID is " + strconv.FormatInt(jobNumber, 10) + "."
	botComment := github.IssueComment{
		Body: &msg,
	}

	if _, _, err := client.Issues.CreateComment(ctx, prComment.repoOwner, prComment.repoName, prComment.prNum, &botComment); err != nil {
		h.Logger.Error("Failed to comment on pull request: %w", err)
		return err
	}

	return nil
}

func (h *PRCommentHandler) unknownCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("Unknown command received on %s/%s#%d by %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)

	msg := "Beep, boop 🤖  Sorry, I don't understand that command"
	botComment := github.IssueComment{
		Body: &msg,
	}

	if _, _, err := client.Issues.CreateComment(ctx, prComment.repoOwner, prComment.repoName, prComment.prNum, &botComment); err != nil {
		h.Logger.Error("Failed to comment on pull request: %w", err)
		return err
	}

	return nil
}
