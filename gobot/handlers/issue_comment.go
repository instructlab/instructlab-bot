package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v60/github"
	"github.com/instruct-lab/instruct-lab-bot/gobot/util"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	TriageReadinessMsg = "PR is ready for evaluation."
)

type PRCommentHandler struct {
	githubapp.ClientCreator
	Logger         *zap.SugaredLogger
	RedisHostPort  string
	RequiredLabels []string
	BotUsername    string
	Maintainers    []string
}

type PRComment struct {
	repoOwner string
	repoName  string
	repoOrg   string
	prNum     int
	author    string
	body      string
	installID int64
	sha       string
	labels    []*github.Label
}

func (h *PRCommentHandler) Handles() []string {
	// PR comments come in as issue comments, not an independent event type.
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

	h.Logger.Debugf("Details of the event: %v", event)
	repo := event.GetRepo()
	prComment := PRComment{
		repoOwner: repo.GetOwner().GetLogin(),
		repoName:  repo.GetName(),
		repoOrg:   event.GetOrganization().GetLogin(),
		prNum:     event.GetIssue().GetNumber(),
		author:    event.GetComment().GetUser().GetLogin(),
		body:      event.GetComment().GetBody(),
		installID: githubapp.GetInstallationIDFromEvent(&event),
	}

	client, err := h.NewInstallationClient(prComment.installID)
	if err != nil {
		return err
	}
	// Fetch the PR sha and labels to avoid multiple Pull Request API calls
	pr, _, err := client.PullRequests.Get(ctx, prComment.repoOwner, prComment.repoName, prComment.prNum)
	if err != nil {
		h.Logger.Errorf("Failed to get pull request (%s/%s#%d) related to the issue comment: %w", prComment.repoOwner, prComment.repoName, prComment.prNum, err)
		return err
	}

	prComment.sha = pr.GetHead().GetSHA()
	prComment.labels = pr.Labels

	words := strings.Fields(strings.TrimSpace(prComment.body))
	if len(words) < 2 {
		return nil
	}
	if words[0] != h.BotUsername {
		return nil
	}
	switch words[1] {
	case "enable":
		err = h.enableCommand(ctx, client, &prComment)
		if err != nil {
			h.reportError(ctx, client, &prComment, err)
		}
		return err
	case "generate-local":
		err = h.generateCommand(ctx, client, &prComment)
		if err != nil {
			h.reportError(ctx, client, &prComment, err)
		}
		return err
	case "precheck":
		err = h.precheckCommand(ctx, client, &prComment)
		if err != nil {
			h.reportError(ctx, client, &prComment, err)
		}
		return err
	case "generate":
		err = h.sdgSvcCommand(ctx, client, &prComment)
		if err != nil {
			h.reportError(ctx, client, &prComment, err)
		}
		return err
	default:
		return h.unknownCommand(ctx, client, &prComment)
	}
}

func (h *PRCommentHandler) reportError(ctx context.Context, client *github.Client, prComment *PRComment, err error) {
	h.Logger.Warnf("Error processing command on %s/%s#%d by %s: %v",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author, err)

	msg := fmt.Sprintf("Beep, boop 🤖  Sorry, An error has occurred : %v", err)
	botComment := github.IssueComment{
		Body: &msg,
	}

	if _, _, err := client.Issues.CreateComment(ctx, prComment.repoOwner, prComment.repoName, prComment.prNum, &botComment); err != nil {
		h.Logger.Error("Failed to comment on pull request: %w", err)
	}
}

func (h *PRCommentHandler) checkRequiredLabel(ctx context.Context, client *github.Client, prComment *PRComment, requiredLabels []string) (bool, error) {
	if len(requiredLabels) == 0 {
		return true, nil
	}

	labelFound := false
	for _, required := range requiredLabels {
		for _, label := range prComment.labels {
			if label.GetName() == required {
				labelFound = true
				break
			}
		}
	}

	if !labelFound {
		h.Logger.Infof("Required label %s not found on PR %v/%s#%d by %s",
			requiredLabels, prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)
		missingLabelComment := fmt.Sprintf("Beep, boop 🤖: To proceed, the pull request must have one of the '%v' labels.", requiredLabels)
		botComment := github.IssueComment{Body: &missingLabelComment}
		_, _, err := client.Issues.CreateComment(ctx, prComment.repoOwner, prComment.repoName, prComment.prNum, &botComment)
		if err != nil {
			h.Logger.Errorf("Failed to comment on pull request about missing label: %v", err)
		}
		return false, nil
	}

	return true, nil
}

