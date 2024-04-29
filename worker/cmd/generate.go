package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/gomodule/redigo/redis"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

var (
	WorkDir             string
	VenvDir             string
	PreCheckEndpointURL string
	SdgEndpointURL      string
	NumInstructions     int
	GitRemote           string
	Origin              string
	GithubUsername      string
	GithubToken         string
	S3Bucket            string
	AWSRegion           string
	TlsClientCertPath   string
	TlsClientKeyPath    string
	TlsServerCaCertPath string
	TlsInsecure         bool
	MaxSeed             int
	TaxonomyFolders     = []string{"compositional_skills", "knowledge"}
)

const (
	gitMaxRetries            = 5
	gitRetryDelay            = 2 * time.Second
	ilabConfigPath           = "config.yaml"
	localEndpoint            = "http://localhost:8000/v1"
	jobSDG                   = "sdg-svc"
	jobGenerateLocal         = "generate"
	jobPreCheck              = "precheck"
	sdgModel                 = "mistralai/mixtral-8x7b-instruct-v0-1"
	jsonViewerFilenameSuffix = "-viewer.html"
)

const (
	jobStatusSuccess = "success"
	jobStatusError   = "error"
	jobStatusRunning = "running"
	jobStatusPending = "pending"
)

// Worker encapsulates dependencies and methods to process jobs
type Worker struct {
	ctx                 context.Context
	pool                *redis.Pool
	svc                 *s3.Client
	logger              *zap.SugaredLogger
	job                 string
	precheckEndpoint    string
	sdgEndpoint         string
	jobStart            time.Time
	tlsClientCertPath   string
	tlsClientKeyPath    string
	tlsServerCaCertPath string
	maxSeed             int
}

func NewJobProcessor(ctx context.Context, pool *redis.Pool, svc *s3.Client, logger *zap.SugaredLogger, job, precheckEndpoint, sdgEndpoint, tlsClientCertPath, tlsClientKeyPath, tlsServerCaCertPath string, maxSeed int) *Worker {
	return &Worker{
		ctx:                 ctx,
		pool:                pool,
		svc:                 svc,
		logger:              logger,
		job:                 job,
		precheckEndpoint:    precheckEndpoint,
		sdgEndpoint:         sdgEndpoint,
		jobStart:            time.Now(),
		tlsClientCertPath:   tlsClientCertPath,
		tlsClientKeyPath:    tlsClientKeyPath,
		tlsServerCaCertPath: tlsServerCaCertPath,
		maxSeed:             maxSeed,
	}
}

type IlabConfig struct {
	Generate struct {
		Model string `yaml:"model"`
	} `yaml:"generate"`
}

