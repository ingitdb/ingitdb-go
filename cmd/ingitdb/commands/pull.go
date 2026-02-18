package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Pull returns the pull command.
func Pull() *cli.Command {
	return &cli.Command{
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
	}
}
