package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/gomodule/redigo/redis"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

var (
	WorkDir         string
	VenvDir         string
	EndpointURL     string
	NumInstructions int
	Origin          string
	GithubToken     string
	S3Bucket        string
	AWSRegion       string
)

const (
	gitMaxRetries  = 3
	gitRetryDelay  = 2 * time.Second
	ilabConfigPath = "config.yaml"
)

type IlabConfig struct {
	Generate struct {
		Model string `yaml:"model"`
	} `yaml:"generate"`
}

func init() {
	generateCmd.Flags().StringVarP(&WorkDir, "work-dir", "w", "", "Directory to work in")
	generateCmd.Flags().StringVarP(&VenvDir, "venv-dir", "v", "", "The virtual environment directory")
	generateCmd.Flags().StringVarP(&EndpointURL, "endpoint-url", "e", "http://localhost:8000/v1", "Endpoint hosting the model API. Default, it assumes the model is served locally.")
	generateCmd.Flags().IntVarP(&NumInstructions, "num-instructions", "n", 10, "The number of instructions to generate")
	generateCmd.Flags().StringVarP(&Origin, "origin", "o", "origin", "The origin to fetch from")
	generateCmd.Flags().StringVarP(&GithubToken, "github-token", "g", "", "The GitHub token to use for authentication")
	generateCmd.Flags().StringVarP(&S3Bucket, "s3-bucket", "b", "instruct-lab-bot", "The S3 bucket to use")
	generateCmd.Flags().StringVarP(&AWSRegion, "aws-region", "a", "us-east-2", "The AWS region to use for the S3 Bucket")
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
		// Connect to Redis
		conn, err := redis.DialContext(ctx, "tcp", RedisHost)
		if err != nil {
			sugar.Fatal("Could not connect to Redis")
		}
		defer conn.Close()

		// Using the SDK's default configuration, loading additional config
		// and credentials values from the environment variables, shared
		// credentials, and shared configuration files
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(AWSRegion))
		if err != nil {
			log.Fatalf("unable to load SDK config, %v", err)
		}

		// Using the Config value, create the S3 client
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
					// Wait for a job on the "generate" Redis queue
					job, err := redis.String(conn.Do("RPOP", "generate"))
					if err == redis.ErrNil {
						// No job available
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
			for job := range jobChan {
				processJob(ctx, conn, svc, sugar, job)
			}
		}(jobChan)

		wg.Wait()
	},
}

func runPrecheck(ctx context.Context, sugar *zap.SugaredLogger, lab, outputDir string) error {
	workDir := "."
	if WorkDir != "" {
		workDir = WorkDir
	}
	chatlogDir := path.Join(workDir, "data", "chatlogs")

	defer func() {
		// Move everything from chatlogDir to outputDir
		chatlogFiles, err := os.ReadDir(chatlogDir)
		if err != nil {
			sugar.Errorf("Could not read chatlog directory: %v", err)
			return
		}

		for _, file := range chatlogFiles {
			if err := os.Rename(path.Join(chatlogDir, file.Name()), path.Join(outputDir, file.Name())); err != nil {
				sugar.Errorf("Could not move file: %v", err)
				return
			}
		}
	}()

	cmd := exec.CommandContext(ctx, lab, "diff")
	cmd.Dir = workDir
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sugar.Errorf("Could not get stdout pipe: %v", err)
		return err
	}

	sugar.Debug("Running ilab diff")
	if err := cmd.Start(); err != nil {
		sugar.Errorf("Could not start command(%s %s): %v", cmd.Path, strings.Join(cmd.Args, " "), err)
		return err
	}

	// Get an array of lines from the output
	output, err := io.ReadAll(stdout)
	if err != nil {
		sugar.Errorf("Could not read stdout: %v", err)
		return err
	}
	outputStr := string(output)
	sugar.Debugf("Output: %s", outputStr)
	labDiffOutput := strings.Split(outputStr, "\n")

	for _, file := range labDiffOutput {
		if !strings.HasSuffix(file, ".yaml") {
			continue
		}
		filePath := path.Join(WorkDir, "taxonomy", file)

		f, err := os.Open(filePath)
		if err != nil {
			sugar.Errorf("Could not open taxonomy file: %v", err)
			return err
		}
		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			sugar.Error(err)
			return err
		}

		var data map[string]interface{}
		err = yaml.Unmarshal(content, &data)
		if err != nil {
			sugar.Error(err)
			return err
		}

		// Check if "seed_examples" exists and is a list
		seedExamples, ok := data["seed_examples"].([]interface{})
		if !ok {
			err = fmt.Errorf("seed_examples not found or not a list")
			sugar.Error(err)
			return err
		}

		for _, item := range seedExamples {
			question, ok := item.(map[interface{}]interface{})["question"].(string)
			if !ok {
				err = fmt.Errorf("question not found or not a string")
				sugar.Error(err)
				return err
			}

			cmd := exec.Command(lab, "chat", "--quick-question", question)
			cmd.Dir = workDir
			cmd.Env = os.Environ()
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			err = cmd.Run()
			if err != nil {
				sugar.Error(err)
				return err
			}
		}
	}
	return nil
}