func init() {
	generateCmd.Flags().StringVarP(&WorkDir, "work-dir", "w", "", "Directory to work in")
	generateCmd.Flags().StringVarP(&VenvDir, "venv-dir", "v", "", "The virtual environment directory")
	generateCmd.Flags().StringVarP(&PreCheckEndpointURL, "precheck-endpoint-url", "e", "http://localhost:8000/v1", "Endpoint hosting the model API. Default, it assumes the model is served locally.")
	generateCmd.Flags().StringVarP(&SdgEndpointURL, "sdg-endpoint-url", "", "http://localhost:8000/v1", "Endpoint hosting the model API. Default, it assumes the model is served locally.")
	generateCmd.Flags().IntVarP(&NumInstructions, "num-instructions", "n", 10, "The number of instructions to generate")
	generateCmd.Flags().StringVarP(&GitRemote, "git-remote", "", "https://github.com/instructlab/taxonomy", "The git remote for the taxonomy repo")
	generateCmd.Flags().StringVarP(&Origin, "origin", "o", "origin", "The origin to fetch from")
	generateCmd.Flags().StringVarP(&GithubUsername, "github-username", "u", "instructlab-bot", "The GitHub username to use for authentication")
	generateCmd.Flags().StringVarP(&GithubToken, "github-token", "g", "", "The GitHub token to use for authentication")
	generateCmd.Flags().StringVarP(&S3Bucket, "s3-bucket", "b", "instruct-lab-bot", "The S3 bucket to use")
	generateCmd.Flags().StringVarP(&AWSRegion, "aws-region", "a", "us-east-2", "The AWS region to use for the S3 Bucket")
	generateCmd.Flags().StringVarP(&TlsClientCertPath, "tls-client-cert", "", "client-tls-crt.pem2", "Path to the TLS client certificate. Defaults to 'client-tls-crt.pem2'")
	generateCmd.Flags().StringVarP(&TlsClientKeyPath, "tls-client-key", "", "client-tls-key.pem2", "Path to the TLS client key. Defaults to 'client-tls-key.pem2'")
	generateCmd.Flags().StringVarP(&TlsServerCaCertPath, "tls-server-ca-cert", "", "server-ca-crt.pem2", "Path to the TLS server CA certificate. Defaults to 'server-ca-crt.pem2'")
	generateCmd.Flags().BoolVarP(&TlsInsecure, "tls-insecure", "", false, "Whether to skip TLS verification")
	generateCmd.Flags().IntVarP(&MaxSeed, "max-seed", "m", 40, "Maximum number of seed Q&A pairs to process to SDG.")
	_ = generateCmd.MarkFlagRequired("github-token")
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Listen for jobs on the 'generate' Redis queue and process them.",
	Run: func(cmd *cobra.Command, args []string) {
		logger := initLogger(Debug)
		sugar := logger.Sugar()
		ctx := cmd.Context()

		sugar.Info("Starting generate worker")

		// Initialize Redis connection pool
		pool := &redis.Pool{
			MaxIdle: 3,
			Dial: func() (redis.Conn, error) {
				return redis.DialContext(ctx, "tcp", RedisHost)
			},
		}
		defer pool.Close()

		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(AWSRegion))
		if err != nil {
			log.Fatalf("unable to load SDK config, %v", err)
		}

		svc := s3.NewFromConfig(cfg)

		sigChan := make(chan os.Signal, 1)
		jobChan := make(chan string)
		stopChan := make(chan struct{})

		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		var wg sync.WaitGroup
		wg.Add(1)
		go func(jobChan chan<- string, stopChan <-chan struct{}) {
			defer wg.Done()
			timer := time.NewTicker(1 * time.Second)
			for {
				select {
				case <-stopChan:
					sugar.Info("Shutting down job listener")
					close(jobChan)
					return
				case <-timer.C:
					conn := pool.Get()
					job, err := redis.String(conn.Do("RPOP", "generate"))
					conn.Close()
					if err == redis.ErrNil {
						continue
					} else if err != nil {
						sugar.Errorf("Could not pop from redis queue: %v", err)
						continue
					}
					jobChan <- job
				}
			}
		}(jobChan, stopChan)

		wg.Add(1)
		go func(ch <-chan os.Signal) {
			defer wg.Done()
			<-ch
			sugar.Info("Shutting down")
			close(stopChan)
		}(sigChan)

		wg.Add(1)
		go func(ch <-chan string) {
			defer wg.Done()
			for job := range ch {
				jp := NewJobProcessor(ctx, pool, svc, sugar, job, PreCheckEndpointURL, SdgEndpointURL, TlsClientCertPath, TlsClientKeyPath, TlsServerCaCertPath, MaxSeed)
				jp.processJob()
			}
		}(jobChan)

		wg.Wait()
	},
}

