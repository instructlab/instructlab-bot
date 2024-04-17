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
	"github.com/instructlab/instruct-lab-bot/gobot/util"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	TriageReadinessMsg = "PR is ready for evaluation."
	AccessCheckFailed  = "Access check failed."
	LabelsNotFound     = "Required labels not found."
	BotEnabled         = "Bot is successfully enabled."
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

	prComment.prSha = pr.GetHead().GetSHA()
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

	return labelFound, nil
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

	err = setJobKey(r, jobNumber, "pr_sha", prComment.prSha)
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
		checkName = util.GenerateLocalCheck
		statusName = util.GenerateLocalStatus
	case "precheck":
		checkName = util.PrecheckCheck
		statusName = util.PrecheckStatus
	case "sdg-svc":
		checkName = util.GenerateSDGCheck
		statusName = util.GenerateSDGStatus
	case "enable":
		checkName = util.TriageReadinessCheck
		statusName = util.TriageReadinessStatus
	default:
		h.Logger.Errorf("Unknown job type: %s", jobType)
	}

	params := util.PullRequestStatusParams{
		Status:       util.CheckInProgress,
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

	h.Logger.Debugf("Team membership for user %s: %v", prComment.author, teamMembership)

	if teamMembership.GetState() != "active" {
		h.Logger.Infof("User %s does not have required permission in the team %s", prComment.author, teamName)
		return false, nil
	}
	h.Logger.Infof("User %s is a part of the team %s", prComment.author, teamName)
	return true, nil
}

func (h *PRCommentHandler) checkBotEnableStatus(ctx context.Context, client *github.Client, prComment *PRComment) (bool, error) {
	checkStatus, response, err := client.Checks.ListCheckRunsForRef(ctx, prComment.repoOwner, prComment.repoName, prComment.prSha, nil)
	if err != nil {
		h.Logger.Errorf("Failed to check bot enable status for %s/%s: %v", prComment.repoOwner, prComment.repoName, err.Error())
		return false, err
	}

	h.Logger.Debugf("Repository status for %s/%s: %v", prComment.repoOwner, prComment.repoName, checkStatus)

	if response.StatusCode == http.StatusOK {
		for _, status := range checkStatus.CheckRuns {
			if status.GetHeadSHA() == prComment.prSha &&
				status.GetConclusion() == util.CheckStatusSuccess &&
				status.GetName() == util.TriageReadinessCheck {
				return true, nil
			}
		}
	}
	return false, nil
}

func (h *PRCommentHandler) enableCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("Enable command received on %s/%s#%d by author %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)

	h.Logger.Debugf("Maintainers: %v", h.Maintainers)

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

	params := util.PullRequestStatusParams{
		Status:    util.CheckComplete,
		CheckName: util.TriageReadinessCheck,
		RepoOwner: prComment.repoOwner,
		RepoName:  prComment.repoName,
		PrNum:     prComment.prNum,
		PrSha:     prComment.prSha,
	}

	if !isAllowed {
		detailsMsg := fmt.Sprintf("User %s is not allowed to enable the instruct bot. Only %v teams are allowed to enable the bot.", prComment.author, h.Maintainers)
		params.Conclusion = util.CheckStatusFailure
		params.CheckSummary = AccessCheckFailed
		params.CheckDetails = detailsMsg
		params.Comment = detailsMsg

		err := util.PostPullRequestCheck(ctx, client, params)
		if err != nil {
			h.Logger.Errorf("Failed to post check on PR %s/%s#%d: %v", prComment.repoOwner, prComment.repoName, prComment.prNum, err)
			return err
		}
		err = util.PostPullRequestComment(ctx, client, params)
		if err != nil {
			h.Logger.Errorf("Failed to post comment on PR %s/%s#%d: %v", prComment.repoOwner, prComment.repoName, prComment.prNum, err)
			return err
		}
		return nil
	}

	detailsMsg := fmt.Sprintf("Beep, boop .. Hi, I'm %s and I'm going to help you"+
		" with your pull request. Thanks for you contribution! 🎉\n\n", h.BotUsername)
	detailsMsg += fmt.Sprintf("I support the following commands:\n\n"+
		"* `%s precheck` -- Check existing model behavior using the questions in this proposed change.\n"+
		"* `%s generate` -- Generate a sample of synthetic data using the synthetic data generation backend infrastructure.\n"+
		"* `%s generate-local` -- Generate a sample of synthetic data using a local model.\n"+
		"> [!NOTE] \n > **Results or Errors of these commands will be posted as a pull request check in the Checks section below**.\n",
		h.BotUsername, h.BotUsername, h.BotUsername)

	params.Conclusion = util.CheckStatusSuccess
	params.CheckSummary = BotEnabled
	params.CheckDetails = detailsMsg
	params.Comment = detailsMsg

	statusExist, _ := util.StatusExist(ctx, client, params, util.TriageReadinessStatus)
	if statusExist {
		detailsMsg += fmt.Sprintf("\n > [!CAUTION] \n > **Migration Alert** There is an existing github Status (%s) present on your PR.\n"+
			"Please ignore that Status because we recently moved from github Status to github Checks.\n"+
			"Results (success or error) for this command will be present under the new github Check named %s.\n", util.TriageReadinessStatus, util.TriageReadinessCheck)
		params.Comment = detailsMsg
	}

	err := util.PostPullRequestComment(ctx, client, params)
	if err != nil {
		h.Logger.Errorf("Failed to post comment on PR %s/%s#%d: %v", prComment.repoOwner, prComment.repoName, prComment.prNum, err)
		return err
	}
	err = util.PostPullRequestCheck(ctx, client, params)
	if err != nil {
		h.Logger.Errorf("Failed to post check on PR %s/%s#%d: %v", prComment.repoOwner, prComment.repoName, prComment.prNum, err)
		return err
	}
	return nil
}

