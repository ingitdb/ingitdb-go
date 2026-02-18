package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Find returns the find command.
func Find() *cli.Command {
	return &cli.Command{
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
	}
}