// runPrecheck runs lab chat against git diffed yaml files
func (w *Worker) runPrecheck(lab, outputDir, modelName string) error {
	workDir := "."
	if WorkDir != "" {
		workDir = WorkDir
	}
	chatlogDir := path.Join(workDir, "data", "chatlogs")

	defer func() {
		// Move everything from chatlogDir to outputDir
		chatlogFiles, err := os.ReadDir(chatlogDir)
		if err != nil {
			w.logger.Errorf("Could not read chatlog directory: %v", err)
			return
		}

		for _, file := range chatlogFiles {
			if err := os.Rename(path.Join(chatlogDir, file.Name()), path.Join(outputDir, file.Name())); err != nil {
				w.logger.Errorf("Could not move file: %v", err)
				return
			}
		}
	}()

	cmd := exec.CommandContext(w.ctx, lab, "diff")
	cmd.Dir = workDir
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		w.logger.Errorf("Could not get stdout pipe: %v", err)
		return err
	}

	w.logger.Debug("Running ilab diff")
	if err := cmd.Start(); err != nil {
		w.logger.Errorf("Could not start command(%s %s): %v", cmd.Path, strings.Join(cmd.Args, " "), err)
		return err
	}

	output, err := io.ReadAll(stdout)
	if err != nil {
		w.logger.Errorf("Could not read stdout: %v", err)
		return err
	}
	outputStr := string(output)
	w.logger.Debugf("Output: %s", outputStr)

	yamlFileCount := 0
	labDiffOutput := strings.Split(outputStr, "\n")

	// Early check for YAML file presence before further processing
	for _, file := range labDiffOutput {
		if strings.HasSuffix(file, ".yaml") {
			yamlFileCount++
		}
	}

	if yamlFileCount == 0 {
		errMsg := "No modified YAML files detected in the PR for precheck"
		w.logger.Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	// Proceed with YAML files processing if they exist
	for _, file := range labDiffOutput {
		if !strings.HasSuffix(file, ".yaml") {
			continue
		}
		filePath := path.Join(workDir, "taxonomy", file)

		f, err := os.Open(filePath)
		if err != nil {
			w.logger.Errorf("Could not open taxonomy file: %v", err)
			return err
		}
		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			w.logger.Error(err)
			return err
		}

		var data map[string]interface{}
		err = yaml.Unmarshal(content, &data)
		if err != nil {
			// Odd are, the PR was not yaml-linted since its invalid yaml failing an unmarshall
			err = fmt.Errorf("the original taxonomy yaml likely did not pass yaml-linting, here is the unmarshalling error: %v", err)
			w.logger.Error(err)
			return err
		}

		// Check if "seed_examples" exists and is a list
		seedExamples, ok := data["seed_examples"].([]interface{})
		if !ok {
			err = fmt.Errorf("seed_examples not found or not a list")
			w.logger.Error(err)
			return err
		}

		for _, item := range seedExamples {
			example, ok := item.(map[interface{}]interface{})
			if !ok {
				w.logger.Error("Invalid seed example format")
				continue
			}
			question, ok := example["question"].(string)
			if !ok {
				w.logger.Error("Question not found or not a string")
				continue
			}

			chatArgs := []string{"chat", "--quick-question", question}
			context, hasContext := example["context"].(string)
			if hasContext {
				chatArgs = append(chatArgs, "--context", context)
			}
			if TlsInsecure {
				chatArgs = append(chatArgs, "--tls-insecure")
			}
			if PreCheckEndpointURL != localEndpoint && modelName != "unknown" {
				chatArgs = append(chatArgs, "--endpoint-url", PreCheckEndpointURL, "--model", modelName)
			}

			cmd := exec.Command(lab, chatArgs...)
			w.logger.Infof("Running the precheck command: %s", cmd.String())
			cmd.Dir = workDir
			cmd.Env = os.Environ()
			cmd.Stderr = os.Stderr
			var out bytes.Buffer
			cmd.Stdout = &out
			err = cmd.Run()
			if err != nil {
				w.logger.Error(err)
				continue
			}

			logData := map[string]interface{}{
				"input": map[string]string{
					"question": question,
				},
				"output": out.String(),
			}
			if hasContext {
				logData["input"].(map[string]string)["context"] = context
			}

			logYAML, err := yaml.Marshal(logData)
			if err != nil {
				w.logger.Errorf("Could not marshal log data to YAML: %v", err)
				continue
			}

			// Generate uniquely timestamped filenames for the combined input/output yaml files
			timestamp := time.Now().Format("2006-01-02T15_04_05")
			logFileName := fmt.Sprintf("chat_%s.yaml", timestamp)
			err = os.WriteFile(path.Join(chatlogDir, logFileName), logYAML, 0644)
			if err != nil {
				w.logger.Errorf("Could not write chatlog to file: %v", err)
				continue
			}

			// Sleep to ensure unique timestamps for filenames
			time.Sleep(1 * time.Second)
		}
	}
	return nil
}

