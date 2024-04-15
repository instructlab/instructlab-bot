package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
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
           mode: 'preview',
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
