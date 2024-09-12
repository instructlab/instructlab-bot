package util

import (
	"context"
	"net/http"

	"github.com/google/go-github/v61/github"
	"github.com/instructlab/instructlab-bot/gobot/common"
)

const KnowledgeLabel string = "knowledge"

func CheckBotEnableStatus(ctx context.Context, client *github.Client, params PullRequestStatusParams) (bool, error) {
	checkStatus, response, err := client.Checks.ListCheckRunsForRef(ctx, params.RepoOwner, params.RepoName, params.PrSha, nil)
	if err != nil {
		return false, err
	}

	if response.StatusCode == http.StatusOK {
		for _, status := range checkStatus.CheckRuns {
			if status.GetHeadSHA() == params.PrSha &&
				status.GetConclusion() == common.CheckStatusSuccess &&
				status.GetName() == params.CheckName {
				return true, nil
			}
		}
	}
	return false, nil
}

func CheckKnowledgeLabel(labels []*github.Label) (bool, error) {

	labelFound := false
	for _, label := range labels {
		if label.GetName() == KnowledgeLabel {
			labelFound = true
			break
		}
	}
	return labelFound, nil
}

func CheckRequiredLabel(labels []*github.Label, requiredLabels []string) (bool, error) {
	if len(requiredLabels) == 0 {
		return true, nil
	}

	labelFound := false
	for _, required := range requiredLabels {
		for _, label := range labels {
			if label.GetName() == required {
				labelFound = true
				break
			}
		}
	}
	return labelFound, nil
}
