package commands

import (
	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Update returns the update command group.
func Update(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:     "update",
		Aliases:  []string{"u"},
		Usage:    "Update database objects",
		Commands: []*cli.Command{updateRecord(homeDir, getWd, readDefinition, newDB, logf)},
	}
}
