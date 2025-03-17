package githubdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// Client is a client for interacting with GitHub repositories
type Client struct {
	client *github.Client
	repo   string
	owner  string
}

// NewClient creates a new GitHub client with the given repository and token
func NewClient(repo, token string) *Client {
	// Split the repo string into owner and repository name
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		panic("Invalid repository format. Expected 'owner/repo'")
	}
	owner := parts[0]
	repoName := parts[1]

	// Create an OAuth2 client with the token
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	// Create a GitHub client
	client := github.NewClient(tc)

	return &Client{
		client: client,
		repo:   repoName,
		owner:  owner,
	}
}

// SaveToGitHubRepo saves the given objects to the GitHub repository in a transactional manner.
// Either all objects are saved successfully, or none of them are saved.
// Returns the commit hash and any error that occurred.
func (c *Client) SaveToGitHubRepo(ctx context.Context, objects []struct {
	Path    string
	Content any
}) (commitHash string, err error) {
	// First, validate all objects can be marshaled to JSON
	fileContents := make(map[string][]byte, len(objects))
	for _, obj := range objects {
		content, err := json.Marshal(obj.Content)
		if err != nil {
			return "", fmt.Errorf("failed to marshal content for path %s: %w", obj.Path, err)
		}
		fileContents[obj.Path] = content
	}

	// Get the reference to the default branch
	ref, _, err := c.client.Git.GetRef(ctx, c.owner, c.repo, "heads/main")
	if err != nil {
		// Try master if main doesn't exist
		ref, _, err = c.client.Git.GetRef(ctx, c.owner, c.repo, "heads/master")
		if err != nil {
			return "", fmt.Errorf("failed to get reference to default branch: %w", err)
		}
	}

	// Get the commit that the branch points to
	commit, _, err := c.client.Git.GetCommit(ctx, c.owner, c.repo, *ref.Object.SHA)
	if err != nil {
		return "", fmt.Errorf("failed to get latest commit: %w", err)
	}

	// Get the tree that the commit points to
	baseTree, _, err := c.client.Git.GetTree(ctx, c.owner, c.repo, *commit.Tree.SHA, true)
	if err != nil {
		return "", fmt.Errorf("failed to get base tree: %w", err)
	}

	// Create tree entries for all files
	entries := make([]*github.TreeEntry, 0, len(objects))
	for path, content := range fileContents {
		// Create a blob for the file content
		blob, _, err := c.client.Git.CreateBlob(ctx, c.owner, c.repo, &github.Blob{
			Content:  github.String(string(content)),
			Encoding: github.String("utf-8"),
		})
		if err != nil {
			return "", fmt.Errorf("failed to create blob for %s: %w", path, err)
		}

		// Create a tree entry for the file
		entries = append(entries, &github.TreeEntry{
			Path: github.String(path),
			Mode: github.String("100644"), // Regular file
			Type: github.String("blob"),
			SHA:  blob.SHA,
		})
	}

	// Create a new tree with the new entries
	newTree, _, err := c.client.Git.CreateTree(ctx, c.owner, c.repo, *baseTree.SHA, entries)
	if err != nil {
		return "", fmt.Errorf("failed to create new tree: %w", err)
	}

	// Create a new commit with the new tree
	message := fmt.Sprintf("Update %d files", len(objects))
	newCommit, _, err := c.client.Git.CreateCommit(ctx, c.owner, c.repo, &github.Commit{
		Message: github.String(message),
		Tree:    newTree,
		Parents: []*github.Commit{{SHA: commit.SHA}},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new commit: %w", err)
	}

	// Update the reference to point to the new commit
	ref.Object.SHA = newCommit.SHA
	_, _, err = c.client.Git.UpdateRef(ctx, c.owner, c.repo, ref, false)
	if err != nil {
		return "", fmt.Errorf("failed to update reference: %w", err)
	}

	return *newCommit.SHA, nil
}