// processJob processes a given job, all jobs start here
func (w *Worker) processJob() {
	sugar := w.logger.With("job", w.job)
	sugar.Infof("Processing job %s", w.job)

	// Get a new Redis connection from the pool for this operation
	conn := w.pool.Get()
	defer conn.Close()

	// Set job status to 'pending'
	if _, err := conn.Do("SET", fmt.Sprintf("jobs:%s:status", w.job), jobStatusRunning); err != nil {
		sugar.Errorf("Could not set job status to pending in redis: %v", err)
		return
	}

	prNumber, err := redis.String(conn.Do("GET", fmt.Sprintf("jobs:%s:pr_number", w.job)))
	if err != nil {
		sugar.Errorf("Could not get pr_number from redis: %v", err)
		return
	}

	jobType, err := redis.String(conn.Do("GET", fmt.Sprintf("jobs:%s:job_type", w.job)))
	if err != nil {
		sugar.Errorf("Could not get job_type from redis: %v", err)
		return
	}
	switch jobType {
	case jobGenerateLocal:
	case jobPreCheck:
	case jobSDG:
	default:
		sugar.Errorf("Unknown job type: %s", jobType)
		return
	}

	// If in test mode, immediately post to the results queue
	if TestMode {
		w.postJobResults("https://example.com", jobType)
		sugar.Info("Job done (test mode)")
		return
	}

	sugar = sugar.With("pr_number", prNumber)

	workDir, err := os.Getwd()
	if err != nil {
		sugar.Errorf("Could not get working directory: %v", err)
		return
	}
	if WorkDir != "" {
		workDir = WorkDir
	}
	taxonomyDir := path.Join(workDir, "taxonomy")
	sugar = sugar.With("work_dir", workDir, "origin", Origin)

	headHash, err := w.gitOperations(sugar, taxonomyDir, prNumber)
	if err != nil {
		w.logger.Errorf("git operations error: %v", err)
		wrappedErr := fmt.Errorf("git operations error: %w", err)
		w.reportJobError(wrappedErr)
		return
	}

	outDirName := fmt.Sprintf("%s-pr-%s-%s", jobType, prNumber, headHash)
	outputDir := path.Join(workDir, outDirName)

	sugar = sugar.With("out_dir", outputDir)
	_ = os.MkdirAll(outputDir, 0755)

	lab := "ilab"
	if VenvDir != "" {
		lab = path.Join(VenvDir, "bin", "ilab")
	}

	var modelName string
	// sdg-svc does not have a models endpoint as yet
	if jobType != jobSDG && PreCheckEndpointURL != localEndpoint {
		var err error
		modelName, err = w.fetchModelName(true)
		if err != nil {
			w.logger.Errorf("Failed to fetch model name: %v", err)
			modelName = "unknown"
		}
	} else {
		modelName = w.getModelNameFromConfig()
	}

	var cmd *exec.Cmd
	switch jobType {
	case jobGenerateLocal:
		// @instructlab-bot generate-local
		// Runs generate on the local worker node
		generateArgs := []string{"generate", "--num-instructions", fmt.Sprintf("%d", NumInstructions), "--output-dir", outputDir}

		cmd = exec.CommandContext(w.ctx, lab, generateArgs...)
		if WorkDir != "" {
			cmd.Dir = WorkDir
		}

		var stderr bytes.Buffer
		// Capture both the ilab err buffer and the os.Stderr
		cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stdout

		sugar.Debug(fmt.Sprintf("Running %s job", jobType))
		// Run the command
		sugar.Infof("Running the generate command: %s", cmd.String())
		if err := cmd.Run(); err != nil {
			detailedErr := fmt.Errorf("Error running command (%s %s): %v. \nDetails: %s", cmd.Path, strings.Join(generateArgs, " "), err, stderr.String())
			sugar.Errorf(detailedErr.Error())
			w.reportJobError(detailedErr)
			return
		}
	case jobPreCheck:
		// @instructlab-bot precheck
		// Runs precheck on a backend node
		err = w.runPrecheck(lab, outputDir, modelName)
		if err != nil {
			sugar.Errorf("Could not run precheck: %v", err)
			w.reportJobError(err)
			return
		}
	case jobSDG:
		// @instructlab-bot generate
		// Runs generate on the SDG backend
		// ilab diff is run since the sdg generation is not part of upstream cli
		cmdDiff := exec.Command("ilab", "diff")
		var stderr bytes.Buffer
		cmdDiff.Stderr = &stderr

		diffOutput, err := cmdDiff.Output()
		if err != nil {
			detailedErr := fmt.Errorf("Failed to execute 'ilab diff': %v. \nDetails: %s", err, stderr.String())
			w.reportJobError(detailedErr)
			sugar.Errorf(detailedErr.Error())
			return
		}

		diffOutputLines := strings.Split(string(diffOutput), "\n")
		// Filter taxonomy files ending in .yaml and prepare them relative to workDir
		var taxonomyFiles []string
		for _, line := range diffOutputLines {
			if strings.HasSuffix(line, ".yaml") {
				relativePath := filepath.Join("taxonomy", line)
				taxonomyFiles = append(taxonomyFiles, relativePath)
			}
		}

		// Uncomment to bypass ilab diff
		//taxonomyFiles, err := discoverGitTaxonomyFiles(taxonomyDir, "main")
		//if err != nil {
		//	sugar.Errorf("Failed to discover taxonomy files: %v", err)
		//	return
		//}

		if len(taxonomyFiles) == 0 {
			sugar.Info("No taxonomy files were changed.")
			return
		}

		// Process each YAML file and filter questions if over the max seed
		filteredFiles := []string{}
		for _, file := range taxonomyFiles {
			f, err := os.Open(file)
			if err != nil {
				sugar.Errorf("Failed to open file: %v", err)
				continue
			}
			defer f.Close()

			decoder := yaml.NewDecoder(f)
			var data map[string]interface{}
			if err := decoder.Decode(&data); err != nil {
				sugar.Errorf("Failed to decode YAML file: %v", err)
				continue
			}

			if seedExamples, ok := data["seed_examples"].([]interface{}); ok && len(seedExamples) > w.maxSeed {
				originalCount := len(seedExamples)
				data["seed_examples"] = seedExamples[:w.maxSeed]
				outputData, err := yaml.Marshal(data)
				if err != nil {
					sugar.Errorf("Failed to re-marshal filtered YAML data: %v", err)
					continue
				}

				// Write the modified content back to a new file to pass to datagenSvc instead of the original diff
				filteredQNA, err := os.CreateTemp("", "filtered-*.yaml")
				if err != nil {
					sugar.Errorf("Failed to create temporary file: %v", err)
					continue
				}
				defer filteredQNA.Close()

				if _, err = filteredQNA.Write(outputData); err != nil {
					sugar.Errorf("Failed to write filtered data to the new QNA file: %v", err)
					continue
				}
				sugar.Infof("Trimmed %s from %d to %d Q&A pairs", file, originalCount, w.maxSeed)

				filteredFiles = append(filteredFiles, filteredQNA.Name())
			} else {
				// No filtering needed, use the original file
				filteredFiles = append(filteredFiles, file)
			}
		}

		// Generate data with potentially filtered files
		outputFiles, err := w.datagenSvc(filteredFiles, outputDir, NumInstructions)
		if err != nil {
			sugar.Errorf("Failed to generate data: %v", err)
			w.reportJobError(err)
			return
		}
		sugar.Infof("Generated data written to: %v", outputFiles)

	default:
		sugar.Errorf("Unknown job type: %s", jobType)
		return
	}

	// handle file operations and get the index file key
	indexUpKey := w.handleOutputFiles(outputDir, prNumber, outDirName)
	if indexUpKey == "" {
		sugar.Errorf("Failed to handle output files correctly")
		return
	}

	indexPublicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", S3Bucket, AWSRegion, indexUpKey)

	// Notify the "results" queue that the job is done with the public URL
	w.postJobResults(indexPublicURL, jobType)
	sugar.Infof("Job done")
}

