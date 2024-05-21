package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v61/github"
	"github.com/palantir/go-githubapp/githubapp"
	"go.uber.org/zap"
)

type PullRequestCreateHandler struct {
	githubapp.ClientCreator
	Logger         *zap.SugaredLogger
	TaxonomyRepo   string
	GithubUsername string
	GithubToken    string
}

type PullRequestTask struct {
	PullRequestCreateHandler
	branchName string
}

type SkillPRRequest struct {
	Name             string   `json:"name"`
	Email            string   `json:"email"`
	Task_description string   `json:"task_description"`
	Task_details     string   `json:"task_details"`
	Title_work       string   `json:"title_work"`
	Link_work        string   `json:"link_work"`
	License_work     string   `json:"license_work"`
	Creators         string   `json:"creators"`
	Questions        []string `json:"questions"`
	Contexts         []string `json:"contexts"`
	Answers          []string `json:"answers"`
}

type KnowledgePRRequest struct {
	Name             string   `json:"name"`
	Email            string   `json:"email"`
	Task_description string   `json:"task_description"`
	Task_details     string   `json:"task_details"`
	Repo             string   `json:"repo"`
	Commit           string   `json:"commit"`
	Patterns         string   `json:"patterns"`
	Title_work       string   `json:"title_work"`
	Link_work        string   `json:"link_work"`
	Revision         string   `json:"revision"`
	License_work     string   `json:"license_work"`
	Creators         string   `json:"creators"`
	Domain           string   `json:"domain"`
	Questions        []string `json:"questions"`
	Answers          []string `json:"answers"`
}

const (
	TaxonomyPath        = "taxonomy"
	RepoSkillPath       = "/compositional_skills/bot_skills"
	RepoKnowledgePath   = "/knowledge/bot_knowledge"
	SkillStr            = "skill"
	KnowledgeStr        = "knowledge"
	YamlFileName        = "qna.yaml"
	AttributionFileName = "attribution.txt"
	branchNamePrefix    = "bot-pr"
)

func (prc *PullRequestCreateHandler) SkillPRHandler(w http.ResponseWriter, r *http.Request) {
	var requestData SkillPRRequest
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		prc.Logger.Errorf("Error decoding skill request body %v", err)
		http.Error(w, "Error decoding skill request body", http.StatusBadRequest)
		return
	}

	prc.Logger.Infof("Received Skill pull request data: %+v\n", requestData)

	prTask := PullRequestTask{
		PullRequestCreateHandler: *prc,
	}

	prTask.branchName = prc.generateBranchName(SkillStr)

	// Clone the taxonomy repo
	repo, err := prTask.cloneTaxonomyRepo()
	if err != nil {
		prc.Logger.Errorf("Error cloning taxonomy repo %v ", err)
		http.Error(w, "Error cloning taxonomy repo", http.StatusInternalServerError)
		return
	}

	// Checkout branch for the PR
	wt, err := prTask.checkoutBranch(repo)
	if err != nil {
		prc.Logger.Errorf("Error checking out branch %v ", err)
		http.Error(w, "Error checking out branch", http.StatusInternalServerError)
		return
	}

	// Create commit for the PR
	commitSha, err := prTask.createSkillCommit(wt, requestData)
	if err != nil {
		prc.Logger.Errorf("Error creating skill commit %v ", err)
		http.Error(w, "Error creating skill commit", http.StatusInternalServerError)
		return
	}

	// Push the commit to the taxonomy repo
	err = prTask.pushCommit(repo)
	if err != nil {
		prc.Logger.Errorf("Error pushing skill commit %v ", err)
		http.Error(w, "Error pushing skill commit", http.StatusInternalServerError)
		return
	}

	// Create the pull request of the pushed branch to taxonomy main branch
	pr, err := prTask.createPullRequest(SkillStr, requestData.Task_description, requestData.Task_details)
	if err != nil {
		prc.Logger.Errorf("Error creating skill pull request %v ", err)
		http.Error(w, "Error creating skill pull request", http.StatusInternalServerError)
		return
	}

	prc.Logger.Infof("Pull request (%s) created successfully for skill with commit sha: %s", pr.GetHTMLURL(), commitSha)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Pull request (%s) created successfully for the skill", pr.GetHTMLURL())

	//Delete the directory after the pull request is created
	err = os.RemoveAll(prTask.branchName)
	if err != nil {
		prc.Logger.Errorf("Error deleting branch directory %v ", err)
		return
	}
}

