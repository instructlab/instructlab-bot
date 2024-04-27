package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v60/github"
	"github.com/instructlab/instructlab-bot/gobot/common"
	"github.com/instructlab/instructlab-bot/gobot/util"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	DeprecatedBotUsername = "@instruct-lab-bot"

	AccessCheckFailed = "Access check failed."
	LabelsNotFound    = "Required labels not found."
	BotEnabled        = "Bot is successfully enabled."
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
	prSha     string
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

	if event.GetRepo().GetName() != common.RepoName {
		h.Logger.Warnf("Received unexpected event %s from %s/%s repo. Skipping the event.",
			eventType, event.GetOrganization().GetLogin(), event.GetRepo().GetName())
		return nil
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
		h.Logger.Errorf("Failed to create installation client: %v", err)
		return err
	}
	words := strings.Fields(strings.TrimSpace(prComment.body))
	if len(words) < 2 {
		return nil
	}
	if words[0] == DeprecatedBotUsername {
		params := util.PullRequestStatusParams{
			RepoOwner: prComment.repoOwner,
			RepoName:  prComment.repoName,
			PrNum:     prComment.prNum,
		}
		params.Comment = fmt.Sprintf("> [!WARNING] \n > Beep, boop 🤖, The bot username `%s` is going to"+
			" be deprecated soon. Please use `%s` instead.", DeprecatedBotUsername, h.BotUsername)
		if err := util.PostPullRequestComment(ctx, client, params); err != nil {
			h.Logger.Errorf("Failed to post pull request comment: %v", err)
		}
	} else if words[0] != h.BotUsername {
		return nil
	}

	// Fetch the PR sha and labels to avoid multiple Pull Request API calls
	pr, _, err := client.PullRequests.Get(ctx, prComment.repoOwner, prComment.repoName, prComment.prNum)
	if err != nil {
		h.Logger.Errorf("Failed to get pull request (%s/%s#%d) related to the issue comment: %w", prComment.repoOwner, prComment.repoName, prComment.prNum, err)
		return err
	}

	prComment.prSha = pr.GetHead().GetSHA()
	prComment.labels = pr.Labels

	switch words[1] {
	case "help":
		return h.helpCommand(ctx, client, &prComment)
	case "enable":
		return h.enableCommand(ctx, client, &prComment)
	case "generate-local":
		return h.generateCommand(ctx, client, &prComment)
	case "precheck":
		return h.precheckCommand(ctx, client, &prComment)
	case "generate":
		return h.sdgSvcCommand(ctx, client, &prComment)
	default:
		return h.unknownCommand(ctx, client, &prComment)
	}
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

	jobNumber, err := r.Incr(ctx, common.RedisKeyJobs).Result()
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, common.RedisKeyPRNumber, prComment.prNum)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, common.RedisKeyPRSHA, prComment.prSha)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, common.RedisKeyAuthor, prComment.author)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, common.RedisKeyInstallationID, prComment.installID)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, common.RedisKeyRepoOwner, prComment.repoOwner)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, common.RedisKeyRepoName, prComment.repoName)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, common.RedisKeyJobType, jobType)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, common.RedisKeyErrors, "")
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, common.RedisKeyStatus, common.CheckStatusPending)
	if err != nil {
		return err
	}

	err = setJobKey(r, jobNumber, common.RedisKeyRequestTime, strconv.FormatInt(time.Now().Unix(), 10))
	if err != nil {
		return err
	}

	err = r.LPush(ctx, "generate", strconv.FormatInt(jobNumber, 10)).Err()
	if err != nil {
		h.Logger.Errorf("Failed to LPUSH job %d to redis %v", jobNumber, err)
		return err
	}

	summaryMsg := "Job ID: " + strconv.FormatInt(jobNumber, 10) + " - Generating test data.\n\n"
	detailsMsg := fmt.Sprintf("Generating test data for your PR with the job type: *%s*. \n"+
		"Related Job ID is %d.\n"+
		"This may take several minutes...\n\n", jobType, jobNumber)
	commentMsg := fmt.Sprintf("Beep, boop 🤖, Generating test data for your PR with the job type: *%s*. Your Job ID is %d. The "+
		"results will be presented below in the pull request status box. This may take several minutes...\n\n",
		jobType, jobNumber)

	var checkName string
	var statusName string
	switch jobType {
	case "generate":
		checkName = common.GenerateLocalCheck
		statusName = common.GenerateLocalStatus
	case "precheck":
		checkName = common.PrecheckCheck
		statusName = common.PrecheckStatus
	case "sdg-svc":
		checkName = common.GenerateSDGCheck
		statusName = common.GenerateSDGStatus
	default:
		h.Logger.Errorf("Unknown job type: %s", jobType)
	}

	params := util.PullRequestStatusParams{
		Status:       common.CheckInProgress,
		CheckSummary: summaryMsg,
		CheckDetails: detailsMsg,
		Comment:      commentMsg,
		CheckName:    checkName,
		JobType:      jobType,
		JobID:        strconv.FormatInt(jobNumber, 10),
		RepoOwner:    prComment.repoOwner,
		RepoName:     prComment.repoName,
		PrNum:        prComment.prNum,
		PrSha:        prComment.prSha,
	}

	statusExist, _ := util.StatusExist(ctx, client, params, statusName)
	if statusExist {
		commentMsg += fmt.Sprintf("\n > [!CAUTION] \n > **Migration Alert** There is an existing github Status (%s) present on your PR.\n"+
			"Please ignore that Status because we recently moved from github Status to github Checks.\n"+
			"Results (success or error) for this command will be present under the new github Check named %s.\n", statusName, checkName)
		params.Comment = commentMsg
	}

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

