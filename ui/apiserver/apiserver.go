package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	redisQueueGenerate = "generate"
	redisQueueArchive  = "archived"
)

const (
	localEndpoint     = "http://localhost:8000/v1"
	InstructLabBotUrl = "http://bot:8081"
)

type TLSConfig struct {
	TlsClientCertPath   string
	TlsClientKeyPath    string
	TlsServerCaCertPath string
	TlsInsecure         bool
}

type ApiServer struct {
	router              *gin.Engine
	logger              *zap.SugaredLogger
	redis               *redis.Client
	ctx                 context.Context
	testMode            bool
	preCheckEndpointURL string
	instructLabBotUrl   string
	tlsConfig           TLSConfig
}

type JobData struct {
	JobID          string `json:"jobID"`
	Duration       string `json:"duration"`
	Status         string `json:"status"`
	S3URL          string `json:"s3URL"`
	ModelName      string `json:"modelName"`
	RepoOwner      string `json:"repoOwner"`
	Author         string `json:"author"`
	PrNumber       string `json:"prNumber"`
	PrSHA          string `json:"prSHA"`
	RequestTime    string `json:"requestTime"`
	Errors         string `json:"errors"`
	RepoName       string `json:"repoName"`
	JobType        string `json:"jobType"`
	InstallationID string `json:"installationID"`
	Cmd            string `json:"cmd"`
}

