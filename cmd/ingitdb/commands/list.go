package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// List returns the list command.
func List() *cli.Command {
	return &cli.Command{
		Name:     "list",
		Usage:    "List database objects (collections, views, or subscribers)",
		Commands: []*cli.Command{collections(), listView(), subscribers()},
	}
}

func collections() *cli.Command {
	return &cli.Command{
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
	}
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
