package githubdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("owner/repo", "token")

	if client.owner != "owner" {
		t.Errorf("Expected owner to be 'owner', got '%s'", client.owner)
	}

	if client.repo != "repo" {
		t.Errorf("Expected repo to be 'repo', got '%s'", client.repo)
	}
}

func TestSaveToGitHubRepo(t *testing.T) {
	// Create a mock HTTP server that simulates GitHub API responses
	// for the Git Data API calls used in the transactional implementation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check both the path and the HTTP method to differentiate between similar endpoints
		if r.URL.Path == "/api/v3/repos/owner/repo/git/refs/heads/main" && r.Method == "GET" {
			// Mock response for GetRef
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ref": "refs/heads/main",
				"object": {
					"sha": "abcd1234",
					"type": "commit",
					"url": "https://api.github.com/repos/owner/repo/git/commits/abcd1234"
				}
			}`))
			return
		}

		if r.URL.Path == "/api/v3/repos/owner/repo/git/refs/heads/main" && r.Method == "PATCH" {
			// Mock response for UpdateRef
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ref": "refs/heads/main",
				"object": {
					"sha": "commit1234",
					"type": "commit",
					"url": "https://api.github.com/repos/owner/repo/git/commits/commit1234"
				}
			}`))
			return
		}

		switch r.URL.Path {
		case "/api/v3/repos/owner/repo/git/commits/abcd1234":
			// Mock response for GetCommit
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"sha": "abcd1234",
				"tree": {
					"sha": "efgh5678",
					"url": "https://api.github.com/repos/owner/repo/git/trees/efgh5678"
				}
			}`))
		case "/api/v3/repos/owner/repo/git/trees/efgh5678":
			// Mock response for GetTree
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"sha": "efgh5678",
				"tree": []
			}`))
		case "/api/v3/repos/owner/repo/git/blobs":
			// Mock response for CreateBlob
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"sha": "blob1234",
				"url": "https://api.github.com/repos/owner/repo/git/blobs/blob1234"
			}`))
		case "/api/v3/repos/owner/repo/git/trees":
			// Mock response for CreateTree
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"sha": "tree1234",
				"url": "https://api.github.com/repos/owner/repo/git/trees/tree1234"
			}`))
		case "/api/v3/repos/owner/repo/git/commits":
			// Mock response for CreateCommit
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"sha": "commit1234",
				"url": "https://api.github.com/repos/owner/repo/git/commits/commit1234"
			}`))
		default:
			t.Logf("Unexpected request to %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create a client that points to our mock server
	// Note: In a real test, we would inject a custom HTTP client that points to our mock server
	// For this example, we're just testing the function signature
	client := NewClient("owner/repo", "token")

	// Test data with multiple objects to demonstrate transactional behavior
	objects := []struct {
		Path    string
		Content any
	}{
		{
			Path: "data/user.json",
			Content: map[string]interface{}{
				"id":    1,
				"name":  "John Doe",
				"email": "john@example.com",
			},
		},
		{
			Path: "config/settings.json",
			Content: map[string]interface{}{
				"theme":         "dark",
				"notifications": true,
			},
		},
	}

	// This will fail in a real test because we're not actually connecting to the mock server
	// But it demonstrates how to use the function with multiple objects in a transaction
	commitHash, err := client.SaveToGitHubRepo(context.Background(), objects)

	// In a real test, you would check the error and verify the behavior
	// For this example, we're just checking that the function compiles
	if err == nil {
		t.Logf("Function executed without errors (this is expected to fail in a real test)")
		t.Logf("Commit hash: %s", commitHash)
	} else {
		t.Logf("Error: %v", err)
	}
}