// gitOperations handles the Git-related operations for a job and returns the head hash
func (w *Worker) gitOperations(sugar *zap.SugaredLogger, taxonomyDir string, prNumber string) (string, error) {
	sugar.Debug("Opening taxonomy git repo")

	var r *git.Repository
	if _, err := os.Stat(taxonomyDir); os.IsNotExist(err) {
		sugar.Warnf("Taxonomy directory does not exist, cloning from %s", GitRemote)
		r, err = git.PlainClone(taxonomyDir, false, &git.CloneOptions{
			URL: GitRemote,
			Auth: &githttp.BasicAuth{
				Username: GithubUsername,
				Password: GithubToken,
			},
		})
		if err != nil {
			return "", fmt.Errorf("could not clone taxonomy git repo: %v", err)
		}
	} else {
		r, err = git.PlainOpen(taxonomyDir)
		if err != nil {
			return "", fmt.Errorf("could not open taxonomy git repo: %v", err)
		}
	}

	retryFetch := func() error {
		var lastErr error
		for attempt := 1; attempt <= gitMaxRetries; attempt++ {
			sugar.Debug("Fetching from origin")
			err := r.Fetch(&git.FetchOptions{
				RemoteName: Origin,
				Auth: &githttp.BasicAuth{
					Username: GithubUsername,
					Password: GithubToken,
				},
			})
			if err == nil {
				return nil
			}
			lastErr = err
			if attempt < gitMaxRetries {
				sugar.Infof("Retrying fetching updates, attempt %d/%d", attempt+1, gitMaxRetries)
				time.Sleep(gitRetryDelay)
			}
		}
		return lastErr
	}
	if err := retryFetch(); err != nil && err != git.NoErrAlreadyUpToDate {
		return "", fmt.Errorf("could not fetch from origin: %v", err)
	}

	wt, err := r.Worktree()
	if err != nil {
		return "", fmt.Errorf("could not get worktree: %v", err)
	}

	sugar.Debug("Checking out main")
	// Retry mechanism for checking out main branch
	retryCheckout := func() error {
		var lastErr error
		for attempt := 1; attempt <= gitMaxRetries; attempt++ {
			err := wt.Checkout(&git.CheckoutOptions{
				Branch: plumbing.ReferenceName(fmt.Sprintf("refs/remotes/%s/main", Origin)),
			})
			if err == nil {
				return nil
			}
			lastErr = err
			if attempt < gitMaxRetries {
				sugar.Infof("Retrying checkout of main, attempt %d/%d", attempt+1, gitMaxRetries)
				time.Sleep(gitRetryDelay)
			}
		}
		return lastErr
	}

	if err := retryCheckout(); err != nil {
		return "", fmt.Errorf("could not checkout main after retries: %v", err)
	}

	prBranchName := fmt.Sprintf("pr-%s", prNumber)
	if _, err := r.Branch(prBranchName); err == nil {
		err = r.DeleteBranch(prBranchName)
		if err != nil {
			return "", fmt.Errorf("could not delete branch %s: %v", prBranchName, err)
		}
	}

	sugar = sugar.With("pr_branch_name", prBranchName)
	sugar.Debug("Fetching PR branch")
	refspec := gitconfig.RefSpec(fmt.Sprintf("refs/pull/%s/head:refs/heads/%s", prNumber, prBranchName))
	err = r.Fetch(&git.FetchOptions{
		RemoteName: Origin,
		RefSpecs:   []gitconfig.RefSpec{refspec},
		Auth: &githttp.BasicAuth{
			Username: "instructlab-bot",
			Password: GithubToken,
		},
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", fmt.Errorf("could not fetch PR branch: %v", err)
	}

	sugar.Debug("Checking out PR branch")
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(prBranchName),
	})
	if err != nil {
		return "", fmt.Errorf("could not checkout PR branch: %v", err)
	}

	head, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("could not get HEAD: %v", err)
	}

	return head.Hash().String(), nil
}