func processJob(ctx context.Context, conn redis.Conn, svc *s3.Client, logger *zap.SugaredLogger, job string) {
	// Process the job
	sugar := logger.With("job", job)
	sugar.Info("Processing job")

	prNumber, err := redis.String(conn.Do("GET", fmt.Sprintf("jobs:%s:pr_number", job)))
	if err != nil {
		sugar.Errorf("Could not get pr_number from redis: %v", err)
		return
	}

	jobType, err := redis.String(conn.Do("GET", fmt.Sprintf("jobs:%s:job_type", job)))
	if err != nil {
		sugar.Errorf("Could not get job_type from redis: %v", err)
		return
	}
	switch jobType {
	case "generate":
	case "precheck":
	default:
		sugar.Errorf("Unknown job type: %s", jobType)
		return
	}

	// If in test mode, immediately post to the results queue
	if TestMode {
		postJobResults(job, conn, sugar, "https://example.com")
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

	sugar.Debug("Opening taxonomy git repo")
	r, err := git.PlainOpen(taxonomyDir)
	if err != nil {
		sugar.Errorf("Could not open taxonomy git repo: %v", err)
		return
	}

	retryFetch := func() error {
		var lastErr error
		for attempt := 1; attempt <= gitMaxRetries; attempt++ {
			sugar.Debug("Fetching from origin")
			err = r.Fetch(&git.FetchOptions{
				RemoteName: Origin,
				Auth: &http.BasicAuth{
					Username: "instruct-lab-bot",
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
		sugar.Errorf("Could not fetch from origin: %v", err)
		return
	}

	w, err := r.Worktree()
	if err != nil {
		sugar.Errorf("Could not get worktree: %v", err)
		return
	}

	sugar.Debug("Checking out main")
	// Retry mechanism for checking out main branch
	retryCheckout := func() error {
		var lastErr error
		for attempt := 1; attempt <= gitMaxRetries; attempt++ {
			err := w.Checkout(&git.CheckoutOptions{
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
		sugar.Errorf("Could not checkout main after retries: %v", err)
		return
	}

	prBranchName := fmt.Sprintf("pr-%s", prNumber)
	if _, err := r.Branch(prBranchName); err == nil {
		err = r.DeleteBranch(prBranchName)
		if err != nil {
			sugar.Errorf("Could not delete branch %s: %v", prBranchName, err)
			return
		}
	}

	sugar = sugar.With("pr_branch_name", prBranchName)
	sugar.Debug("Fetching PR branch")
	refspec := gitconfig.RefSpec(fmt.Sprintf("refs/pull/%s/head:refs/heads/%s", prNumber, prBranchName))
	err = r.Fetch(&git.FetchOptions{
		RemoteName: Origin,
		RefSpecs:   []gitconfig.RefSpec{refspec},
		Auth: &http.BasicAuth{
			Username: "instruct-lab-bot",
			Password: GithubToken,
		},
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		sugar.Errorf("Could not fetch PR branch: %v", err)
		return
	}

	sugar.Debug("Checking out PR branch")
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(prBranchName),
	})
	if err != nil {
		sugar.Errorf("Could not checkout PR branch: %v", err)
		return
	}

	head, err := r.Head()
	if err != nil {
		sugar.Errorf("Could not get HEAD: %v", err)
		return
	}

	outDirName := fmt.Sprintf("%s-pr-%s-%s", jobType, prNumber, head.Hash())
	outputDir := path.Join(workDir, outDirName)

	sugar = sugar.With("out_dir", outputDir)
	_ = os.MkdirAll(outputDir, 0755)

	lab := "ilab"
	if VenvDir != "" {
		lab = path.Join(VenvDir, "bin", "ilab")
	}

	var cmd *exec.Cmd
	switch jobType {
	case "generate":
		cmd = exec.CommandContext(ctx, lab, "generate", "--num-instructions", fmt.Sprintf("%d", NumInstructions), "--output-dir", outputDir, "--endpoint-url", EndpointURL)
		if WorkDir != "" {
			cmd.Dir = WorkDir
		}

		cmd.Env = os.Environ()
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout

		sugar.Debug(fmt.Sprintf("Running %s job", jobType))
		// Run the command
		if err := cmd.Run(); err != nil {
			sugar.Errorf("Could not run command(%s %s): %v", cmd.Path, strings.Join(cmd.Args, " "), err)
			return
		}
	case "precheck":
		err = runPrecheck(ctx, sugar, lab, outputDir)
		if err != nil {
			sugar.Errorf("Could not run precheck: %v", err)
			return
		}
	default:
		sugar.Errorf("Unknown job type: %s", jobType)
		return
	}

	items, err := os.ReadDir(outputDir)
	if err != nil {
		sugar.Errorf("Could not read output directory: %v", err)
		return
	}

	publicFiles := make([]map[string]string, 0)

	for _, item := range items {
		file, err := os.Open(path.Join(outputDir, item.Name()))
		if err != nil {
			sugar.Errorf("Could not open file: %v", err)
			return
		}
		defer file.Close()
		upKey := fmt.Sprintf("%s/%s", outDirName, item.Name())
		_, err = svc.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(S3Bucket),
			Key:         aws.String(upKey),
			Body:        file,
			ContentType: aws.String("application/json-lines+json"),
		})
		if err != nil {
			sugar.Errorf("Could not upload file to S3: %v", err)
			return
		}

		publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", S3Bucket, AWSRegion, upKey)
		publicFiles = append(publicFiles, map[string]string{
			"name": item.Name(),
			"url":  publicURL,
		})
	}

	indexFile, err := os.Create(path.Join(outputDir, "index.html"))
	if err != nil {
		sugar.Errorf("Could not create index.html: %v", err)
		return
	}
	if err := generateIndexHTML(indexFile, prNumber, publicFiles); err != nil {
		sugar.Errorf("Could not generate index.html: %v", err)
		indexFile.Close()
		return
	}
	indexFile.Close()

	indexFile, err = os.Open(indexFile.Name())
	if err != nil {
		sugar.Errorf("Could not re-open index.html: %v", err)
		return
	}
	defer indexFile.Close()

	upKey := fmt.Sprintf("%s/index.html", outDirName)
	_, err = svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(S3Bucket),
		Key:         aws.String(upKey),
		Body:        indexFile,
		ContentType: aws.String("text/html"),
	})
	if err != nil {
		sugar.Errorf("Could not upload index.html to S3: %v", err)
		return
	}

	indexPublicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", S3Bucket, AWSRegion, upKey)

	// Notify the "results" queue that the job is done with the public URL
	postJobResults(job, conn, sugar, indexPublicURL)
	sugar.Infof("Job done")
}

func postJobResults(job string, conn redis.Conn, logger *zap.SugaredLogger, URL string) {
	if _, err := conn.Do("SET", fmt.Sprintf("jobs:%s:s3_url", job), URL); err != nil {
		logger.Errorf("Could not set s3_url in redis: %v", err)
	}

	modelName := getModelNameFromConfig()

	if _, err := conn.Do("SET", fmt.Sprintf("jobs:%s:model_name", job), modelName); err != nil {
		logger.Errorf("Could not set model name in redis: %v", err)
	}

	if _, err := conn.Do("LPUSH", "results", job); err != nil {
		logger.Errorf("Could not push to redis queue: %v", err)
	}
}

func generateIndexHTML(indexFile *os.File, prNumber string, presignedFiles []map[string]string) error {
	const INDEX_HTML = `
<!DOCTYPE html>
<html>
<head>
    <title>Generated Data for {{ .Name }}</title>
    <style>
        :root {
            --primary-color: #007bff;
            --hover-color: #0056b3;
            --text-color: #333;
            --background-color: #f8f9fa;
            --link-color: #0066cc;
            --link-hover-color: #0044cc;
        }

        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background-color: var(--background-color);
            margin: 0;
            padding: 20px;
            color: var(--text-color);
        }

        h1 {
            color: var(--primary-color);
            text-align: center;
            margin-bottom: 2rem;
        }

        ul {
            list-style-type: none;
            padding: 0;
            max-width: 600px;
            margin: auto;
        }

        li {
            background-color: #fff;
            margin-bottom: 10px;
            padding: 10px;
            border-radius: 5px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            transition: transform 0.2s ease-in-out;
        }

        li:hover {
            transform: translateY(-3px);
        }

        a {
            color: var(--link-color);
            text-decoration: none;
            font-weight: 500;
        }

        a:hover {
            color: var(--link-hover-color);
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <h1>Generated Data for {{ .Name }}</h1>
	<ul>
	{{- range .Files}}
		<li><a href="{{ .url }}">{{ .name }}</a></li>
	{{- end }}
	</ul>
</body>
</html>`

	tmpl := template.Must(template.New("index").Parse(INDEX_HTML))
	data := struct {
		Name  string
		Files []map[string]string
	}{
		Name:  fmt.Sprintf("PR %s", prNumber),
		Files: presignedFiles,
	}

	return tmpl.Execute(indexFile, data)
}

func getModelNameFromConfig() string {
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
