package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-go/githubdb"
)

func main() {
	// Parse command line arguments
	args := os.Args[1:]

	if len(args) < 1 {
		fmt.Println("Usage: githubdb-go <localPath> [repo] [token] [remotePath]")
		fmt.Println("  localPath: Path to local .json file (required)")
		fmt.Println("  repo: GitHub repository in format 'owner/repo' (optional, defaults to INGITDB_GH_REPO env var)")
		fmt.Println("  token: GitHub token (optional, defaults to INGITDB_GH_TOKEN env var)")
		fmt.Println("  remotePath: Path in the repository to upload the file (optional, defaults to root)")
		os.Exit(1)
	}

	// Get the local file path
	localPath := args[0]

	// Get the repository
	var repo string
	if len(args) > 1 && args[1] != "" {
		repo = args[1]
	} else {
		repo = os.Getenv("INGITDB_GH_REPO")
		if repo == "" {
			log.Fatal("Repository not provided and INGITDB_GH_REPO environment variable is not set")
		}
	}

	// Get the token
	var token string
	if len(args) > 2 && args[2] != "" {
		token = args[2]
	} else {
		token = os.Getenv("INGITDB_GH_TOKEN")
		if token == "" {
			log.Fatal("Token not provided and INGITDB_GH_TOKEN environment variable is not set")
		}
	}

	// Get the remote path
	var remotePath string
	if len(args) > 3 && args[3] != "" {
		remotePath = args[3]
	} else {
		// Use the filename from the local path as the remote path
		remotePath = filepath.Base(localPath)
	}

	// Read the local file
	fileContent, err := os.ReadFile(localPath)
	if err != nil {
		log.Fatalf("Failed to read file %s: %v", localPath, err)
	}

	// Parse the JSON content
	var jsonContent interface{}
	err = json.Unmarshal(fileContent, &jsonContent)
	if err != nil {
		log.Fatalf("Failed to parse JSON from file %s: %v", localPath, err)
	}

	// Create the GitHub client
	client := githubdb.NewClient(repo, token)

	// Prepare the object to upload
	objects := []struct {
		Path    string
		Content any
	}{
		{
			Path:    remotePath,
			Content: jsonContent,
		},
	}

	// Upload to GitHub
	fmt.Printf("Uploading %s to %s in repository %s...\n", localPath, remotePath, repo)

	// Get file size for reporting
	fileInfo, err := os.Stat(localPath)
	if err == nil {
		fmt.Printf("File size: %d bytes\n", fileInfo.Size())
	}

	// Upload the file
	commitHash, err := client.SaveToGitHubRepo(context.Background(), objects)
	if err != nil {
		log.Fatalf("Failed to save to GitHub: %v", err)
	}

	// Get the repository parts for the success message
	repoParts := strings.Split(repo, "/")
	owner := repoParts[0]
	repoName := repoParts[1]

	fmt.Println("Successfully uploaded to GitHub repository")
	fmt.Printf("Repository: https://github.com/%s/%s\n", owner, repoName)
	fmt.Printf("File path: %s\n", remotePath)
	fmt.Printf("Commit hash: %s\n", commitHash)
	fmt.Printf("Commit URL: https://github.com/%s/%s/commit/%s\n", owner, repoName, commitHash)
}