// postJobResults posts the results of a job to a Redis queue
func (w *Worker) postJobResults(URL, jobType string) {
	conn := w.pool.Get()
	defer conn.Close()

	// Calculate the job duration and round it up
	jobDuration := time.Since(w.jobStart).Seconds()
	roundedDuration := math.Ceil(jobDuration)
	w.logger.Infof("Job took %.0fs to run", roundedDuration)

	if _, err := conn.Do("SET", fmt.Sprintf("jobs:%s:duration", w.job), roundedDuration); err != nil {
		w.logger.Errorf("Could not set job duration in redis: %v", err)
	}

	if _, err := conn.Do("SET", fmt.Sprintf("jobs:%s:status", w.job), jobStatusSuccess); err != nil {
		w.logger.Errorf("Could not set job status in redis: %v", err)
	}

	if _, err := conn.Do("SET", fmt.Sprintf("jobs:%s:s3_url", w.job), URL); err != nil {
		w.logger.Errorf("Could not set s3_url in redis: %v", err)
	}

	modelName := w.determineModelName(jobType)

	if _, err := conn.Do("SET", fmt.Sprintf("jobs:%s:model_name", w.job), modelName); err != nil {
		w.logger.Errorf("Could not set model name in redis: %v", err)
	}

	if _, err := conn.Do("LPUSH", "results", w.job); err != nil {
		w.logger.Errorf("Could not push to redis queue: %v", err)
	}
}

// getModelNameFromConfig retrieves the model name from the config file or precheckEndpoint
func (w *Worker) getModelNameFromConfig() string {
	cfgData, err := os.ReadFile(ilabConfigPath)
	if err != nil {
		return "unknown"
	}

	var cfg IlabConfig
	err = yaml.Unmarshal(cfgData, &cfg)
	if err != nil || cfg.Generate.Model == "" {
		return "unknown"
	}

	return cfg.Generate.Model
}

// fetchModelName hits the defined precheckEndpoint with "/models" appended to extract the model name.
// If fullName is true, it returns the entire ID value; if false, it returns the parsed out name after the double hyphens.
func (w *Worker) fetchModelName(fullName bool) (string, error) {
	// Ensure the endpoint URL ends with "/models"
	endpoint := w.precheckEndpoint
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
	endpoint += "models"

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	http.DefaultTransport.(*http.Transport).TLSHandshakeTimeout = 10 * time.Second
	http.DefaultTransport.(*http.Transport).ResponseHeaderTimeout = 10 * time.Second
	http.DefaultTransport.(*http.Transport).ExpectContinueTimeout = 1 * time.Second

	req, err := http.NewRequestWithContext(w.ctx, "GET", endpoint, nil)
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

// reportJobError push app errors into the redis job 'errors' key
func (w *Worker) reportJobError(err error) {
	conn := w.pool.Get()
	defer conn.Close()

	if _, err := conn.Do("SET", fmt.Sprintf("jobs:%s:errors", w.job), err.Error()); err != nil {
		w.logger.Errorf("Failed to set the error for job %s: %v", w.job, err)
		return
	}

	if _, err := conn.Do("SET", fmt.Sprintf("jobs:%s:status", w.job), jobStatusError); err != nil {
		w.logger.Errorf("Could not set job status in redis: %v", err)
	}

	if _, err := conn.Do("LPUSH", "results", w.job); err != nil {
		w.logger.Errorf("Could not push error results to redis queue: %v", err)
		return
	}
}

// determineModelName decides the model name based on jobType and configuration.
func (w *Worker) determineModelName(jobType string) string {
	if jobType == jobSDG {
		return "sdg service backend"
	}

	// precheck is the only case we use a remote OpenAI endpoint right now
	if PreCheckEndpointURL != localEndpoint && jobType == jobPreCheck {
		modelName, err := w.fetchModelName(false)
		if err != nil {
			w.logger.Errorf("Failed to fetch model name: %v", err)
			return "unknown"
		}
		return modelName
	}

	return w.getModelNameFromConfig()
}

// datagenSvc generates data for the given taxonomy files and writes the results to the specified output directory.
func (w *Worker) datagenSvc(taxonomyFiles []string, outputDir string, numSamples int) ([]string, error) {
	var outputFiles []string
	httpClient, err := w.createTLSHttpClient()
	if err != nil {
		return nil, err
	}

	for _, tf := range taxonomyFiles {
		tfData, err := os.ReadFile(tf)
		if err != nil {
			return nil, fmt.Errorf("failed to read taxonomy file '%s': %w", tf, err)
		}

		var jsonData []byte
		var requestURL string

		if strings.Contains(tf, "taxonomy/knowledge") {
			tfMap, err := w.createKnowledgePostJSON(tfData, numSamples)
			if err != nil {
				return nil, err
			}
			jsonData, err = json.Marshal(tfMap)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal knowledge JSON post: %w", err)
			}
			// Adjust endpoint for knowledge
			requestURL = strings.Replace(w.sdgEndpoint, "skill", "knowledge", -1)
		} else {
			tfMap, err := w.createSkillsPostJSON(tfData, numSamples)
			if err != nil {
				return nil, err
			}
			jsonData, err = json.Marshal(tfMap)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal skills JSON post: %w", err)
			}
			// Use the existing endpoint for skills
			requestURL = w.sdgEndpoint
		}

		// Modify the endpoint URL if the filepath includes "taxonomy/knowledge"
		if strings.Contains(tf, "taxonomy/knowledge") {
			requestURL = strings.Replace(requestURL, "skill", "knowledge", -1)
		}

		request, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json")

		w.logger.Infof("SDG Post Details: %v", request)

		response, err := httpClient.Do(request)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}
		defer response.Body.Close()

		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code %d: %s", response.StatusCode, string(responseBody))
		}

		outputPath := path.Join(outputDir, fmt.Sprintf("sdg_%d_%s.json", time.Now().Unix(), filepath.Base(tf)))
		if err := os.WriteFile(outputPath, responseBody, 0644); err != nil {
			return nil, fmt.Errorf("failed to write output file: %w", err)
		}

		outputFiles = append(outputFiles, outputPath)
	}

	return outputFiles, nil
}

