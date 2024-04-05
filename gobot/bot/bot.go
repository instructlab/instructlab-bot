package bot

import (
	"context"
	"fmt"
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
		RequiredLabel: config.AppConfig.RequiredLabel,
		BotUsername:   config.GetBotUsername(),
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
		receiveResults(config, logger, cc)
	}()
	wg.Wait()

	return nil
}

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

		modelName, err := r.Get("jobs:" + result + ":model_name").Result()
		if err != nil || modelName == "" || modelName == "unknown" {
			logger.Infof("No specific model name found for job %s, using generic message.", result)
			modelName = ""
		} else {
			modelName = "using the model " + modelName
		}

		// Add the model name only if it's not empty
		commentBody := "Beep, boop 🤖  The test data has been generated"
		if modelName != "" {
			commentBody += " " + modelName
		}
		commentBody += fmt.Sprintf("!\n\nFind your results [here](%s).", s3Url)

		issueComment := github.IssueComment{
			Body: github.String(commentBody),
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
