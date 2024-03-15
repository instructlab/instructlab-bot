package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
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
)

var (
	WorkDir         string
	VenvDir         string
	NumInstructions int
	Origin          string
)

func init() {
	generateCmd.Flags().StringVarP(&WorkDir, "work-dir", "w", "", "Directory to work in")
	generateCmd.Flags().StringVarP(&VenvDir, "venv-dir", "v", "", "The virtual environment directory")
	generateCmd.Flags().IntVarP(&NumInstructions, "num-instructions", "n", 10, "The number of instructions to generate")
	generateCmd.Flags().StringVarP(&Origin, "origin", "o", "origin", "The origin to fetch from")
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Listen for jobs on the 'generate' Redis queue and process them.",
	Run: func(cmd *cobra.Command, args []string) {
		logger := initLogger(Debug)
		sugar := logger.Sugar()
		ctx := cmd.Context()

		sugar.Info("Starting worker")
		// Connect to Redis
		conn, err := redis.DialContext(ctx, "tcp", RedisHost)
		if err != nil {
			sugar.Fatal("Could not connect to Redis")
		}
		defer conn.Close()

		// Using the SDK's default configuration, loading additional config
		// and credentials values from the environment variables, shared
		// credentials, and shared configuration files
		cfg, err := awsconfig.LoadDefaultConfig(context.TODO(), awsconfig.WithRegion("us-west-2"))
		if err != nil {
			log.Fatalf("unable to load SDK config, %v", err)
		}

		// Using the Config value, create the DynamoDB client
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
				wg.Add(1)
				go func(job string) {
					defer wg.Done()
					processJob(ctx, conn, svc, sugar, job)
				}(job)
			}
		}(jobChan)

		wg.Wait()
	},
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

	sugar.Debug("Fetching from origin")
	err = r.Fetch(&git.FetchOptions{
		RemoteName: Origin,
		Auth: &http.BasicAuth{
			Username: "instruct-lab-bot",
			Password: os.Getenv("GITHUB_TOKEN"),
		},
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		sugar.Errorf("Could not fetch from origin: %v", err)
		return
	}

	w, err := r.Worktree()
	if err != nil {
		sugar.Errorf("Could not get worktree: %v", err)
		return
	}

	sugar.Debug("Checking out main")
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(fmt.Sprintf("refs/remotes/%s/main", Origin)),
	})
	if err != nil {
		sugar.Errorf("Could not checkout main: %v", err)
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
			Password: os.Getenv("GITHUB_TOKEN"),
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

	outDirName := fmt.Sprintf("generate-pr-%s-%s", prNumber, head.Name().Short())
	outputDir := path.Join(workDir, outDirName)

	_ = os.MkdirAll(outputDir, 0755)

	lab := "lab"
	if VenvDir != "" {
		lab = path.Join(VenvDir, "bin", "lab")
	}

	cmd := exec.CommandContext(ctx, lab, "generate", "--num-instructions", fmt.Sprintf("%d", NumInstructions), "--output-dir", outputDir)

	if WorkDir != "" {
		cmd.Dir = WorkDir
	}

	cmd.Env = os.Environ()

	sugar.Debug("Running lab generate")
	// Run the command
	if err := cmd.Run(); err != nil {
		// TODO: Log stderr
		sugar.Errorf("Could not run lab generate: %v, %s", err)
		return
	}

	items, err := os.ReadDir(outputDir)
	if err != nil {
		sugar.Errorf("Could not read output directory: %v", err)
		return
	}
	presign := s3.NewPresignClient(svc)
	presignedFiles := make([]map[string]string, 0)

	for _, item := range items {
		file, err := os.Open(path.Join(outputDir, item.Name()))
		if err != nil {
			sugar.Errorf("Could not open file: %v", err)
			return
		}
		defer file.Close()

		result, err := presign.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String("instruct-lab-bot"),
			Key:    aws.String(fmt.Sprintf("%s/%s", outDirName, item.Name())),
			Body:   file,
		})

		if err != nil {
			sugar.Errorf("Could not presign object: %v", err)
			return
		}

		presignedFiles = append(presignedFiles, map[string]string{
			"name": item.Name(),
			"url":  result.URL,
		})
	}

	indexFile, err := os.Create(path.Join(outputDir, "index.html"))
	if err != nil {
		sugar.Errorf("Could not create index.html: %v", err)
		return
	}
	defer indexFile.Close()

	if err := generateIndexHTML(indexFile, prNumber, presignedFiles); err != nil {
		sugar.Errorf("Could not generate index.html: %v", err)
		return
	}

	indexResult, err := presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("instruct-lab-bot"),
		Key:    aws.String(fmt.Sprintf("%s/index.html", outDirName)),
		Body:   indexFile,
	})

	if err != nil {
		sugar.Errorf("Could not presign index.html: %v", err)
		return
	}

	if _, err = conn.Do("SET", fmt.Sprintf("jobs:%s:s3_url", job), indexResult.URL); err != nil {
		sugar.Errorf("Could not set s3_url in redis: %v", err)
	}

	// Notify the "results" queue that the job is done
	if _, err = conn.Do("LPUSH", "results", job); err != nil {
		sugar.Errorf("Could not push to redis queue: %v", err)
	}
	sugar.Infof("Job done")
}

func generateIndexHTML(indexFile *os.File, prNumber string, presignedFiles []map[string]string) error {
	const INDEX_HTML = `
<!DOCTYPE html>
<html>
<head>
    <title>Generated Data for {{ .Name }}</title>
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
