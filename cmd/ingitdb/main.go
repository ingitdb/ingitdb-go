package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-go/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-go/pkg/ingitdb/validator"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	exit    = os.Exit
)

func main() {
	fatal := func(err error) {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		exit(1)
	}
	logf := func(args ...any) { fmt.Fprintln(os.Stderr, args...) }
	run(os.Args, os.UserHomeDir, os.Getwd, validator.ReadDefinition, fatal, logf)
}

func run(
	args []string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	fatal func(error),
	logf func(...any),
) {
	app := &cli.Command{
		Name:      "ingitdb",
		Usage:     "Git-backed database CLI",
		ErrWriter: os.Stderr,
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Print build version, commit hash, and build date",
				Action: func(_ context.Context, _ *cli.Command) error {
					fmt.Printf("ingitdb %s (%s) @ %s\n", version, commit, date)
					return nil
				},
			},
			{
				Name:  "validate",
				Usage: "Validate an inGitDB database directory",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "path",
						Usage: "path to the database directory (default: current directory)",
					},
					&cli.StringFlag{
						Name:  "from-commit",
						Usage: "validate only records changed since this commit",
					},
					&cli.StringFlag{
						Name:  "to-commit",
						Usage: "validate only records up to this commit",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					dirPath := cmd.String("path")
					if dirPath == "" {
						wd, err := getWd()
						if err != nil {
							return fmt.Errorf("failed to get working directory: %w", err)
						}
						dirPath = wd
					}
					expanded, err := expandHome(dirPath, homeDir)
					if err != nil {
						return err
					}
					dirPath = expanded
					logf("inGitDB db path: ", dirPath)
					_, err = readDefinition(dirPath, ingitdb.Validate())
					if err != nil {
						return fmt.Errorf("inGitDB database validation failed: %w", err)
					}
					return nil
				},
			},
			{
				Name:  "query",
				Usage: "Query records from a collection",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "collection",
						Usage:    "collection to query",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "path",
						Usage: "path to the database directory",
					},
					&cli.StringFlag{
						Name:  "format",
						Usage: "output format (json, yaml)",
					},
				},
				Action: func(_ context.Context, _ *cli.Command) error {
					return cli.Exit("not yet implemented", 1)
				},
			},
			{
				Name:  "materialize",
				Usage: "Materialize views in the database",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "path",
						Usage: "path to the database directory",
					},
					&cli.StringFlag{
						Name:  "views",
						Usage: "comma-separated list of views to materialize",
					},
				},
				Action: func(_ context.Context, _ *cli.Command) error {
					return cli.Exit("not yet implemented", 1)
				},
			},
			{
				Name:  "pull",
				Usage: "Pull latest changes, resolve conflicts, and rebuild views",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "path",
						Usage: "path to the database directory",
					},
					&cli.StringFlag{
						Name:  "strategy",
						Usage: "git pull strategy: rebase (default) or merge",
					},
					&cli.StringFlag{
						Name:  "remote",
						Usage: "remote name (default: origin)",
					},
					&cli.StringFlag{
						Name:  "branch",
						Usage: "branch to pull (default: tracking branch)",
					},
				},
				Action: func(_ context.Context, _ *cli.Command) error {
					return cli.Exit("not yet implemented", 1)
				},
			},
			{
				Name:  "setup",
				Usage: "Set up a new inGitDB database",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "path",
						Usage: "path to the database directory",
					},
				},
				Action: func(_ context.Context, _ *cli.Command) error {
					return cli.Exit("not yet implemented", 1)
				},
			},
			{
				Name:  "resolve",
				Usage: "Resolve merge conflicts in database files",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "path",
						Usage: "path to the database directory",
					},
					&cli.StringFlag{
						Name:  "file",
						Usage: "specific file to resolve",
					},
				},
				Action: func(_ context.Context, _ *cli.Command) error {
					return cli.Exit("not yet implemented", 1)
				},
			},
			{
				Name:  "watch",
				Usage: "Watch database for changes and log events to stdout",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "path",
						Usage: "path to the database directory",
					},
					&cli.StringFlag{
						Name:  "format",
						Usage: "output format: text (default) or json",
					},
				},
				Action: func(_ context.Context, _ *cli.Command) error {
					return cli.Exit("not yet implemented", 1)
				},
			},
			{
				Name:  "serve",
				Usage: "Start one or more servers (MCP, HTTP API, watcher)",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "path",
						Usage: "path to the database directory",
					},
					&cli.BoolFlag{
						Name:  "mcp",
						Usage: "enable MCP server",
					},
					&cli.BoolFlag{
						Name:  "http",
						Usage: "enable HTTP API server",
					},
					&cli.BoolFlag{
						Name:  "watcher",
						Usage: "enable file watcher",
					},
				},
				Action: func(_ context.Context, _ *cli.Command) error {
					return cli.Exit("not yet implemented", 1)
				},
			},
			{
				Name:  "list",
				Usage: "List database objects (collections, views, or subscribers)",
				Commands: []*cli.Command{
					{
						Name:  "collections",
						Usage: "List collections in the database",
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
								Usage: "pattern to filter collection names (e.g. *substr*)",
							},
						},
						Action: func(_ context.Context, _ *cli.Command) error {
							return cli.Exit("not yet implemented", 1)
						},
					},
					{
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
					},
					{
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
					},
				},
			},
			{
				Name:  "find",
				Usage: "Search for records matching a pattern",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "path",
						Usage: "path to the database directory",
					},
					&cli.StringFlag{
						Name:  "substr",
						Usage: "match records containing this substring",
					},
					&cli.StringFlag{
						Name:  "re",
						Usage: "match records where a field value matches this regular expression",
					},
					&cli.StringFlag{
						Name:  "exact",
						Usage: "match records where a field value matches exactly",
					},
					&cli.StringFlag{
						Name:  "in",
						Usage: "regular expression scoping the search to a sub-path",
					},
					&cli.IntFlag{
						Name:  "limit",
						Usage: "maximum number of records to return",
					},
					&cli.StringFlag{
						Name:  "fields",
						Usage: "comma-separated list of fields to search (default: all fields)",
					},
				},
				Action: func(_ context.Context, _ *cli.Command) error {
					return cli.Exit("not yet implemented", 1)
				},
			},
			{
				Name:  "delete",
				Usage: "Delete database objects (collection, view, or records)",
				Commands: []*cli.Command{
					{
						Name:  "collection",
						Usage: "Delete a collection and all its records",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "path",
								Usage: "path to the database directory",
							},
							&cli.StringFlag{
								Name:     "collection",
								Usage:    "collection id to delete (e.g. countries/ie/counties)",
								Required: true,
							},
						},
						Action: func(_ context.Context, _ *cli.Command) error {
							return cli.Exit("not yet implemented", 1)
						},
					},
					{
						Name:  "view",
						Usage: "Delete a view definition and its materialised files",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "path",
								Usage: "path to the database directory",
							},
							&cli.StringFlag{
								Name:     "view",
								Usage:    "view id to delete",
								Required: true,
							},
						},
						Action: func(_ context.Context, _ *cli.Command) error {
							return cli.Exit("not yet implemented", 1)
						},
					},
					{
						Name:  "records",
						Usage: "Delete individual records from a collection",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "path",
								Usage: "path to the database directory",
							},
							&cli.StringFlag{
								Name:     "collection",
								Usage:    "collection to delete records from",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "in",
								Usage: "regular expression scoping deletion to a sub-path",
							},
							&cli.StringFlag{
								Name:  "filter-name",
								Usage: "pattern to match record names to delete",
							},
						},
						Action: func(_ context.Context, _ *cli.Command) error {
							return cli.Exit("not yet implemented", 1)
						},
					},
				},
			},
			{
				Name:  "truncate",
				Usage: "Remove all records from a collection",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "path",
						Usage: "path to the database directory",
					},
					&cli.StringFlag{
						Name:     "collection",
						Usage:    "collection id to truncate (e.g. countries/ie/counties/dublin)",
						Required: true,
					},
				},
				Action: func(_ context.Context, _ *cli.Command) error {
					return cli.Exit("not yet implemented", 1)
				},
			},
			{
				Name:  "migrate",
				Usage: "Migrate data between schema versions",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "from",
						Usage:    "source schema version",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "to",
						Usage:    "target schema version",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "target",
						Usage:    "migration target",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "path",
						Usage: "path to the database directory",
					},
					&cli.StringFlag{
						Name:  "format",
						Usage: "output format",
					},
					&cli.StringFlag{
						Name:  "collections",
						Usage: "comma-separated list of collections to migrate",
					},
					&cli.StringFlag{
						Name:  "output-dir",
						Usage: "directory for migration output",
					},
				},
				Action: func(_ context.Context, _ *cli.Command) error {
					return cli.Exit("not yet implemented", 1)
				},
			},
		},
	}

	err := app.Run(context.Background(), args)
	if err == nil {
		return
	}
	var exitErr cli.ExitCoder
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		if code != 0 {
			exit(code)
		}
		return
	}
	fatal(err)
}

func expandHome(path string, homeDir func() (string, error)) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := homeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}
		return filepath.Join(home, path[1:]), nil
	}
	return path, nil
}