func (prc *PullRequestCreateHandler) KnowledgePRHandler(w http.ResponseWriter, r *http.Request) {
	var requestData KnowledgePRRequest
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		prc.Logger.Errorf("Error decoding knowledge request body %v", err)
		http.Error(w, "Error decoding knowledge request body", http.StatusBadRequest)
		return
	}

	prc.Logger.Infof("Received Knowledge pull request data: %+v\n", requestData)

	prTask := PullRequestTask{
		PullRequestCreateHandler: *prc,
	}

	prTask.branchName = prc.generateBranchName(KnowledgeStr)

	// Clone the taxonomy repo
	repo, err := prTask.cloneTaxonomyRepo()
	if err != nil {
		prc.Logger.Errorf("Error cloning taxonomy repo %v ", err)
		http.Error(w, "Error cloning taxonomy repo", http.StatusInternalServerError)
		return
	}

	// Checkout branch for the pull request
	wt, err := prTask.checkoutBranch(repo)
	if err != nil {
		prc.Logger.Errorf("Error checking out branch %v ", err)
		http.Error(w, "Error checking out branch", http.StatusInternalServerError)
		return
	}

	// Create commit for the pull request
	commitSha, err := prTask.createKnowledgeCommit(wt, requestData)
	if err != nil {
		prc.Logger.Errorf("Error creating knowledge commit %v ", err)
		http.Error(w, "Error creating knowledge commit", http.StatusInternalServerError)
		return
	}

	// Push the commit to the taxonomy repo
	err = prTask.pushCommit(repo)
	if err != nil {
		prc.Logger.Errorf("Error pushing knowledge commit %v ", err)
		http.Error(w, "Error pushing knowledge commit", http.StatusInternalServerError)
		return
	}

	// Create the pull request of the pushed branch to taxonomy main branch
	pr, err := prTask.createPullRequest(KnowledgeStr, requestData.Task_description, requestData.Task_details)
	if err != nil {
		prc.Logger.Errorf("Error creating knowledge pull request %v ", err)
		http.Error(w, "Error creating knowledge pull request", http.StatusInternalServerError)
		return
	}

	prc.Logger.Infof("Pull request (%s) created successfully for knowledge with commit sha: %s", pr.GetHTMLURL(), commitSha)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Pull request (%s) created successfully for the knowledge", pr.GetHTMLURL())

	//Delete the directory after the pull request is created
	err = os.RemoveAll(prTask.branchName)
	if err != nil {
		prc.Logger.Errorf("Error deleting branch directory %v ", err)
		return
	}
}

func (prt *PullRequestTask) cloneTaxonomyRepo() (*git.Repository, error) {
	sugar := prt.Logger.With("user_name", prt.GithubUsername)

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get working directory: %v", err)
	}
	taxonomyDir := path.Join(workDir, prt.branchName, TaxonomyPath)

	// Check if the taxonomy directory exists and delete if it does
	if _, err := os.Stat(taxonomyDir); !os.IsNotExist(err) {
		sugar.Warn("Taxonomy directory already exists, deleting")
		err = os.RemoveAll(taxonomyDir)
		if err != nil {
			return nil, fmt.Errorf("could not delete taxonomy directory: %v", err)
		}
	}

	var r *git.Repository
	if _, err := os.Stat(taxonomyDir); os.IsNotExist(err) {
		sugar.Warnf("Taxonomy directory does not exist, cloning from %s", prt.TaxonomyRepo)
		r, err = git.PlainClone(taxonomyDir, false, &git.CloneOptions{
			URL: prt.TaxonomyRepo,
			Auth: &githttp.BasicAuth{
				Username: prt.GithubUsername,
				Password: prt.GithubToken,
			},
			Progress: os.Stdout,
		})
		if err != nil {
			return nil, fmt.Errorf("could not clone taxonomy git repo: %v", err)
		}
	} else {
		r, err = git.PlainOpen(taxonomyDir)
		if err != nil {
			return nil, fmt.Errorf("could not open taxonomy git repo: %v", err)
		}
	}
	return r, nil
}