func setJobKey(r *redis.Client, jobNumber int64, key string, value interface{}) error {
	return r.Set(context.Background(), "jobs:"+strconv.FormatInt(jobNumber, 10)+":"+key, value, 0).Err()
}

func (h *PRCommentHandler) queueGenerateJob(ctx context.Context, client *github.Client, prComment *PRComment, jobType string) error {
	r := redis.NewClient(&redis.Options{
		Addr:     h.RedisHostPort,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	jobNumber, err := r.Incr(ctx, "jobs").Result()
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, "pr_number", prComment.prNum)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, "pr_sha", prComment.sha)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, "author", prComment.author)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, "installation_id", prComment.installID)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, "repo_owner", prComment.repoOwner)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, "repo_name", prComment.repoName)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, "job_type", jobType)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, "errors", "")
	if err != nil {
		return err
	}

	err = r.LPush(ctx, "generate", strconv.FormatInt(jobNumber, 10)).Err()
	if err != nil {
		return err
	}

	msgStatus := "Job ID: " + strconv.FormatInt(jobNumber, 10) + " - Generating test data for your PR.\n\n" +
		"This may take several minutes...\n\n"

	msgComment := fmt.Sprintf("Generating test data for your PR with the job type: *%s*. Your Job ID is %d. The "+
		"reults will be presented below in the pull request status box. This may take several minutes...\n\n",
		jobType, jobNumber)

	var statusContext string
	switch jobType {
	case "generate":
		statusContext = util.GenerateLocalStatus
	case "precheck":
		statusContext = util.PrecheckStatus
	case "sdg-svc":
		statusContext = util.GenerateSDGStatus
	case "enable":
		statusContext = util.TriageReadinessStatus
	default:
		h.Logger.Errorf("Unknown job type: %s", jobType)
	}

	params := util.PullRequestStatusParams{
		State:      util.Pending,
		MsgStatus:  msgStatus,
		MsgComment: msgComment,
		StatusCtx:  statusContext,
		TargetURL:  util.InstructLabMaintainersTeamUrl,
		RepoOwner:  prComment.repoOwner,
		RepoName:   prComment.repoName,
		PrSha:      prComment.sha,
		PrNum:      prComment.prNum,
		JobType:    jobType,
		JobID:      strconv.FormatInt(jobNumber, 10),
	}

	return util.PostPullRequestStatus(ctx, client, params)

}

func (h *PRCommentHandler) checkAuthorPermission(ctx context.Context, client *github.Client, prComment *PRComment, teamName string) (bool, error) {
	// Check if the user is a part of teams allowed to trigger the command

	if prComment.repoOrg == "" {
		h.Logger.Warnf("No organization found in the repository URL")
	}

	teamMembership, response, err := client.Teams.GetTeamMembershipBySlug(ctx, prComment.repoOrg, teamName, prComment.author)
	if err != nil {
		if response.StatusCode == http.StatusNotFound {
			h.Logger.Infof("User %s is not a part of the team %s : %v", prComment.author, teamName, err)
			return false, nil
		}
		return false, err
	}

	h.Logger.Debugf("Team membership response for user %s: %v", prComment.author, response)
	h.Logger.Debugf("Team membership for user %s: %v", prComment.author, teamMembership)

	if teamMembership.GetState() != "active" || teamMembership.GetRole() != "maintainer" {
		h.Logger.Infof("User %s does not have required permission in the team %s", prComment.author, teamName)
		return false, nil
	}
	h.Logger.Infof("User %s is a part of the team %s", prComment.author, teamName)
	return true, nil
}