func (h *PRCommentHandler) checkAuthorPermission(ctx context.Context, client *github.Client, prComment *PRComment) bool {
	if prComment.repoOrg == "" {
		h.Logger.Warnf("No organization found in the repository URL")
	}

	// Check if user is part of the teams that are allowed to enable the bot
	isAllowed := true
	for _, teamName := range h.Maintainers {
		var err error
		isAllowed = true
		teamMembership, _, err := client.Teams.GetTeamMembershipBySlug(ctx, prComment.repoOrg, teamName, prComment.author)
		if err != nil {
			isAllowed = false
			h.Logger.Debugf("Failed to get team membership for user %s in team %s: %v", prComment.author, teamName, err)
			continue
		}

		if teamMembership.GetState() != "active" {
			h.Logger.Debugf("User %s does not have required permission in the team %s", prComment.author, teamName)
			isAllowed = false
			continue
		}
		h.Logger.Infof("User %s is a part of the team %s", prComment.author, teamName)
		if isAllowed {
			break
		}
	}
	return isAllowed
}

func (h *PRCommentHandler) helpCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("Help command received on %s/%s#%d by %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)
	err := util.PostBotWelcomeMessage(ctx, client, prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.prSha, h.BotUsername, h.Maintainers)
	if err != nil {
		h.Logger.Errorf("Failed to post welcome message on PR %s/%s#%d: %v", prComment.repoOwner, prComment.repoName, prComment.prNum, err)
		return err
	}
	return nil
}

func (h *PRCommentHandler) enableCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("Enable command received on %s/%s#%d by %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)
	params := util.PullRequestStatusParams{
		RepoOwner: prComment.repoOwner,
		RepoName:  prComment.repoName,
		PrNum:     prComment.prNum,
		PrSha:     prComment.prSha,
	}
	params.Comment = fmt.Sprintf("> [!NOTE] \n > **Enable command is deprecated and removed now. If you are member of the maintainers team [%v], "+
		"you can run the commands directly. Enabling the bot is not required.**", h.Maintainers)

	err := util.PostPullRequestComment(ctx, client, params)
	if err != nil {
		h.Logger.Errorf("Failed to post comment on PR %s/%s#%d: %v", prComment.repoOwner, prComment.repoName, prComment.prNum, err)
		return err
	}
	return nil
}

