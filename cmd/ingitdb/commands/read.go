package commands

import (
	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Read returns the read command group.
func Read(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:    "read",
		Aliases: []string{"r"},
		Usage:   "Read database objects",
		Commands: []*cli.Command{
			readRecord(homeDir, getWd, readDefinition, newDB, logf),
			readCollection(homeDir, getWd, readDefinition, logf),
		},
	}
}