type ChatRequest struct {
	Question string `json:"question"`
	Context  string `json:"context"`
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

func (api *ApiServer) chatHandler(c *gin.Context) {

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.logger.Error("Failed to bind JSON:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	api.logger.Infof("Received chat request - question: %v context: %v", req.Question, req.Context)

	answer, err := api.runIlabChatCommand(req.Question, req.Context)
	if err != nil {
		api.logger.Error("Failed to execute chat command:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"answer": answer})
}

func (api *ApiServer) skillPRHandler(c *gin.Context) {

	var prData SkillPRRequest
	if err := c.ShouldBindJSON(&prData); err != nil {
		api.logger.Error("Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	api.logger.Infof("Received Skill pull request data: %v", prData)

	prJson, err := json.Marshal(prData)
	if err != nil {
		api.logger.Errorf("Error encoding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}

	url := fmt.Sprintf("%s/pr/skill", InstructLabBotUrl)
	resp, err := api.sendPostRequest(url, bytes.NewBuffer(prJson))
	if err != nil {
		api.logger.Errorf("Error sending post request to bot http server: %v -- %v", err, resp)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}
	defer resp.Body.Close()

	responseBody := new(bytes.Buffer)
	_, err = responseBody.ReadFrom(resp.Body)
	if err != nil {
		api.logger.Errorf("Error reading response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
	}

	if resp.StatusCode != http.StatusOK {
		api.logger.Errorf("Error response (code : %s) from bot http server: %v", resp.StatusCode, responseBody.String())
		c.JSON(http.StatusInternalServerError, gin.H{"error": responseBody.String()})
		return
	}

	api.logger.Infof("Skill pull request response: %v", responseBody.String())
	c.JSON(http.StatusCreated, gin.H{"msg": responseBody.String()})
}

func (api *ApiServer) knowledgePRHandler(c *gin.Context) {

	var prData KnowledgePRRequest
	if err := c.ShouldBindJSON(&prData); err != nil {
		api.logger.Error("Failed to bind JSON:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	api.logger.Infof("Received Knowledge pull request data: %v", prData)

	prJson, err := json.Marshal(prData)
	if err != nil {
		api.logger.Errorf("Error encoding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}
	url := fmt.Sprintf("%s/pr/knowledge", InstructLabBotUrl)
	resp, err := api.sendPostRequest(url, bytes.NewBuffer(prJson))
	if err != nil {
		api.logger.Errorf("Error sending post request to bot http server: %v -- %v", err, resp)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}

	responseBody := new(bytes.Buffer)
	_, err = responseBody.ReadFrom(resp.Body)
	if err != nil {
		api.logger.Errorf("Error reading response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		api.logger.Errorf("Error response (code : %s) from bot http server: %v", resp.StatusCode, responseBody.String())
		c.JSON(http.StatusInternalServerError, gin.H{"error": responseBody.String()})
		return
	}

	api.logger.Infof("Knowledge pull request response: %v", responseBody.String())

	c.JSON(http.StatusCreated, gin.H{"msg": responseBody.String()})
}

func (api *ApiServer) buildHTTPServer() (http.Client, error) {
	defaultHTTPClient := http.Client{
		Timeout: 0 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	if !api.tlsConfig.TlsInsecure {
		certs, err := tls.LoadX509KeyPair(api.tlsConfig.TlsClientCertPath, api.tlsConfig.TlsClientKeyPath)
		if err != nil {
			api.logger.Warnf("failed to load client certificate/key: %w", err)
			return defaultHTTPClient, fmt.Errorf("Error load client certificate/key, defaulting to TLS Insecure session (http)")
		}
		// // NOT SURE WE NEED SERVER CA CERT FOR THIS, PLEASE ADVISE
		caCert, err := os.ReadFile(api.tlsConfig.TlsServerCaCertPath)
		if err != nil {
			api.logger.Warnf("failed to read server CA certificate: %w", err)
			return defaultHTTPClient, fmt.Errorf("Error load server CA certificate, defaulting to TLS Insecure session (http)")
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig := &tls.Config{
			Certificates:       []tls.Certificate{certs},
			RootCAs:            caCertPool,
			InsecureSkipVerify: true,
		}
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig:       tlsConfig,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}
		return *httpClient, nil
	} else {
		return defaultHTTPClient, nil
	}
}

// Sent http post request using custom client with zero timeout
func (api *ApiServer) sendPostRequest(url string, body io.Reader) (*http.Response, error) {
	client, err := api.buildHTTPServer()
	if err != nil {
		// Either running http with tlsInsecure = true, or https runing with tlsInsecure = false
		if err.Error() == "Error load client certificate/key, defaulting to TLS Insecure session (http)" ||
			err.Error() == "Error load server CA certificate, defaulting to TLS Insecure session (http)" {
			// Handle the specific error (e.g., log it)
			api.logger.Warn("Warning: TLS certificate/key or server CA certificate not loaded, downgraded to http client.")
		} else {
			// Handle other errors
			err = fmt.Errorf("Error creating http(s) server: %v", err)
			fmt.Print(err)
			return nil, err
		}
	}

	request, err := http.NewRequest("POST", url, body)
	if err != nil {
		api.logger.Errorf("Error creating http request: %v", err)
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		api.logger.Errorf("Error sending http request: %v", err)
		return nil, err
	}
	return response, nil
}

func (api *ApiServer) getAllJobs(c *gin.Context) {
	resultsJobIDs, err := api.redis.LRange(context.Background(), redisQueueGenerate, 0, -1).Result()
	if err != nil {
		api.logger.Error("Error retrieving results job IDs from Redis", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve results job IDs"})
		return
	}

	archiveJobIDs, err := api.redis.LRange(context.Background(), redisQueueArchive, 0, -1).Result()
	if err != nil {
		api.logger.Error("Error retrieving archive job IDs from Redis", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve archive job IDs"})
		return
	}

	jobIDs := append(archiveJobIDs, resultsJobIDs...)
	jobs := make([]JobData, 0)
	seenIDs := make(map[string]bool)
	for _, jobID := range jobIDs {
		if _, found := seenIDs[jobID]; !found {
			seenIDs[jobID] = true
			jobData, err := api.fetchJobData(jobID)
			if err != nil {
				api.logger.Error("Failed to fetch job data", zap.String("Job ID", jobID), zap.Error(err))
				continue
			}
			jobs = append(jobs, jobData)
		}
	}

	c.JSON(http.StatusOK, jobs)
}

func (api *ApiServer) fetchJobData(jobID string) (JobData, error) {
	var jobData JobData
	jobData.JobID = jobID
	jobData.Duration = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:duration", jobID)).Val()
	jobData.Status = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:status", jobID)).Val()
	jobData.S3URL = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:s3_url", jobID)).Val()
	jobData.ModelName = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:model_name", jobID)).Val()
	jobData.RepoOwner = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:repo_owner", jobID)).Val()
	jobData.Author = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:author", jobID)).Val()
	jobData.PrNumber = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:pr_number", jobID)).Val()
	jobData.PrSHA = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:pr_sha", jobID)).Val()
	jobData.RequestTime = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:request_time", jobID)).Val()
	jobData.Errors = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:errors", jobID)).Val()
	jobData.RepoName = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:repo_name", jobID)).Val()
	jobData.JobType = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:job_type", jobID)).Val()
	jobData.InstallationID = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:installation_id", jobID)).Val()
	jobData.Cmd = api.redis.Get(context.Background(), fmt.Sprintf("jobs:%s:cmd", jobID)).Val()

	return jobData, nil
}

func AuthRequired(username, password string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, pass, hasAuth := c.Request.BasicAuth()
		if hasAuth && user == username && pass == password {
			c.Next()
			return
		}
		c.Header("WWW-Authenticate", "Basic realm=\"Authorization Required\"")
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	}
}

func (api *ApiServer) setupRoutes(apiUser, apiPass string) {
	api.router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	authorized := api.router.Group("/")
	authorized.Use(AuthRequired(apiUser, apiPass))
	authorized.GET("/jobs", api.getAllJobs)
	authorized.POST("/chat", api.chatHandler)
	authorized.POST("pr/skill", api.skillPRHandler)
	authorized.POST("pr/knowledge", api.knowledgePRHandler)

	api.router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "IL Redis Queue")
	})
}

func (api *ApiServer) runIlabChatCommand(question, context string) (string, error) {
	if question == "" {
		api.logger.Error("Question not found or not a string")
		return "", fmt.Errorf("invalid question")
	}

	// Append the context to the question if present
	if context != "" {
		question = fmt.Sprintf("%s Answer this based on the following context: %s.", question, context)
	}

	// Construct the command string with the formatted question
	commandStr := fmt.Sprintf("chat --quick-question '%s' --tls-insecure", question)

	// Determine the mode and adjust the command string accordingly
	var cmd *exec.Cmd
	if api.testMode {
		// the model name is a dummy example in test mode
		commandStr += fmt.Sprintf(" --endpoint-url %s --model %s", api.preCheckEndpointURL, "/shared_model_storage/transformers_cache/models--ibm--merlinite-7b/snapshots/233d12759d5bb9344231dafdb51310ec19d79c0e")
		cmdArgs := strings.Fields(commandStr)
		cmd = exec.Command("echo", cmdArgs...)
		api.logger.Infof("Running in test mode: %s", commandStr)
	} else {
		modelName, err := api.fetchModelName(true)
		if err != nil {
			api.logger.Errorf("Failed to fetch model name: %v", err)
			return "failed to retrieve the model name", err
		}
		commandStr += fmt.Sprintf(" --endpoint-url %s --model %s", api.preCheckEndpointURL, modelName)
		cmdArgs := strings.Fields(commandStr)
		cmd = exec.Command("ilab", cmdArgs...)
		api.logger.Infof("Running in production mode with model name %s: %s", modelName, commandStr)
	}

	// Set environment and buffers for capturing output
	cmd.Env = os.Environ()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	// Execute the command and check for errors
	err := cmd.Run()
	if err != nil {
		api.logger.Error("Command failed with error: ", zap.Error(err), zap.String("stderr", errOut.String()))
		return "", err
	}

	// Log successful execution and return the trimmed output
	api.logger.Infof("Command executed successfully, output: %s", out.String())
	return strings.TrimSpace(out.String()), nil
}

func setupLogger(debugMode bool) *zap.SugaredLogger {
	config := zap.NewDevelopmentConfig()
	if debugMode {
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	logger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("Cannot build logger: %v", err))
	}
	return logger.Sugar()
}

// fetchModelName hits the defined precheck endpoint with "/models" appended to extract the model name.
// If fullName is true, it returns the entire ID value; if false, it returns the parsed out name after the double hyphens.
func (api *ApiServer) fetchModelName(fullName bool) (string, error) {
	// Ensure the endpoint URL ends with "/models"
	endpoint := api.preCheckEndpointURL
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
	endpoint += "models"

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	http.DefaultTransport.(*http.Transport).TLSHandshakeTimeout = 10 * time.Second
	http.DefaultTransport.(*http.Transport).ExpectContinueTimeout = 1 * time.Second

	req, err := http.NewRequestWithContext(api.ctx, "GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch model details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var responseData struct {
		Object string `json:"object"`
		Data   []struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &responseData); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if responseData.Object != "list" {
		return "", fmt.Errorf("expected object type 'list', got '%s'", responseData.Object)
	}

	// Extract the model name or the full ID based on the fullName flag
	for _, item := range responseData.Data {
		if item.Object == "model" {
			if !fullName {
				// Otherwise, parse and return the name after the last "--"
				parts := strings.Split(item.ID, "/")
				for _, part := range parts {
					if strings.Contains(part, "--") {
						nameParts := strings.Split(part, "--")
						if len(nameParts) > 1 {
							return nameParts[len(nameParts)-1], nil
						}
					}
				}
			}
			return item.ID, nil
		}
	}

	return "", fmt.Errorf("model name not found in response")
}

func main() {
	debugFlag := pflag.Bool("debug", false, "Enable debug mode")
	testMode := pflag.Bool("test-mode", false, "Don't run ilab commands, just echo back the ilab command to the chat response")
	listenAddress := pflag.String("listen-address", "localhost:3000", "Address to listen on")
	redisAddress := pflag.String("redis-server", "localhost:6379", "Redis server address")
	apiUser := pflag.String("api-user", "", "API username")
	apiPass := pflag.String("api-pass", "", "API password")
	preCheckEndpointURL := pflag.String("precheck-endpoint", "", "Precheck endpoint URL")
	InstructLabBotUrl := pflag.String("bot-url", InstructLabBotUrl, "InstructLab Bot URL")
	// TLS variables
	tlsInsecure := pflag.Bool("tls-insecure", false, "Whether to skip TLS verification")
	tlsClientCertPath := pflag.String("tls-client-cert", "$HOME/client-tls-crt.pem2", "Path to the TLS client certificate. Defaults to 'client-tls-crt.pem2'")
	tlsClientKeyPath := pflag.String("tls-client-key", "$HOME/client-tls-key.pem2", "Path to the TLS client key. Defaults to 'client-tls-key.pem2'")
	tlsServerCaCertPath := pflag.String("tls-server-ca-cert", "$HOME/server-ca-crt.pem2", "Path to the TLS server CA certificate. Defaults to 'server-ca-crt.pem2'")
	pflag.Parse()

	/* Support env population with priority being:
	1) flag
	2) env
	3) acceptable defaults
	*/

	// Precheck endpoint
	HOME := os.Getenv("HOME")
	if *preCheckEndpointURL == "" {
		preCheckEndpointURLEnvValue := os.Getenv("PECHECK_ENDPOINT")
		if preCheckEndpointURLEnvValue != "" {
			*preCheckEndpointURL = preCheckEndpointURLEnvValue
		} else {
			*preCheckEndpointURL = localEndpoint
		}
	}
	// TLS certPath
	if *tlsClientCertPath == "" {
		tlsClientCertPathEnvValue := os.Getenv("TLS_CLIENT_CERT_PATH")
		if tlsClientCertPathEnvValue != "" {
			*tlsClientCertPath = tlsClientCertPathEnvValue
		} else {
			*tlsClientCertPath = fmt.Sprintf("%s/client-tls-crt.pem2", HOME)
		}
	}
	// TLS keyPath
	if *tlsClientKeyPath == "" {
		tlsClientKeyPathEnvValue := os.Getenv("TLS_CLIENT_KEY_PATH")
		if tlsClientKeyPathEnvValue != "" {
			*tlsClientKeyPath = tlsClientKeyPathEnvValue
		} else {
			*tlsClientKeyPath = fmt.Sprintf("%s/client-tls-key.pem2", HOME)
		}
	}
	// NOTE: TLSInsecure not settable by env, just apiserver cli flag or defaults to false

	logger := setupLogger(*debugFlag)
	defer logger.Sync()

	if *apiUser == "" || *apiPass == "" {
		logger.Fatal("Username and password must be provided")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: *redisAddress,
	})

	router := gin.Default()
	svr := ApiServer{
		router:              router,
		logger:              logger,
		redis:               rdb,
		ctx:                 context.Background(),
		testMode:            *testMode,
		preCheckEndpointURL: *preCheckEndpointURL,
		instructLabBotUrl:   *InstructLabBotUrl,
		tlsConfig: TLSConfig{
			TlsInsecure:         *tlsInsecure,
			TlsClientCertPath:   *tlsClientCertPath,
			TlsClientKeyPath:    *tlsClientKeyPath,
			TlsServerCaCertPath: *tlsServerCaCertPath,
		},
	}
	svr.setupRoutes(*apiUser, *apiPass)

	if *tlsInsecure == false {
		// Check if we is valid key pair
		_, err := tls.LoadX509KeyPair(*tlsClientCertPath, *tlsClientKeyPath)
		if err != nil {
			logger.Fatal(fmt.Errorf("TLS enforced but failed to load client certificate/key: %w", err))
		}
		svr.logger.Info("ApiServer starting with TLS", zap.String("listen-address", *listenAddress))
		if err := svr.router.RunTLS(*listenAddress, *tlsClientCertPath, *tlsClientKeyPath); err != nil {
			svr.logger.Error("ApiServer failed to start", zap.Error(err))
		}
	} else {
		if err := svr.router.Run(*listenAddress); err != nil {
			svr.logger.Error("ApiServer failed to start", zap.Error(err))
		}
	}
}