func (h *PRCommentHandler) generateCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("Generate command received on %s/%s#%d by %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)

	params := util.PullRequestStatusParams{
		Status:     common.CheckComplete,
		Conclusion: common.CheckStatusFailure,
		CheckName:  common.GenerateLocalCheck,
		RepoOwner:  prComment.repoOwner,
		RepoName:   prComment.repoName,
		PrNum:      prComment.prNum,
		PrSha:      prComment.prSha,
	}

	// Check if user is part of the teams that are allowed to enable the bot
	isAllowed := h.checkAuthorPermission(ctx, client, prComment)

	if !isAllowed {
		params.Comment = fmt.Sprintf("User %s is not allowed to run the InstructLab bot. Only %v teams are allowed to access the bot functions.", prComment.author, h.Maintainers)

		err := util.PostPullRequestComment(ctx, client, params)
		if err != nil {
			h.Logger.Errorf("Failed to post comment on PR %s/%s#%d: %v", prComment.repoOwner, prComment.repoName, prComment.prNum, err)
			return err
		}
		return nil
	}

	present, err := util.CheckRequiredLabel(prComment.labels, h.RequiredLabels)
	if err != nil {
		h.Logger.Errorf("Failed to check required labels: %v", err)
	}
	if !present {
		detailsMsg := fmt.Sprintf("Beep, boop 🤖: To proceed, the pull request must have one of the '%v' labels.", h.RequiredLabels)
		if err != nil {
			detailsMsg = fmt.Sprintf("%s\nError: %v", detailsMsg, err)
		}

		params.CheckSummary = LabelsNotFound
		params.CheckDetails = detailsMsg

		return util.PostPullRequestCheck(ctx, client, params)
	}

	return h.queueGenerateJob(ctx, client, prComment, "generate")
}

func (h *PRCommentHandler) precheckCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("Precheck command received on %s/%s#%d by %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)

	params := util.PullRequestStatusParams{
		Status:     common.CheckComplete,
		Conclusion: common.CheckStatusFailure,
		CheckName:  common.PrecheckCheck,
		RepoOwner:  prComment.repoOwner,
		RepoName:   prComment.repoName,
		PrNum:      prComment.prNum,
		PrSha:      prComment.prSha,
	}

	// Check if user is part of the teams that are allowed to enable the bot
	isAllowed := h.checkAuthorPermission(ctx, client, prComment)
	if !isAllowed {
		params.Comment = fmt.Sprintf("User %s is not allowed to run the InstructLab bot. Only %v teams are allowed to access the bot functions.", prComment.author, h.Maintainers)

		err := util.PostPullRequestComment(ctx, client, params)
		if err != nil {
			h.Logger.Errorf("Failed to post comment on PR %s/%s#%d: %v", prComment.repoOwner, prComment.repoName, prComment.prNum, err)
			return err
		}
		return nil
	}

	present, err := util.CheckRequiredLabel(prComment.labels, h.RequiredLabels)
	if err != nil {
		h.Logger.Errorf("Failed to check required labels: %v", err)
	}
	if !present {
		detailsMsg := fmt.Sprintf("Beep, boop 🤖: To proceed, the pull request must have one of the '%v' labels.", h.RequiredLabels)
		if err != nil {
			detailsMsg = fmt.Sprintf("%s\nError: %v", detailsMsg, err)
		}

		params.CheckSummary = LabelsNotFound
		params.CheckDetails = detailsMsg

		return util.PostPullRequestCheck(ctx, client, params)
	}

	return h.queueGenerateJob(ctx, client, prComment, "precheck")
}

func (h *PRCommentHandler) sdgSvcCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("SDG svc command received on %s/%s#%d by %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)

	params := util.PullRequestStatusParams{
		Status:     common.CheckComplete,
		Conclusion: common.CheckStatusFailure,
		CheckName:  common.GenerateSDGCheck,
		RepoOwner:  prComment.repoOwner,
		RepoName:   prComment.repoName,
		PrNum:      prComment.prNum,
		PrSha:      prComment.prSha,
	}

	// Check if user is part of the teams that are allowed to enable the bot
	isAllowed := h.checkAuthorPermission(ctx, client, prComment)
	if !isAllowed {
		params.Comment = fmt.Sprintf("User %s is not allowed to run the InstructLab bot. Only %v teams are allowed to access the bot functions.", prComment.author, h.Maintainers)

		err := util.PostPullRequestComment(ctx, client, params)
		if err != nil {
			h.Logger.Errorf("Failed to post comment on PR %s/%s#%d: %v", prComment.repoOwner, prComment.repoName, prComment.prNum, err)
			return err
		}
		return nil
	}

	present, err := util.CheckRequiredLabel(prComment.labels, h.RequiredLabels)
	if err != nil {
		h.Logger.Errorf("Failed to check required labels: %v", err)
	}
	if !present {
		detailsMsg := fmt.Sprintf("Beep, boop 🤖: To proceed, the pull request must have one of the '%v' labels.", h.RequiredLabels)
		if err != nil {
			detailsMsg = fmt.Sprintf("%s\nError: %v", detailsMsg, err)
		}

		params.CheckSummary = LabelsNotFound
		params.CheckDetails = detailsMsg

		return util.PostPullRequestCheck(ctx, client, params)
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