func (prt *PullRequestTask) checkoutBranch(repo *git.Repository) (*git.Worktree, error) {
	// Create a new branch
	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("Error getting head reference: %v", err)
	}

	branchRefName := plumbing.NewBranchReferenceName(prt.branchName)

	ref := plumbing.NewHashReference(branchRefName, headRef.Hash())

	// The created reference is saved in the storage.
	err = repo.Storer.SetReference(ref)
	if err != nil {
		return nil, fmt.Errorf("Error setting store reference: %v", err)
	}

	// Add the file to the git repo
	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("Error getting work tree: %v", err)
	}

	// Checkout to the new branch
	branchCoOpts := git.CheckoutOptions{
		Branch: plumbing.ReferenceName(branchRefName),
		Force:  true,
	}
	if err := wt.Checkout(&branchCoOpts); err != nil {
		return nil, fmt.Errorf("Error creating new branch: %v", err)
	}

	return wt, nil
}

func (prt *PullRequestTask) createSkillCommit(wt *git.Worktree, requestData SkillPRRequest) (string, error) {
	// Write the requestData to a file
	workDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Error get current working directory: %v", err)
	}

	dirPath := path.Join(workDir, prt.branchName, TaxonomyPath, RepoSkillPath, prt.branchName)
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("Error creating directory: %v", err)
	}

	filePath := path.Join(dirPath, YamlFileName)
	yamlFile, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("Error creating skill yaml file: %v", err)
	}
	defer yamlFile.Close()

	// Convert requestData to yaml
	yamlData, err := prt.generateSkillYaml(requestData)
	if err != nil {
		return "", fmt.Errorf("Error generating yaml from the skill pull request data: %v", err)
	}

	_, err = yamlFile.WriteString(yamlData)
	if err != nil {
		return "", fmt.Errorf("Error writing to skill yaml file: %v", err)
	}

	// Add the attribution file to the PR
	attributionFilePath := path.Join(dirPath, AttributionFileName)
	attributionFile, err := os.Create(attributionFilePath)
	if err != nil {
		return "", fmt.Errorf("Error creating attribution file for skill: %v", err)
	}
	defer attributionFile.Close()

	// Add the attribution file to the PR
	attributionFileData := prt.generateSkillAttributionData(requestData)

	_, err = attributionFile.WriteString(attributionFileData)
	if err != nil {
		return "", fmt.Errorf("Error writing to skill attribution file: %v", err)
	}

	skillDir := path.Join("."+RepoSkillPath, prt.branchName)
	_, err = wt.Add(skillDir)
	if err != nil {
		return "", fmt.Errorf("Error adding skill files to git: %v", err)
	}

	// Commit the changes with signature
	signature := &object.Signature{
		Name:  requestData.Name,
		Email: requestData.Email,
		When:  time.Now(),
	}

	commitMsg := fmt.Sprintf("%s: %s \n\n Signed-off-by: %s <%s>", SkillStr, requestData.Task_description, signature.Name, signature.Email)

	commit, err := wt.Commit(commitMsg, &git.CommitOptions{
		Author:    signature,
		Committer: signature,
	})
	if err != nil {
		return "", fmt.Errorf("Error committing skill related changes: %v", err)
	}
	return commit.String(), nil
}

