package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	gosmee "github.com/chmouel/gosmee/gosmee"
	"github.com/go-redis/redis"
	"github.com/google/go-github/v60/github"
	"github.com/gregjones/httpcache"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/rcrowley/go-metrics"
	"go.uber.org/zap"

	"github.com/instruct-lab/instruct-lab-bot/gobot/config"
	"github.com/instruct-lab/instruct-lab-bot/gobot/handlers"
)

func Run(zLogger *zap.Logger) error {
	config, err := config.ReadConfig("config.yaml")
	if err != nil {
		return err
	}

	logger := zLogger.Sugar()

	metricsRegistry := metrics.DefaultRegistry

	cc, err := githubapp.NewDefaultCachingClientCreator(
		config.Github,
		githubapp.WithClientUserAgent("instruct-lab-bot/0.0.1"),
		githubapp.WithClientTimeout(3*time.Second),
		githubapp.WithClientCaching(false, func() httpcache.Cache { return httpcache.NewMemoryCache() }),
		githubapp.WithClientMiddleware(
			githubapp.ClientMetrics(metricsRegistry),
		),
	)
	if err != nil {
		return err
	}

	prCommentHandler := &handlers.PRCommentHandler{
		ClientCreator: cc,
		Logger:        logger,
		RedisHostPort: config.AppConfig.RedisHostPort,
	}

	webhookHandler := githubapp.NewDefaultEventDispatcher(config.Github, prCommentHandler)

	http.Handle(githubapp.DefaultWebhookRoute, webhookHandler)

	addr := net.JoinHostPort(config.Server.Address, strconv.Itoa(config.Server.Port))
	logger.Infof("Starting server on %s...", addr)

	wg := sync.WaitGroup{}
	if config.AppConfig.WebhookProxyURL != "" {
		args := []string{
			"gosmee",
			"client",
			config.AppConfig.WebhookProxyURL,
			fmt.Sprintf("http://%s/api/github/hook", addr),
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := gosmee.Run(args)
			if err != nil {
				logger.Errorf("Error running gosmee: %v", err)
			}
		}()
	}
	wg.Add(1)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.Errorf("Failed to start server: %v", err)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		recievePRBenchData(config, logger, cc)
		receiveResults(config, logger, cc)
	}()
	wg.Wait()

	return nil
}

// PRData is the struct that the raw artifact data will be unmarshaled into.
type PRData struct {
	PullRequest int     `json:"PR"`
	ScoreBefore float64 `json:"score_before"`
	ScoreAfter  float64 `json:"score_after"`
}

// this pulls an existing artifact ID down from redis. Assuming we have access to publish to this from the python backend
// if not, we will somehow need to make the bot aware of this artifact ID
func recievePRBenchData(config *config.Config, logger *zap.SugaredLogger, cc githubapp.ClientCreator) {
	ctx := context.Background()

	r := redis.NewClient(&redis.Options{
		Addr:     config.AppConfig.RedisHostPort,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	for {
		count, err := r.LLen("results").Result()
		if err != nil {
			logger.Errorf("Redis Client Error: %v", err)
			continue
		}
		if count == 0 {
			time.Sleep(5 * time.Second)
			continue
		}
		result, err := r.RPop("results").Result()
		if err != nil {
			logger.Errorf("Redis Client Error: %v", err)
		}

		if result == "" {
			continue
		}
		artifactID, err := r.Get("jobs:" + result + ":artifact_id").Result()
		if err != nil || artifactID == "" {
			logger.Errorf("No PR number found for job %s", result)
			continue
		}

		project := "your_project"
		runID := "your_run_id"
		apiKey := "your_wandb_api_key"

		url := fmt.Sprintf("https://api.wandb.ai/api/v1/artifacts/%s/%s/%s/download", project, runID, &artifactID)

		// Create HTTP request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			fmt.Println("Error creating request:", err)
			return
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		// Send HTTP request
		httpClient := &http.Client{}
		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Println("Error sending request:", err)
			return
		}
		defer resp.Body.Close()

		// Check response status code
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Error: %s\n", resp.Status)
			return
		}

		// Read the artifact content
		artifactContent, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading artifact content:", err)
			return

		}

		prData := &PRData{}
		err = json.Unmarshal(artifactContent, prData)
		if err != nil {
			fmt.Println("Error unmarshaling artifact content:", err)
			return
		}

		installID, err := r.Get("jobs:" + result + ":installation_id").Result()
		if err != nil || installID == "" {
			logger.Errorf("No installation ID found for job %s", result)
			continue
		}
		installIDInt, err := strconv.Atoi(installID)
		if err != nil {
			logger.Errorf("Error converting installation ID to int: %v", err)
			continue
		}

		body := ""
		if prData.ScoreAfter > prData.ScoreBefore {
			body = fmt.Sprintf("Your PR improved the model performance by %f points, congratulations ", prData.ScoreAfter-prData.ScoreBefore)
		} else {
			body = fmt.Sprintf("Your PR decreased the model performance by %f points, congratulations ", prData.ScoreBefore-prData.ScoreAfter)
		}
		issueComment := github.IssueComment{
			Body: github.String(
				body),
		}
		client, err := cc.NewInstallationClient(int64(installIDInt))
		if err != nil {
			logger.Errorf("Error creating GitHub client: %v", err)
			continue
		}
		_, _, err = client.Issues.CreateComment(ctx, "instruct-lab", "taxonomy", prData.PullRequest, &issueComment)
		if err != nil {
			logger.Errorf("Error creating comment: %v", err)
		}

	}

}

