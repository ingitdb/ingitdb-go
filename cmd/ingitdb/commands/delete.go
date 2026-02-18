package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Delete returns the delete command.
func Delete() *cli.Command {
	return &cli.Command{
		Name:     "delete",
		Usage:    "Delete database objects (collection, view, or records)",
		Commands: []*cli.Command{collection(), deleteView(), records()},
	}
}

func collection() *cli.Command {
	return &cli.Command{
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
	}
}

// deleteView is named with the parent prefix because "view" also appears as a
// subcommand of list.
func deleteView() *cli.Command {
	return &cli.Command{
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
	}
}

func records() *cli.Command {
	return &cli.Command{
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
	}
}
