package commands

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

// Version returns the version command.
func Version(ver, commit, date string) *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Print build version, commit hash, and build date",
		Action: func(_ context.Context, _ *cli.Command) error {
			fmt.Printf("ingitdb %s (%s) @ %s\n", ver, commit, date)
			return nil
		},
	}
}
