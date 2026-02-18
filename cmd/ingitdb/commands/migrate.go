package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Migrate returns the migrate command.
func Migrate() *cli.Command {
	return &cli.Command{
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
	}
}
