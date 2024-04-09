package bot

import (
	"context"
	"errors"
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
	cfg, err := getConfig()
	if err != nil {
		return err
	}

	logger := zLogger.Sugar()

	metricsRegistry := metrics.DefaultRegistry

	cc, err := githubapp.NewDefaultCachingClientCreator(
		cfg.Github,
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
		RedisHostPort: cfg.AppConfig.RedisHostPort,
		RequiredLabel: cfg.AppConfig.RequiredLabel,
		BotUsername:   cfg.GetBotUsername(),
	}

	prHandler := &handlers.PullRequestHandler{
		ClientCreator: cc,
		Logger:        logger,
		RequiredLabel: cfg.AppConfig.RequiredLabel,
		BotUsername:   cfg.GetBotUsername(),
	}

	webhookHandler := githubapp.NewDefaultEventDispatcher(cfg.Github, prCommentHandler, prHandler)

	http.Handle(githubapp.DefaultWebhookRoute, webhookHandler)

	addr := net.JoinHostPort(cfg.Server.Address, strconv.Itoa(cfg.Server.Port))
	logger.Infof("Starting server on %s...", addr)

	wg := sync.WaitGroup{}
	if cfg.AppConfig.WebhookProxyURL != "" {
		args := []string{
			"gosmee",
			"client",
			cfg.AppConfig.WebhookProxyURL,
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
		receiveResults(cfg, logger, cc)
	}()
	wg.Wait()

	return nil
}

func receiveResults(cfg *config.Config, logger *zap.SugaredLogger, cc githubapp.ClientCreator) {
	ctx := context.Background()

	r := redis.NewClient(&redis.Options{
		Addr:     cfg.AppConfig.RedisHostPort,
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
			continue
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

		logger.Infof("Processing result for %s/%s#%s, job ID: %s", repoOwner, repoName, prNumber, result)

		// check for errors prior to checking for an S3 url and models since that will not get produced on a failure
		prErrors, _ := r.Get("jobs:" + result + ":errors").Result()
		if prErrors != "" {
			errCommentBody := fmt.Sprintf("Beep, boop 🤖 an error occurred while processing your request, please review the following log for job id %s :\n```\n%s\n```", result, prErrors)
			if err := PostComment(ctx, cc, logger, installIDInt, repoOwner, repoName, prNumInt, errCommentBody); err != nil {
				logger.Errorf("Failed to send issue comment: %v", err)
			}
			continue
		}

		s3Url, err := r.Get("jobs:" + result + ":s3_url").Result()
		if err != nil || s3Url == "" {
			logger.Errorf("No S3 URL found for job %s", result)
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
		commentBody := fmt.Sprintf("Beep, boop 🤖  The test data has been generated for job ID: %s", result)
		if modelName != "" {
			commentBody += " " + modelName
		}
		commentBody += fmt.Sprintf("!\n\nFind your results [here](%s).", s3Url)

		err = PostComment(ctx, cc, logger, installIDInt, repoOwner, repoName, prNumInt, commentBody)
		if err != nil {
			logger.Errorf("Failed to send issue comment: %v", err)
		}
	}
}

// PostComment sends a comment to the GH pull request.
func PostComment(ctx context.Context, cc githubapp.ClientCreator, logger *zap.SugaredLogger, installIDInt int, repoOwner, repoName string, prNumInt int, commentBody string) error {
	issueComment := github.IssueComment{
		Body: github.String(commentBody),
	}

	client, err := cc.NewInstallationClient(int64(installIDInt))
	if err != nil {
		logger.Errorf("Error creating GitHub client: %v", err)
		return err
	}
	_, _, err = client.Issues.CreateComment(ctx, repoOwner, repoName, prNumInt, &issueComment)
	if err != nil {
		logger.Errorf("Error posting comment to GitHub PR: %v", err)
		return err
	}

	return nil
}

func getConfig() (*config.Config, error) {
	var cfg *config.Config
	var err error
	for _, path := range []string{"./config.yaml", "$HOME/.config/instruct-lab-bot/config.yaml", "/etc/instruct-lab-bot/config.yaml"} {
		cfg, err = config.ReadConfig(path)
		if err == nil {
			break
		}
	}
	if cfg == nil {
		return nil, errors.New("failed to read config file from any of the expected locations")
	}
	return cfg, nil
}
