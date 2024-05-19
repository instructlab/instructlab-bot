package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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

	normalizedExpected := normalizeHTML(expected)
	normalizedActual := normalizeHTML(string(contents))

	assert.Equal(t, normalizedExpected, normalizedActual)
}

// TestFetchModelName verify the model name is extracted from the id key.
func TestFetchModelName(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{
            "object": "list",
            "data": [
                {
                    "id": "/shared_model_storage/transformers_cache/models--mistralai--Mixtral-8x7B-Instruct-v0.1/snapshots/5c79a376139be989ef1838f360bf4f1f256d7aec",
                    "object": "model",
                    "created": 1712329535,
                    "owned_by": "vllm",
                    "root": "/shared_model_storage/transformers_cache/models--mistralai--Mixtral-8x7B-Instruct-v0.1/snapshots/5c79a376139be989ef1838f360bf4f1f256d7aec",
                    "parent": null,
                    "permission": [
                        {
                            "id": "modelperm-2d4deca190134cd9b6bab49f1c769d91",
                            "object": "model_permission",
                            "created": 1712329535,
                            "allow_create_engine": false,
                            "allow_sampling": true,
                            "allow_logprobs": true,
                            "allow_search_indices": false,
                            "allow_view": true,
                            "allow_fine_tuning": false,
                            "organization": "*",
                            "group": null,
                            "is_blocking": false
                        }
                    ]
                }
            ]
        }`)
	}))
	defer mockServer.Close()

	w := NewJobProcessor(
		context.Background(),
		nil,
		nil,
		zap.NewExample().Sugar(),
		"job-id",
		mockServer.URL,
		mockServer.URL,
		"http://sdg-example.com",
		"dummy-client-cert-path.pem",
		"dummy-client-key-path.pem",
		"dummy-ca-cert-path.pem",
		20,
	)

	modelName, err := w.fetchModelName(false, w.precheckEndpoint)
	assert.NoError(t, err, "fetchModelName should not return an error")
	expectedModelName := "Mixtral-8x7B-Instruct-v0.1"
	assert.Equal(t, expectedModelName, modelName, "The model name should be extracted correctly")

	modelName, err = w.fetchModelName(true, w.precheckEndpoint)
	assert.NoError(t, err, "fetchModelName should not return an error")
	expectedModelName = "/shared_model_storage/transformers_cache/models--mistralai--Mixtral-8x7B-Instruct-v0.1/snapshots/5c79a376139be989ef1838f360bf4f1f256d7aec"
	assert.Equal(t, expectedModelName, modelName, "The model name should be extracted correctly")
}

// TestFetchModelNameWithInvalidObject negative test if the returned object is not a model
func TestFetchModelNameWithInvalidObject(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond with a payload where the "object" field within the data array is not "model"
		fmt.Fprintln(w, `{
			"object": "list",
			"data": [
				{
					"id": "/shared_model_storage/transformers_cache/models--mistralai--Mixtral-8x7B-Instruct-v0.1/snapshots/5c79a376139be989ef1838f360bf4f1f256d7aec",
					"object": "foo",  // bogus value here
					"created": 1712329535,
					"owned_by": "vllm",
					"root": "/shared_model_storage/transformers_cache/models--mistralai--Mixtral-8x7B-Instruct-v0.1/snapshots/5c79a376139be989ef1838f360bf4f1f256d7aec",
					"parent": null,
					"permission": [
						{
							"id": "modelperm-2d4deca190134cd9b6bab49f1c769d91",
							"object": "model_permission",
							"created": 1712329535,
							"allow_create_engine": false,
							"allow_sampling": true,
							"allow_logprobs": true,
							"allow_search_indices": false,
							"allow_view": true,
							"allow_fine_tuning": false,
							"organization": "*",
							"group": null,
							"is_blocking": false
						}
					]
				}
			]
		}`)
	}))
	defer mockServer.Close()

	w := NewJobProcessor(
		context.Background(),
		nil,
		nil,
		zap.NewExample().Sugar(),
		"job-id",
		mockServer.URL,
		mockServer.URL,
		"http://sdg-example.com",
		"dummy-client-cert-path.pem",
		"dummy-client-key-path.pem",
		"dummy-ca-cert-path.pem",
		20,
	)
	modelName, err := w.fetchModelName(false, w.precheckEndpoint)

	// Verify that an error was returned due to the invalid "object" field
	assert.Error(t, err, "fetchModelName should return an error for invalid object field")
	assert.Empty(t, modelName, "The model name should be empty for invalid object field")
}

// Replace all whitespace sequences with a single space. Remove spaces between HTML tags
func normalizeHTML(input string) string {
	compacted := regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")
	return regexp.MustCompile(`>\s+<`).ReplaceAllString(compacted, "><")
}
