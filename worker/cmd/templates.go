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
	"sigs.k8s.io/yaml"
)

func generateAllHTML(allFile *os.File, logEntries []string, fileNames []string) error {
	const INDEX_HTML = `
<!DOCTYPE html>
<html>
<head>
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
       	a {
           	color: var(--link-color);
           	text-decoration: none;
           	font-weight: 500;
       	}	
       	a:hover {
           	color: var(--link-hover-color);
           	text-decoration: underline;
       	}
		.item {
			min-height: 300px
		}
		.item p {
			max-width: fit-content;
			margin-left: auto;
			margin-right: auto;
		}
		.item h5 {
			max-width: fit-content;
			margin-left: auto;
			margin-right: auto;
		}
   	</style>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.4.1/css/bootstrap.min.css">
	<script src="https://ajax.googleapis.com/ajax/libs/jquery/3.7.1/jquery.min.js"></script>
	<script src="https://maxcdn.bootstrapcdn.com/bootstrap/3.4.1/js/bootstrap.min.js"></script>
</head>
<body>
	<div class="container">
		<h2>All Log Files as Carousel</h2>
		<div id="logfileCarousel" class="carousel slide" data-ride="carousel" data-interval="false">
			<ol class="carousel-indicators">
			{{ range $index, $value := .Files }}
				{{ if eq $index 0 }}
				<li data-target="#logfileCarousel" data-slide-to="{{$index}}" class="active"></li>
				{{ else }}
				<li data-target="#logfileCarousel" data-slide-to="{{$index}}"></li>
				{{ end}}
			{{ end }}
			</ol>
			<div class="carousel-inner">
			{{ $FileNames := .FileNames }}
			{{ range $index, $value := .Files }}
				{{ if eq $index 0 }}
				<div class="item active">
					<h5> {{ index $FileNames $index }} </h5>
					<pre> {{ $value }} </pre>
				</div>
				{{ else }}
				<div class="item">
					<h5> {{ index $FileNames $index }} </h5>
					<pre> {{ $value }} </pre>
				</div>
				{{ end}}
			{{ end }}
			</div>
			<a class="left carousel-control" href="#logfileCarousel" data-slide="prev">
				<span class="glyphicon glyphicon-chevron-left"></span>
				<span class="sr-only">Previous</span>
			</a>
			<a class="right carousel-control" href="#logfileCarousel" data-slide="next">
				<span class="glyphicon glyphicon-chevron-right"></span>
				<span class="sr-only">Next</span>
			</a>
		</div>
	</div>
</body>
</html>`

	tmpl, err := template.New("combined_chatlogs.html").Parse(INDEX_HTML)
	if err != nil {
		return fmt.Errorf("template parsing error: %w", err)
	}

	data := struct {
		Files     []string
		FileNames []string
	}{
		Files:     logEntries,
		FileNames: fileNames,
	}

	return tmpl.Execute(allFile, data)
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
