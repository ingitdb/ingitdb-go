package commands

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

// List returns the list command.
func List(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cli.Command {
	return &cli.Command{
		Name:     "list",
		Usage:    "List database objects (collections, views, or subscribers)",
		Commands: []*cli.Command{collections(homeDir, getWd, readDefinition), listView(), subscribers()},
	}
}

func collections(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cli.Command {
	return &cli.Command{
		Name:  "collections",
		Usage: "List collections in the database",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:  "github",
				Usage: "GitHub source as owner/repo[@branch|tag|commit]",
			},
			&cli.StringFlag{
				Name:  "token",
				Usage: "GitHub personal access token (or set GITHUB_TOKEN env var)",
			},
			&cli.StringFlag{
				Name:  "in",
				Usage: "regular expression for the starting-point path",
			},
			&cli.StringFlag{
				Name:  "filter-name",
				Usage: "pattern to filter collection names (e.g. *substr*)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			githubValue := cmd.String("github")
			if githubValue != "" {
				return listCollectionsGitHub(ctx, githubValue, githubToken(cmd))
			}
			return listCollectionsLocal(cmd, homeDir, getWd, readDefinition)
		},
	}
}

func listCollectionsLocal(
	cmd *cli.Command,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) error {
	dirPath, resolveErr := resolveDBPath(cmd, homeDir, getWd)
	if resolveErr != nil {
		return resolveErr
	}
	def, readErr := readDefinition(dirPath)
	if readErr != nil {
		return fmt.Errorf("failed to read database definition: %w", readErr)
	}
	ids := make([]string, 0, len(def.Collections))
	for id := range def.Collections {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		_, _ = fmt.Fprintln(os.Stdout, id)
	}
	return nil
}

func listCollectionsGitHub(ctx context.Context, githubValue, token string) error {
	spec, parseErr := parseGitHubRepoSpec(githubValue)
	if parseErr != nil {
		return parseErr
	}
	cfg := newGitHubConfig(spec, token)
	fileReader, newReaderErr := dalgo2ghingitdb.NewGitHubFileReader(cfg)
	if newReaderErr != nil {
		return fmt.Errorf("failed to create github file reader: %w", newReaderErr)
	}
	rootConfigContent, found, readFileErr := fileReader.ReadFile(ctx, config.RootConfigFileName)
	if readFileErr != nil {
		return fmt.Errorf("failed to read .ingitdb.yaml: %w", readFileErr)
	}
	if !found {
		return fmt.Errorf("file not found: %s", config.RootConfigFileName)
	}
	var rootConfig config.RootConfig
	unmarshalErr := yaml.Unmarshal(rootConfigContent, &rootConfig)
	if unmarshalErr != nil {
		return fmt.Errorf("failed to parse .ingitdb.yaml: %w", unmarshalErr)
	}
	validateErr := rootConfig.Validate()
	if validateErr != nil {
		return fmt.Errorf("invalid .ingitdb.yaml: %w", validateErr)
	}
	ids := make([]string, 0)
	for rootID, rootPath := range rootConfig.RootCollections {
		if strings.HasSuffix(rootPath, "/*") {
			dirPath := strings.TrimSuffix(rootPath, "*")
			entries, listErr := fileReader.ListDirectory(ctx, dirPath)
			if listErr != nil {
				return fmt.Errorf("failed to list directory %s: %w", dirPath, listErr)
			}
			for _, entry := range entries {
				collectionID := rootID + "." + entry
				ids = append(ids, collectionID)
			}
		} else {
			ids = append(ids, rootID)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		_, _ = fmt.Fprintln(os.Stdout, id)
	}
	return nil
}

// listView is named with the parent prefix because "view" also appears as a
// subcommand of delete.
func listView() *cli.Command {
	return &cli.Command{
		Name:  "view",
		Usage: "List views in the database",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:  "in",
				Usage: "regular expression for the starting-point path",
			},
			&cli.StringFlag{
				Name:  "filter-name",
				Usage: "pattern to filter view names (e.g. *substr*)",
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}

func subscribers() *cli.Command {
	return &cli.Command{
		Name:  "subscribers",
		Usage: "List subscribers in the database",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:  "in",
				Usage: "regular expression for the starting-point path",
			},
			&cli.StringFlag{
				Name:  "filter-name",
				Usage: "pattern to filter subscriber names (e.g. *substr*)",
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