func (h *PRCommentHandler) checkEnableStatus(ctx context.Context, client *github.Client, prComment *PRComment) (bool, error) {
	repoStatus, response, err := client.Repositories.ListStatuses(ctx, prComment.repoOwner, prComment.repoName, prComment.sha, nil)
	if err != nil {
		h.Logger.Errorf("Failed to check repository status for %s/%s: %v", prComment.repoOwner, prComment.repoName, err.Error())
		return false, err
	}

	h.Logger.Debugf("Repository status for %s/%s: %v", prComment.repoOwner, prComment.repoName, repoStatus)

	if response.StatusCode == http.StatusOK {
		for _, status := range repoStatus {
			if strings.HasSuffix(status.GetURL(), prComment.sha) {
				if status.GetState() == util.Success && status.GetContext() == util.TriageReadinessStatus {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func (h *PRCommentHandler) enableCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("Enable command received on %s/%s#%d by author %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)

	h.Logger.Infof("Maintainers: %v", h.Maintainers)

	// Check if user is part of the teams that are allowed to enable the bot
	isAllowed := true
	for _, teamName := range h.Maintainers {
		var err error
		isAllowed, err = h.checkAuthorPermission(ctx, client, prComment, teamName)
		if err != nil {
			h.Logger.Errorf("Failed to check comment author (%s) permission for team (%s): %v", prComment.author, teamName, err.Error())
		}
		if isAllowed {
			break
		}
	}

	if !isAllowed {
		return fmt.Errorf("User %s is not allowed to enable the instruct bot. Only %v teams are allowed to enable the bot.", prComment.author, h.Maintainers)
	}

	msg := fmt.Sprintf("Beep, boop 🤖 Hi, I'm %s and I'm going to help you"+
		" with your pull request. Thanks for you contribution! 🎉\n\n", h.BotUsername)
	msg += fmt.Sprintf("I support the following commands:\n\n"+
		"* `%s precheck` -- Check existing model behavior using the questions in this proposed change.\n"+
		"* `%s generate` -- Generate a sample of synthetic data using the synthetic data generation backend infrastructure.\n"+
		"* `%s generate-local` -- Generate a sample of synthetic data using a local model.\n",
		h.BotUsername, h.BotUsername, h.BotUsername)
	botComment := github.IssueComment{
		Body: &msg,
	}

	if _, _, err := client.Issues.CreateComment(ctx, prComment.repoOwner, prComment.repoName, prComment.prNum, &botComment); err != nil {
		h.Logger.Error("Failed to comment on pull request: %w", err)
	}

	params := util.PullRequestStatusParams{
		State:     util.Success,
		MsgStatus: TriageReadinessMsg,
		StatusCtx: util.TriageReadinessStatus,
		TargetURL: util.InstructLabMaintainersTeamUrl,
		RepoOwner: prComment.repoOwner,
		RepoName:  prComment.repoName,
		PrSha:     prComment.sha,
		PrNum:     prComment.prNum,
	}

	return util.PostPullRequestStatus(ctx, client, params)
	//return util.PostPullRequestStatus(ctx, client, util.Success, TriageReadinessMsg, util.TriageReadinessStatus, util.InstructLabMaintainersTeamUrl, prComment.repoOwner, prComment.repoName, prComment.sha, 0, "", "")
}

func (h *PRCommentHandler) generateCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("Generate command received on %s/%s#%d by %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)

	isBotEnabled, err := h.checkEnableStatus(ctx, client, prComment)
	if err != nil {
		return err
	}

	if !isBotEnabled {
		return errors.New("Bot is not enabled on this PR. Maintainers need to enable the bot first.")
	}

	present, err := h.checkRequiredLabel(ctx, client, prComment, h.RequiredLabels)
	if !present || err != nil {
		return err
	}

	return h.queueGenerateJob(ctx, client, prComment, "generate")
}

func (h *PRCommentHandler) precheckCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("Precheck command received on %s/%s#%d by %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)

	isBotEnabled, err := h.checkEnableStatus(ctx, client, prComment)
	if err != nil {
		return err
	}

	if !isBotEnabled {
		return fmt.Errorf("Bot is not enabled on this PR. Maintainers need to enable the bot first.")
	}

	present, err := h.checkRequiredLabel(ctx, client, prComment, h.RequiredLabels)
	if !present || err != nil {
		return err
	}

	return h.queueGenerateJob(ctx, client, prComment, "precheck")
}

func (h *PRCommentHandler) sdgSvcCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("SDG svc command received on %s/%s#%d by %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)

	isBotEnabled, err := h.checkEnableStatus(ctx, client, prComment)
	if err != nil {
		return err
	}

	if !isBotEnabled {
		return fmt.Errorf("Bot is not enabled on this PR. Maintainers need to enable the bot first.")
	}

	present, err := h.checkRequiredLabel(ctx, client, prComment, h.RequiredLabels)
	if !present || err != nil {
		return err
	}

	return h.queueGenerateJob(ctx, client, prComment, "sdg-svc")
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
