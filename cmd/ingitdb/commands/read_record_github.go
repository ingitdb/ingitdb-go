package commands

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
	"gopkg.in/yaml.v3"
)

type githubRepoSpec struct {
	Owner string
	Repo  string
	Ref   string
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
