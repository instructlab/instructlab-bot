package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

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

	tmpl, err := template.New("index").Parse(INDEX_HTML)
	if err != nil {
		return fmt.Errorf("template parsing error: %w", err)
	}

	data := struct {
		Name  string
		Files []map[string]string
	}{
		Name:  fmt.Sprintf("PR %s", prNumber),
		Files: presignedFiles,
	}

	return tmpl.Execute(indexFile, data)
}

// Generate a JSON viewer only for files with valid JSON output
func generateFormattedJSON(ctx context.Context, outputDir, filename string, svc *s3.Client, logger *zap.SugaredLogger) string {
	inputFile := path.Join(outputDir, filename)
	formattedHTMLFile := inputFile + jsonViewerFilenameSuffix

	s3Key := fmt.Sprintf("%s/%s", path.Base(outputDir), path.Base(formattedHTMLFile))

	// Check if formatted HTML file already exists from previous runs
	if _, err := os.Stat(formattedHTMLFile); err == nil {
		return s3Key
	}

	jsonData, err := os.ReadFile(inputFile)
	if err != nil {
		logger.Errorf("Failed to read JSON file: %v", err)
		return ""
	}

	var temp interface{}
	// If the JSON doesn't marshall, skip.
	// TODO: support invalid top-level format such as an array in generate-local such as test_merlinite & train_merlinite
	if err := json.Unmarshal(jsonData, &temp); err != nil {
		return ""
	}

	encodedJSON, err := json.Marshal(string(jsonData))
	if err != nil {
		logger.Errorf("Failed to encode JSON data: %v", err)
		return ""
	}

	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
   <title>Formatted JSON Viewer</title>
   <link href="https://cdnjs.cloudflare.com/ajax/libs/jsoneditor/10.0.2/jsoneditor.min.css" rel="stylesheet" type="text/css">
   <script src="https://cdnjs.cloudflare.com/ajax/libs/jsoneditor/10.0.2/jsoneditor.min.js"></script>
</head>
<body>
<div id="json-editor" style="height: 95vh;"></div>
<script>
   document.addEventListener("DOMContentLoaded", function() {
       var container = document.getElementById('json-editor');
       var options = {
           mode: 'view',
           modes: ['code', 'form', 'text', 'tree', 'view', 'preview']
       };
       var editor = new JSONEditor(container, options);
       var json = %s;
       editor.set(JSON.parse(json));
       editor.expandAll();
   });
</script>
</body>
</html>
`, encodedJSON)

	err = os.WriteFile(formattedHTMLFile, []byte(htmlContent), 0644)
	if err != nil {
		logger.Errorf("Failed to write HTML file: %v", err)
		return ""
	}

	file, err := os.Open(formattedHTMLFile)
	if err != nil {
		logger.Errorf("Could not open generated HTML file: %v", err)
		return ""
	}
	defer file.Close()

	_, err = svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(S3Bucket),
		Key:         aws.String(s3Key),
		Body:        file,
		ContentType: aws.String("text/html"),
	})
	if err != nil {
		logger.Errorf("Could not upload formatted HTML file to S3: %v", err)
		return ""
	}

	return s3Key
}

// Generate formatted YAML HTML from JSON files
func generateFormattedYAML(ctx context.Context, outputDir, filename string, svc *s3.Client, logger *zap.SugaredLogger) string {
	inputFile := path.Join(outputDir, filename)
	outputFile := inputFile + ".yaml.html"
	s3Key := fmt.Sprintf("%s/%s", path.Base(outputDir), path.Base(outputFile))

	jsonData, err := os.ReadFile(inputFile)
	if err != nil {
		logger.Errorf("Failed to read JSON file: %v", err)
		return ""
	}

	var temp interface{}
	// If the JSON doesn't marshall, skip.
	// TODO: support invalid top-level format such as an array in generate-local such as test_merlinite & train_merlinite
	if err := json.Unmarshal(jsonData, &temp); err != nil {
		return ""
	}

	yamlData, err := yaml.JSONToYAML(jsonData)
	if err != nil {
		logger.Errorf("Failed to convert JSON to YAML: %v", err)
		return ""
	}

	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>YAML Viewer</title>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github.min.css">
    <script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/highlight.min.js"></script>
    <script>hljs.highlightAll();</script>
    <style>
        body { font-family: Arial, sans-serif; padding: 20px; }
    </style>
</head>
<body>
<pre><code class="language-yaml">%s</code></pre>
<script>
    document.addEventListener('DOMContentLoaded', (event) => {
        document.querySelectorAll('pre code').forEach((block) => {
            hljs.highlightBlock(block);
        });
    });
</script>
</body>
</html>
`, string(yamlData))

	err = os.WriteFile(outputFile, []byte(htmlContent), 0644)
	if err != nil {
		logger.Errorf("Failed to write HTML file: %v", err)
		return ""
	}

	file, err := os.Open(outputFile)
	if err != nil {
		logger.Errorf("Could not open generated HTML file: %v", err)
		return ""
	}
	defer file.Close()

	_, err = svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(S3Bucket),
		Key:         aws.String(s3Key),
		Body:        file,
		ContentType: aws.String("text/html"),
	})
	if err != nil {
		logger.Errorf("Could not upload formatted HTML file to S3: %v", err)
		return ""
	}

	return s3Key
}

func generatePrecheckScoringPrompt(precheckPRAnswer string, precheckEndpointAnswer string, precheckQuestion string) (error, string) {
	promptTemplate := `
 	Evaluate and compare the quality of the below ### Model answer compared to the ### Human answer when given the same ### Question provided below.
  	The ### Human answer is to be treated as the ground truth answer.
  	Assign a score using the following 3 point scale:
  	1: It means that the answers are identical or nearly identical, based on both the content of the two provided answers as
   	well as the wording and details of the answer provided.

     	2: It means that there is moderate variation in the answers. The two provided answers could have a moderately different sentence structure
	and wording, or have some differences in the content or perspective, but still share some key points.

       	3: It means the answers are significantly different. The two provided answers differ greatly in wording and perspective or have very different
	or contridictory facts and content.
 
 	### Question:
  	"{{ .Question }}"
 	### Human answer:
	"{{ .HumanAnswer }}"
	### Model answer:
	"{{ .ModelAnswer }}"
 
	`

	tmpl, err := template.New("modelScoring").Parse(promptTemplate)
	if err != nil {
		return fmt.Errorf("error parsing modelScoring prompt template: %w", err), ""
	}

	data := struct {
		HumanAnswer string
		ModelAnswer string
		Question string
	}{
		HumanAnswer: precheckPRAnswer,
		ModelAnswer: precheckEndpointAnswer,
		Question: precheckQuestion,
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return fmt.Errorf("error executing modelScoring prompt template: %w", err), ""
	}
	return nil, buf.String()
}
