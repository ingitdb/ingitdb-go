package commands

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

type githubRepoSpec struct {
	Owner string
	Repo  string
	Ref   string
}

// githubToken returns the GitHub token from the --token flag or GITHUB_TOKEN env var.
func githubToken(cmd *cli.Command) string {
	token := cmd.String("token")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	return token
}

// newGitHubConfig builds a dalgo2ghingitdb.Config from a repo spec and token.
func newGitHubConfig(spec githubRepoSpec, token string) dalgo2ghingitdb.Config {
	return dalgo2ghingitdb.Config{
		Owner: spec.Owner,
		Repo:  spec.Repo,
		Ref:   spec.Ref,
		Token: token,
	}
}

func parseGitHubRepoSpec(value string) (githubRepoSpec, error) {
	if value == "" {
		return githubRepoSpec{}, fmt.Errorf("--github cannot be empty")
	}
	repoPart, refPart, hasRef := strings.Cut(value, "@")
	segments := strings.Split(repoPart, "/")
	if len(segments) != 2 || segments[0] == "" || segments[1] == "" {
		return githubRepoSpec{}, fmt.Errorf("invalid --github value %q: expected owner/repo[@ref]", value)
	}
	spec := githubRepoSpec{Owner: segments[0], Repo: segments[1]}
	if hasRef {
		if refPart == "" {
			return githubRepoSpec{}, fmt.Errorf("invalid --github value %q: empty ref", value)
		}
		spec.Ref = refPart
	}
	return spec, nil
}

func readRemoteDefinitionForID(ctx context.Context, spec githubRepoSpec, id string) (*ingitdb.Definition, string, string, error) {
	cfg := dalgo2ghingitdb.Config{Owner: spec.Owner, Repo: spec.Repo, Ref: spec.Ref}
	fileReader, err := dalgo2ghingitdb.NewGitHubFileReader(cfg)
	if err != nil {
		return nil, "", "", err
	}
	return readRemoteDefinitionForIDWithReader(ctx, id, fileReader)
}

func readRemoteDefinitionForIDWithReader(ctx context.Context, id string, fileReader dalgo2ghingitdb.FileReader) (*ingitdb.Definition, string, string, error) {
	rootConfigPath := config.RootConfigFileName
	rootConfigContent, found, err := fileReader.ReadFile(ctx, rootConfigPath)
	if err != nil {
		return nil, "", "", err
	}
	if !found {
		return nil, "", "", fmt.Errorf("file not found: %s", rootConfigPath)
	}
	var rootConfig config.RootConfig
	err = yaml.Unmarshal(rootConfigContent, &rootConfig)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to parse %s: %w", rootConfigPath, err)
	}
	err = rootConfig.Validate()
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid %s: %w", rootConfigPath, err)
	}
	collectionID, recordKey, collectionPath, err := resolveRemoteCollectionPath(rootConfig.RootCollections, id)
	if err != nil {
		return nil, "", "", err
	}
	collectionDefPath := path.Join(collectionPath, ingitdb.CollectionDefFileName)
	collectionDefContent, found, err := fileReader.ReadFile(ctx, collectionDefPath)
	if err != nil {
		return nil, "", "", err
	}
	if !found {
		return nil, "", "", fmt.Errorf("collection definition not found: %s", collectionDefPath)
	}
	colDef := &ingitdb.CollectionDef{}
	err = yaml.Unmarshal(collectionDefContent, colDef)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to parse %s: %w", collectionDefPath, err)
	}
	colDef.ID = collectionID
	colDef.DirPath = collectionPath
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			collectionID: colDef,
		},
	}
	return def, collectionID, recordKey, nil
}

func resolveRemoteCollectionPath(rootCollections map[string]string, id string) (collectionID, recordKey, collectionPath string, err error) {
	var bestPrefixLen int
	for rootID, rootPath := range rootCollections {
		// Check both "/" and "." based prefixes to handle both formats
		// e.g., for rootID="todo", check "todo/" and "todo."
		// Also handle slash-normalized format: "todo/" maps to both "todo/" and "todo/"
		prefixes := []string{
			rootID + "/", // "todo/" for rootID="todo"
			rootID + ".", // "todo." for rootID="todo"
			strings.ReplaceAll(rootID, ".", "/") + "/", // "todo/" for rootID="todo"
		}
		for _, prefix := range prefixes {
			if !strings.HasPrefix(id, prefix) {
				continue
			}
			if len(prefix) <= bestPrefixLen {
				continue
			}
			remainder := id[len(prefix):]
			if remainder == "" {
				continue
			}
			if strings.HasSuffix(rootPath, "*") {
				localID, remRecordKey, found := strings.Cut(remainder, "/")
				if !found || localID == "" || remRecordKey == "" {
					continue
				}
				collectionPrefix := rootID
				if !strings.HasSuffix(collectionPrefix, ".") {
					collectionPrefix += "."
				}
				collectionID = collectionPrefix + localID
				recordKey = remRecordKey
				basePath := strings.TrimSuffix(rootPath, "*")
				collectionPath = path.Clean(path.Join(basePath, localID))
				bestPrefixLen = len(prefix)
				continue
			}
			collectionID = rootID
			recordKey = remainder
			collectionPath = path.Clean(rootPath)
			bestPrefixLen = len(prefix)
		}
	}
	if collectionID == "" {
		return "", "", "", fmt.Errorf("unable to resolve collection for record id %q", id)
	}
	return collectionID, recordKey, collectionPath, nil
}

// listCollectionsFromFileReader reads the root config and lists all collections from a FileReader.
func listCollectionsFromFileReader(fileReader dalgo2ghingitdb.FileReader) ([]string, error) {
	ctx := context.Background()
	rootConfigContent, found, readErr := fileReader.ReadFile(ctx, config.RootConfigFileName)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read .ingitdb.yaml: %w", readErr)
	}
	if !found {
		return nil, fmt.Errorf("file not found: %s", config.RootConfigFileName)
	}
	var rootConfig config.RootConfig
	unmarshalErr := yaml.Unmarshal(rootConfigContent, &rootConfig)
	if unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse .ingitdb.yaml: %w", unmarshalErr)
	}
	validateErr := rootConfig.Validate()
	if validateErr != nil {
		return nil, fmt.Errorf("invalid .ingitdb.yaml: %w", validateErr)
	}
	var ids []string
	for rootID, rootPath := range rootConfig.RootCollections {
		if strings.HasSuffix(rootPath, "/*") {
			dirPath := strings.TrimSuffix(rootPath, "*")
			entries, listErr := fileReader.ListDirectory(ctx, dirPath)
			if listErr != nil {
				return nil, fmt.Errorf("failed to list directory %s: %w", dirPath, listErr)
			}
			for _, entry := range entries {
				collectionID := rootID + "." + entry
				ids = append(ids, collectionID)
			}
		} else {
			ids = append(ids, rootID)
		}
	}
	return ids, nil
}