func (prt *PullRequestTask) createKnowledgeCommit(wt *git.Worktree, requestData KnowledgePRRequest) (string, error) {
	// Write the requestData to a file
	workDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Error getting current working directory: %v", err)
	}

	dirPath := path.Join(workDir, prt.branchName, TaxonomyPath, RepoKnowledgePath, prt.branchName)
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("Error creating directory: %v", err)
	}

	filePath := path.Join(dirPath, YamlFileName)
	yamlFile, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("Error creating knowledge yaml file: %v", err)
	}
	defer yamlFile.Close()

	// Convert requestData to yaml
	yamlData, err := prt.generateKnowledgeYaml(requestData)
	if err != nil {
		return "", fmt.Errorf("Error generating knowledge yaml from the pull request data: %v", err)
	}

	_, err = yamlFile.WriteString(yamlData)
	if err != nil {
		return "", fmt.Errorf("Error writing to yaml file: %v", err)
	}

	// Add the attribution file to the PR
	attributionFilePath := path.Join(dirPath, AttributionFileName)
	attributionFile, err := os.Create(attributionFilePath)
	if err != nil {
		return "", fmt.Errorf("Error creating knowledge attribution file: %v", err)
	}
	defer attributionFile.Close()

	// Convert requestData to yaml
	attributionFileData := prt.generateKnowledgeAttributionData(requestData)

	_, err = attributionFile.WriteString(attributionFileData)
	if err != nil {
		return "", fmt.Errorf("Error writing to knowledge attribution file: %v", err)
	}

	skillDir := path.Join("."+RepoKnowledgePath, prt.branchName)
	_, err = wt.Add(skillDir)
	if err != nil {
		return "", fmt.Errorf("Error adding knowledge files to git: %v", err)
	}

	// Commit the changes with signature
	signature := &object.Signature{
		Name:  requestData.Name,
		Email: requestData.Email,
		When:  time.Now(),
	}

	commitMsg := fmt.Sprintf("%s: %s \n\n Signed-off-by: %s <%s>", KnowledgeStr, requestData.Task_description, signature.Name, signature.Email)

	commit, err := wt.Commit(commitMsg, &git.CommitOptions{
		Author:    signature,
		Committer: signature,
	})
	if err != nil {
		return "", fmt.Errorf("Error committing knowledge related changes: %v", err)
	}

	return commit.String(), nil
}

func (prt *PullRequestTask) pushCommit(repo *git.Repository) error {
	// Push the changes
	err := repo.Push(&git.PushOptions{
		Auth: &githttp.BasicAuth{
			Username: prt.GithubUsername,
			Password: prt.GithubToken,
		},
	})
	if err != nil {
		return fmt.Errorf("Error pushing changes: %v", err)
	}
	return nil
}

func (prt *PullRequestTask) createPullRequest(prType string, prTitle string, prDescription string) (*github.PullRequest, error) {
	// Create a PR
	client, err := prt.ClientCreator.NewTokenClient(prt.GithubToken)
	if err != nil {
		return nil, fmt.Errorf("Error creating Github client: %v", err)
	}

	ctx := context.Background()

	// Create a pull request
	newPR := &github.NewPullRequest{
		Title:               github.String(fmt.Sprintf("%s: %s", prType, prTitle)),
		Head:                github.String(prt.branchName),
		Base:                github.String("main"),
		Body:                github.String(prDescription),
		MaintainerCanModify: github.Bool(true),
	}

	pr, _, err := client.PullRequests.Create(ctx, prt.GithubUsername, TaxonomyPath, newPR)
	if err != nil {
		return nil, err
	}
	return pr, nil
}

// TODO: We need better way to generate branch name
func (prc *PullRequestCreateHandler) generateBranchName(prType string) string {
	return fmt.Sprintf("%s-%s-%s", branchNamePrefix, prType, time.Now().Format("20060102150405"))

}