func (w *Worker) createTLSHttpClient() (*http.Client, error) {
	certs, err := tls.LoadX509KeyPair(w.tlsClientCertPath, w.tlsClientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate/key: %w", err)
	}
	caCert, err := os.ReadFile(w.tlsServerCaCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
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
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	return httpClient, nil
}

// createKnowledgePostJSON convert a skills taxonomy file from YAML to json
func (w *Worker) createSkillsPostJSON(tfData []byte, numSamples int) (map[string]interface{}, error) {
	var tfMapInterface map[interface{}]interface{}
	if err := yaml.Unmarshal(tfData, &tfMapInterface); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}
	tfMap := interfaceMapToStringMap(tfMapInterface).(map[string]interface{})

	tfMap["mm_model_id"] = sdgModel
	tfMap["num_samples"] = numSamples
	return tfMap, nil
}

// createKnowledgePostJSON convert a knowledge taxonomy file from YAML to json
func (w *Worker) createKnowledgePostJSON(tfData []byte, numSamples int) (map[string]interface{}, error) {
	var tfMapInterface map[interface{}]interface{}
	if err := yaml.Unmarshal(tfData, &tfMapInterface); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}
	tfMap := interfaceMapToStringMap(tfMapInterface).(map[string]interface{})

	tfMap["mm_model_id"] = sdgModel
	tfMap["num_samples"] = numSamples

	// Handle the 'document' field if it exists
	if doc, ok := tfMap["document"].(map[string]interface{}); ok {
		docMap := make(map[string]interface{})
		if repo, repoOk := doc["repo"].(string); repoOk {
			docMap["repo"] = repo
		}
		if commit, commitOk := doc["commit"].(string); commitOk {
			docMap["commit"] = commit
		}
		if patterns, patternsOk := doc["patterns"].([]interface{}); patternsOk {
			// Ensure patterns are in the correct format (slice of strings)
			stringPatterns := make([]string, 0)
			for _, pattern := range patterns {
				if strPattern, isStr := pattern.(string); isStr {
					stringPatterns = append(stringPatterns, strPattern)
				}
			}
			docMap["patterns"] = stringPatterns
		}
		// Add the parsed 'document' map back to the main tfMap
		tfMap["document"] = docMap
	}

	return tfMap, nil
}

func interfaceMapToStringMap(in interface{}) interface{} {
	switch x := in.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[fmt.Sprint(k)] = interfaceMapToStringMap(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = interfaceMapToStringMap(v)
		}
	}
	return in
}