func (h *PRCommentHandler) generateCommand(ctx context.Context, client *github.Client, prComment *PRComment) error {
	h.Logger.Infof("Generate command received on %s/%s#%d by %s",
		prComment.repoOwner, prComment.repoName, prComment.prNum, prComment.author)

	params := util.PullRequestStatusParams{
		Status:     util.CheckComplete,
		Conclusion: util.CheckStatusFailure,
		CheckName:  util.GenerateLocalCheck,
		RepoOwner:  prComment.repoOwner,
		RepoName:   prComment.repoName,
		PrNum:      prComment.prNum,
		PrSha:      prComment.prSha,
	}

	isBotEnabled, err := h.checkBotEnableStatus(ctx, client, prComment)
	if !isBotEnabled {
		detailsMsg := "Bot is not enabled on this PR. Maintainers need to enable the bot first."
		if err != nil {
			detailsMsg = fmt.Sprintf("%s\nError: %v", detailsMsg, err)
		}

		params.CheckSummary = AccessCheckFailed
		params.CheckDetails = detailsMsg

		return util.PostPullRequestCheck(ctx, client, params)
	}

	present, err := h.checkRequiredLabel(ctx, client, prComment, h.RequiredLabels)
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
		Status:     util.CheckComplete,
		Conclusion: util.CheckStatusFailure,
		CheckName:  util.PrecheckCheck,
		RepoOwner:  prComment.repoOwner,
		RepoName:   prComment.repoName,
		PrNum:      prComment.prNum,
		PrSha:      prComment.prSha,
	}

	isBotEnabled, err := h.checkBotEnableStatus(ctx, client, prComment)
	if !isBotEnabled {
		detailsMsg := "Bot is not enabled on this PR. Maintainers need to enable the bot first."
		if err != nil {
			detailsMsg = fmt.Sprintf("%s\nError: %v", detailsMsg, err)
		}

		params.CheckSummary = AccessCheckFailed
		params.CheckDetails = detailsMsg

		return util.PostPullRequestCheck(ctx, client, params)
	}

	present, err := h.checkRequiredLabel(ctx, client, prComment, h.RequiredLabels)
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
		Status:     util.CheckComplete,
		Conclusion: util.CheckStatusFailure,
		CheckName:  util.GenerateSDGCheck,
		RepoOwner:  prComment.repoOwner,
		RepoName:   prComment.repoName,
		PrNum:      prComment.prNum,
		PrSha:      prComment.prSha,
	}

	isBotEnabled, err := h.checkBotEnableStatus(ctx, client, prComment)
	if !isBotEnabled {
		detailsMsg := "Bot is not enabled on this PR. Maintainers need to enable the bot first."
		if err != nil {
			detailsMsg = fmt.Sprintf("%s\nError: %v", detailsMsg, err)
		}
		params.CheckSummary = AccessCheckFailed
		params.CheckDetails = detailsMsg

		return util.PostPullRequestCheck(ctx, client, params)
	}

	present, err := h.checkRequiredLabel(ctx, client, prComment, h.RequiredLabels)
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
