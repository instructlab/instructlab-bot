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
