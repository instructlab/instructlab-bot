package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	gosmee "github.com/chmouel/gosmee/gosmee"
	"github.com/go-redis/redis"
	"github.com/gregjones/httpcache"
	"github.com/instructlab/instructlab-bot/gobot/common"
	"github.com/instructlab/instructlab-bot/gobot/handlers"
	"github.com/instructlab/instructlab-bot/gobot/util"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/rcrowley/go-metrics"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	JobFailed = "Command execution failed. Check details."
)

var (
	RedisHost           string
	HTTPAddress         string
	HTTPPort            int
	GithubIntegrationID int
	GithubURL           string
	GithubWebhookSecret string
	GithubAppPrivateKey string
	WebhookProxyURL     string
	RequiredLabels      []string
	Maintainers         []string
	BotUsername         string
	Debug               bool
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&RedisHost, "redis", "", "redis:6379", "The Redis instance to connect to")
	rootCmd.PersistentFlags().StringVarP(&HTTPAddress, "http-address", "", "127.0.0.1", "HTTP Address to bind to")
	rootCmd.PersistentFlags().IntVarP(&HTTPPort, "http-port", "", 8080, "HTTP Port to bind to")
	rootCmd.PersistentFlags().IntVarP(&GithubIntegrationID, "github-integration-id", "", 0, "The GitHub App Integration ID")
	rootCmd.PersistentFlags().StringVarP(&GithubURL, "github-url", "", "https://api.github.com/", "The URL of the GitHub instance")
	rootCmd.PersistentFlags().StringVarP(&GithubWebhookSecret, "github-webhook-secret", "", "", "The GitHub App Webhook Secret")
	rootCmd.PersistentFlags().StringVarP(&GithubAppPrivateKey, "github-app-private-key", "", "", "The GitHub App Private Key")
	rootCmd.PersistentFlags().StringVarP(&WebhookProxyURL, "webhook-proxy-url", "", "", "Get an ID from https://smee.io/new. If blank, the app will not use a webhook proxy")
	rootCmd.PersistentFlags().StringSliceVarP(&RequiredLabels, "required-labels", "", []string{}, "Label(s) required before a PR can be tested")
	rootCmd.PersistentFlags().StringSliceVarP(&Maintainers, "maintainers", "", []string{}, "GitHub users or groups that are considered maintainers")
	rootCmd.PersistentFlags().BoolVarP(&Debug, "debug", "d", false, "Enable debug logging")
	rootCmd.PersistentFlags().StringVarP(&BotUsername, "bot-username", "", "@instructlab-bot", "The username of the bot")
}

var rootCmd = &cobra.Command{
	Use:   "bot",
	Short: "Bot receives events from GitHub and processes them",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeConfig(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		zlogger := initLogger(Debug)
		logger := zlogger.Sugar()
		return run(logger)
	},
}

