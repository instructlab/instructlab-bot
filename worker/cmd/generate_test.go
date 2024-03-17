package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateIndexHTML(t *testing.T) {

	f, err := os.CreateTemp("", "index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	presignedFiles := []map[string]string{
		{"name": "file1", "url": "http://example.com/file1"},
		{"name": "file2", "url": "http://example.com/file2"},
	}

	if err := generateIndexHTML(f, "123", presignedFiles); err != nil {
		t.Fatal(err)
	}

	// Check the file contents is correct
	contents, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	expected := `
<!DOCTYPE html>
<html>
<head>
    <title>Generated Data for PR 123</title>
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
    <h1>Generated Data for PR 123</h1>
	<ul>
		<li><a href="http://example.com/file1">file1</a></li>
		<li><a href="http://example.com/file2">file2</a></li>
	</ul>
</body>
</html>`

	assert.Equal(t, expected, string(contents))
}