// this is where comments are responded to
// what is the difference between this and generate.go?
func receiveResults(config *config.Config, logger *zap.SugaredLogger, cc githubapp.ClientCreator) {
	ctx := context.Background()

	r := redis.NewClient(&redis.Options{
		Addr:     config.AppConfig.RedisHostPort,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	for {
		count, err := r.LLen("results").Result()
		if err != nil {
			logger.Errorf("Redis Client Error: %v", err)
			continue
		}
		if count == 0 {
			time.Sleep(5 * time.Second)
			continue
		}
		result, err := r.RPop("results").Result()
		if err != nil {
			logger.Errorf("Redis Client Error: %v", err)
		}

		if result == "" {
			continue
		}

		prNumber, err := r.Get("jobs:" + result + ":pr_number").Result()
		if err != nil || prNumber == "" {
			logger.Errorf("No PR number found for job %s", result)
			continue
		}
		prNumInt, err := strconv.Atoi(prNumber)
		if err != nil {
			logger.Errorf("Error converting PR number to int: %v", err)
			continue
		}

		s3Url, err := r.Get("jobs:" + result + ":s3_url").Result()
		if err != nil || s3Url == "" {
			logger.Errorf("No S3 URL found for job %s", result)
			continue
		}

		installID, err := r.Get("jobs:" + result + ":installation_id").Result()
		if err != nil || installID == "" {
			logger.Errorf("No installation ID found for job %s", result)
			continue
		}
		installIDInt, err := strconv.Atoi(installID)
		if err != nil {
			logger.Errorf("Error converting installation ID to int: %v", err)
			continue
		}

		repoOwner, err := r.Get("jobs:" + result + ":repo_owner").Result()
		if err != nil || repoOwner == "" {
			logger.Errorf("No repo owner found for job %s", result)
			continue
		}

		repoName, err := r.Get("jobs:" + result + ":repo_name").Result()
		if err != nil || repoName == "" {
			logger.Errorf("No repo name found for job %s", result)
			continue
		}

		issueComment := github.IssueComment{
			Body: github.String(
				"Beep, boop 🤖  The test data has been generated!\n\n" +
					"Find your results [here](" + s3Url + ").\n\n" +
					"*This URL expires in 7 days.*"),
		}
		client, err := cc.NewInstallationClient(int64(installIDInt))
		if err != nil {
			logger.Errorf("Error creating GitHub client: %v", err)
			continue
		}
		_, _, err = client.Issues.CreateComment(ctx, repoOwner, repoName, prNumInt, &issueComment)
		if err != nil {
			logger.Errorf("Error creating comment: %v", err)
		}
	}
}