func run(logger *zap.SugaredLogger) error {
	logger.Info("Starting bot...")
	metricsRegistry := metrics.DefaultRegistry
	// Replace all instances of \n with actual newlines
	GithubAppPrivateKey = strings.ReplaceAll(GithubAppPrivateKey, "\\n", "\n")

	ghConfig := githubapp.Config{
		V3APIURL: GithubURL,
		App: struct {
			IntegrationID int64  `yaml:"integration_id" json:"integrationId"`
			WebhookSecret string `yaml:"webhook_secret" json:"webhookSecret"`
			PrivateKey    string `yaml:"private_key" json:"privateKey"`
		}{
			IntegrationID: int64(GithubIntegrationID),
			WebhookSecret: GithubWebhookSecret,
			PrivateKey:    GithubAppPrivateKey,
		},
	}

	cc, err := githubapp.NewDefaultCachingClientCreator(
		ghConfig,
		githubapp.WithClientUserAgent("instructlab-bot/0.0.1"),
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
		ClientCreator:  cc,
		Logger:         logger,
		RedisHostPort:  RedisHost,
		RequiredLabels: RequiredLabels,
		BotUsername:    BotUsername,
		Maintainers:    Maintainers,
	}

	prHandler := &handlers.PullRequestHandler{
		ClientCreator:  cc,
		Logger:         logger,
		RequiredLabels: RequiredLabels,
		BotUsername:    BotUsername,
		Maintainers:    Maintainers,
	}

	webhookHandler := githubapp.NewDefaultEventDispatcher(ghConfig, prCommentHandler, prHandler)

	http.Handle(githubapp.DefaultWebhookRoute, webhookHandler)

	addr := net.JoinHostPort(HTTPAddress, strconv.Itoa(HTTPPort))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	defer cancel()

	wg := sync.WaitGroup{}
	if WebhookProxyURL != "" {
		args := []string{
			"gosmee",
			"client",
			WebhookProxyURL,
			fmt.Sprintf("http://%s/api/github/hook", addr),
		}
		go func() {
			for {
				select {
				case <-ctx.Done():
					logger.Infof("Shutting down gosmee webhook relayer")
					return
				default:
					logger.Infof("Starting gosmee webhook event relayer..")
					err := gosmee.Run(args)
					if err != nil {
						logger.Warnf("Error running gosmee webhook relayer. Restarting... %v", err)
					}
				}
			}
		}()
	}
	wg.Add(1)
	httpServer := &http.Server{Addr: addr}
	go func() {
		logger.Infof("Starting server on %s...", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Http server hit an error: %v", err)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		receiveResults(ctx, RedisHost, logger, cc)
		wg.Done()
	}()

	<-ctx.Done()

	ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err = httpServer.Shutdown(ctx); err != nil {
		logger.Errorf("Error shutting down http server: %v", err)
	}

	wg.Wait()
	return nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initLogger(debug bool) *zap.Logger {
	level := zap.InfoLevel

	if debug {
		level = zap.DebugLevel
	}

	loggerConfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, _ := loggerConfig.Build()
	return logger
}

func initializeConfig(cmd *cobra.Command) error {
	v := viper.New()
	v.SetEnvPrefix("ILBOT")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
	bindFlags(cmd, v)
	return nil
}

func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		configName := f.Name
		if !f.Changed && v.IsSet(configName) {
			val := v.Get(configName)
			_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

func receiveResults(ctx context.Context, redisHostPort string, logger *zap.SugaredLogger, cc githubapp.ClientCreator) {

	r := redis.NewClient(&redis.Options{
		Addr:     redisHostPort,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
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
			prNumber, err := r.Get(buildRedisKey(result, common.RedisKeyPRNumber)).Result()
			if err != nil || prNumber == "" {
				logger.Errorf("No PR number found for job %s", result)
				continue
			}

			installID, err := r.Get(buildRedisKey(result, common.RedisKeyInstallationID)).Result()
			if err != nil || installID == "" {
				logger.Errorf("No installation ID found for job %s", result)
				continue
			}
			installIDInt, err := strconv.Atoi(installID)
			if err != nil {
				logger.Errorf("Error converting installation ID to int: %v", err)
				continue
			}

			repoOwner, err := r.Get(buildRedisKey(result, common.RedisKeyRepoOwner)).Result()
			if err != nil || repoOwner == "" {
				logger.Errorf("No repo owner found for job %s", result)
				continue
			}

			repoName, err := r.Get(buildRedisKey(result, common.RedisKeyRepoName)).Result()
			if err != nil || repoName == "" {
				logger.Errorf("No repo name found for job %s", result)
				continue
			}

			jobType, err := r.Get(buildRedisKey(result, common.RedisKeyJobType)).Result()
			if err != nil || jobType == "" {
				logger.Errorf("No job type found for job %s", result)
				continue
			}

			prSha, err := r.Get(buildRedisKey(result, common.RedisKeyPRSHA)).Result()
			if err != nil || prSha == "" {
				logger.Errorf("No PR SHA found for job %s", result)
				continue
			}

			requestTimeStr, err := r.Get(buildRedisKey(result, common.RedisKeyRequestTime)).Result()
			if err != nil || requestTimeStr == "" {
				logger.Errorf("No request time found for job %s", result)
				continue
			}
			requestTime, err := strconv.ParseInt(requestTimeStr, 10, 64)
			if err != nil {
				logger.Errorf("Error parsing request time for job %s: %v", result, err)
				continue
			}
			totalTime := time.Now().Unix() - requestTime

			prURL := fmt.Sprintf("https://github.com/%s/%s/pull/%s", repoOwner, repoName, prNumber)

			jobDuration, err := r.Get(buildRedisKey(result, common.RedisKeyDuration)).Result()
			if err != nil || jobDuration == "" {
				logger.Infof("Job result for %s/%s#%s, job ID: %s, GitHub URL: %s (No job duration time found for job)", repoOwner, repoName, prNumber, result, prURL)
			} else {

				jobDurationInt, err := strconv.ParseInt(jobDuration, 10, 64)
				if err != nil {
					logger.Errorf("Error parsing duration time for job %s: %v", result, err)
					continue
				}
				queueTime := totalTime - jobDurationInt
				logger.Infof("Job result for %s/%s#%s, job ID: %s, job duration: %s, queue time: %d URL: %s", repoOwner, repoName, prNumber, result, jobDuration, queueTime, prURL)
			}

			var statusContext string
			switch jobType {
			case "generate":
				statusContext = common.GenerateLocalCheck
			case "precheck":
				statusContext = common.PrecheckCheck
			case "sdg-svc":
				statusContext = common.GenerateSDGCheck
			default:
				logger.Errorf("Unknown job type: %s", jobType)
			}

			client, err := cc.NewInstallationClient(int64(installIDInt))
			if err != nil {
				logger.Errorf("Failed to create installation client: %v", err)
				continue
			}

			prNum, err := strconv.Atoi(prNumber)
			if err != nil {
				logger.Errorf("error converting string to int: %v", err)
				continue
			}

			// check for errors prior to checking for an S3 url and models since that will not get produced on a failure
			prErrors, _ := r.Get("jobs:" + result + ":errors").Result()
			if prErrors != "" {
				errCommentBody := fmt.Sprintf("An error occurred while processing your request, please review the following log for job id %s :\n\n```\n%s\n```", result, prErrors)

				params := util.PullRequestStatusParams{
					Status:       common.CheckComplete,
					Conclusion:   common.CheckStatusFailure,
					CheckName:    statusContext,
					CheckSummary: JobFailed,
					CheckDetails: errCommentBody,
					Comment:      errCommentBody,
					JobType:      jobType,
					JobID:        result,
					JobErr:       errCommentBody,
					RepoOwner:    repoOwner,
					RepoName:     repoName,
					PrNum:        prNum,
					PrSha:        prSha,
				}

				logger.Errorf("Error processing command on %s/%s#%d: err %s",
					params.RepoOwner, params.RepoName, params.PrNum, params.JobErr)

				err = util.PostPullRequestCheck(ctx, client, params)
				if err != nil {
					logger.Errorf("Failed to update error message on PR for job %s error: %v", result, err)
				}

				// Enable redis keys deletion once we have solution for persisting the job history
				// cleanupRedisKeys(logger, r, result)
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
			detailsMsg := fmt.Sprintf("Results for job ID: %s", result)
			if modelName != "" {
				detailsMsg += " " + modelName
			}
			detailsMsg += fmt.Sprintf("!\n\nResults can be found [here](%s).", s3Url)

			summaryMsg := fmt.Sprintf("Job ID: %s completed successfully. Check Details.", result)

			params := util.PullRequestStatusParams{
				Status:       common.CheckComplete,
				Conclusion:   common.CheckStatusSuccess,
				JobID:        result,
				JobType:      jobType,
				CheckName:    statusContext,
				CheckSummary: summaryMsg,
				CheckDetails: detailsMsg,
				Comment:      detailsMsg,
				RepoOwner:    repoOwner,
				RepoName:     repoName,
				PrNum:        prNum,
				PrSha:        prSha,
			}

			err = util.PostPullRequestCheck(ctx, client, params)
			if err != nil {
				logger.Errorf("Failed to post check on pr %s/%s#%d: %v", params.RepoOwner, params.RepoName, params.PrNum, err)
			}

			err = util.PostPullRequestComment(ctx, client, params)
			if err != nil {
				logger.Errorf("Failed to post comment on pr %s/%s#%d: %v", params.RepoOwner, params.RepoName, params.PrNum, err)
			}
			// Enable redis keys deletion once we have solution for persisting the job history
			// cleanupRedisKeys(logger, r, result)
		}
	}
}

// buildRedisKey constructs a Redis key for job attributes.
func buildRedisKey(jobID, keyType string) string {
	return fmt.Sprintf("%s:%s:%s", common.RedisKeyJobs, jobID, keyType)
}

//lint:ignore U1000
func cleanupRedisKeys(logger *zap.SugaredLogger, r *redis.Client, jobID string) {
	matchKey := fmt.Sprintf("%s:%s:*", common.RedisKeyJobs, jobID)
	var cursor uint64 = 0

	for {
		keys, nextCursor, err := r.Scan(cursor, matchKey, 0).Result()
		if err != nil {
			logger.Errorf("Error scanning keys matching to %s: %v", matchKey, err)
			return
		}

		for _, key := range keys {
			err := r.Del(key).Err()
			if err != nil {
				logger.Warnf("Error deleting key %s: %v", key, err)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	logger.Infof("Deleted all keys matching to %s", matchKey)
}