func (w *Worker) handleOutputFiles(outputDir, prNumber, outDirName string) string {
	sugar := w.logger.With("directory", outputDir)

	items, err := os.ReadDir(outputDir)
	if err != nil {
		sugar.Errorf("Could not read output directory: %v", err)
		return ""
	}

	publicFiles := make([]map[string]string, 0)
	// Append job ID to outDirName for uniqueness
	jobSpecificOutDirName := fmt.Sprintf("%s-job-%s", outDirName, w.job)

	for _, item := range items {
		filename := item.Name()
		fullPath := path.Join(outputDir, filename)
		info, err := item.Info()
		if err != nil {
			sugar.Errorf("Could not get info for file %s: %v", filename, err)
			continue
		}

		// Only process files created after the job start time
		if info.ModTime().After(w.jobStart) {
			if strings.HasSuffix(filename, ".json") || strings.HasSuffix(filename, ".jsonl") {
				formattedJSONKey := generateFormattedJSON(w.ctx, outputDir, filename, w.svc, w.logger)
				if formattedJSONKey != "" {
					formattedJSONURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", S3Bucket, AWSRegion, formattedJSONKey)
					publicFiles = append(publicFiles, map[string]string{
						"name": filename + jsonViewerFilenameSuffix,
						"url":  formattedJSONURL,
					})
				}
			}

			formattedYAMLKey := generateFormattedYAML(w.ctx, outputDir, filename, w.svc, w.logger)
			if formattedYAMLKey != "" {
				yamlFilename := strings.TrimSuffix(filename, path.Ext(filename)) + ".yaml-viewer"
				formattedYAMLURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", S3Bucket, AWSRegion, formattedYAMLKey)
				publicFiles = append(publicFiles, map[string]string{
					"name": yamlFilename + ".html",
					"url":  formattedYAMLURL,
				})
			}

			var contentType string
			if strings.HasSuffix(filename, ".json") || strings.Contains(filename, "json-viewer.html") {
				contentType = "application/json-lines+json"
			} else {
				contentType = "text/plain"
			}

			// Upload the job file and add it to the publicFiles list
			file, err := os.Open(fullPath)
			if err != nil {
				sugar.Errorf("Could not open file: %v", err)
				continue
			}
			defer file.Close()

			upKey := fmt.Sprintf("%s/%s", jobSpecificOutDirName, filename)
			_, err = w.svc.PutObject(w.ctx, &s3.PutObjectInput{
				Bucket:      aws.String(S3Bucket),
				Key:         aws.String(upKey),
				Body:        file,
				ContentType: aws.String(contentType),
			})
			if err != nil {
				sugar.Errorf("Could not upload file to S3: %v", err)
				continue
			}
			publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", S3Bucket, AWSRegion, upKey)
			publicFiles = append(publicFiles, map[string]string{
				"name": filename,
				"url":  publicURL,
			})
		}
	}

	if len(publicFiles) == 0 {
		return ""
	}

	// Generate index.html
	indexFile, err := os.Create(path.Join(outputDir, "index.html"))
	if err != nil {
		sugar.Errorf("Could not create index.html: %v", err)
		return ""
	}
	defer indexFile.Close()

	if err := generateIndexHTML(indexFile, prNumber, publicFiles); err != nil {
		sugar.Errorf("Could not generate index.html: %v", err)
		return ""
	}

	// Re-open index file for uploading
	indexFile, err = os.Open(path.Join(outputDir, "index.html"))
	if err != nil {
		sugar.Errorf("Could not re-open index.html: %v", err)
		return ""
	}
	defer indexFile.Close()

	indexUpKey := fmt.Sprintf("%s/index.html", jobSpecificOutDirName)
	_, err = w.svc.PutObject(w.ctx, &s3.PutObjectInput{
		Bucket:      aws.String(S3Bucket),
		Key:         aws.String(indexUpKey),
		Body:        indexFile,
		ContentType: aws.String("text/html"),
	})
	if err != nil {
		sugar.Errorf("Could not upload index.html to S3: %v", err)
		return ""
	}

	return indexUpKey
}

/* Uncomment to bypass ilab diff (temporary until upstream files are validated prior to merge)
// discoverGitTaxonomyFiles discovers new or modified YAML taxonomy files in the specified Git repository.
// This temporarily replaces ilab diff since that fails on most files because it's hard to validate when most taxonomies
// to test with fail when using ilab diff.
func discoverGitTaxonomyFiles(repoPath string, baseBranchName string) ([]string, error) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	// Get the HEAD commit
	headRef, err := r.Head()
	if err != nil {
		return nil, err
	}
	headCommit, err := r.CommitObject(headRef.Hash())
	if err != nil {
		return nil, err
	}

	// Get the HEAD commit tree
	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, err
	}

	// Get the base branch commit
	baseRef, err := r.Reference(plumbing.NewBranchReferenceName(baseBranchName), true)
	if err != nil {
		return nil, err
	}
	baseCommit, err := r.CommitObject(baseRef.Hash())
	if err != nil {
		return nil, err
	}

	// Get the base commit tree
	baseTree, err := baseCommit.Tree()
	if err != nil {
		return nil, err
	}

	// Get the diff between the base and HEAD commit trees
	diff, err := object.DiffTree(baseTree, headTree)
	if err != nil {
		return nil, err
	}

	// Generate a patch from the diff
	patch, err := diff.Patch()
	if err != nil {
		return nil, err
	}

	var taxonomyFiles []string
	for _, filePatch := range patch.FilePatches() {
		_, to := filePatch.Files()
		if to == nil {
			continue // Deleted file, skip it
		}
		filePath := to.Path()
		// Parse out yaml files
		for _, folder := range TaxonomyFolders {
			if strings.HasPrefix(filePath, folder+"/") && strings.HasSuffix(filePath, ".yaml") {
				taxonomyFiles = append(taxonomyFiles, filePath)
				break
			}
		}
	}

	return taxonomyFiles, nil
}
*/
