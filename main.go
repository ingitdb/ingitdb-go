package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/validator"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	exit    = os.Exit
)

func main() {
	fatal := func(err error) {
		log.Print(err)
		exit(1)
	}
	run(os.Args, os.UserHomeDir, validator.ReadDefinition, fatal, log.Println)
}

func run(
	args []string,
	homeDir func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	fatal func(error),
	logf func(...any),
) {
	if args[1] == "--version" {
		fmt.Printf("ingitdb %s (%s) @ %s\n", version, commit, date)
		return
	}

	dirPath := expandHome(args[1], homeDir, fatal)
	logf("inGitDB db path: ", dirPath)

	_, err := readDefinition(dirPath, ingitdb.Validate())
	if err != nil {
		fatal(fmt.Errorf("inGitDB database validation failed: %w", err))
	}
}

func expandHome(path string, homeDir func() (string, error), fatal func(error)) string {
	if strings.HasPrefix(path, "~") {
		home, err := homeDir()
		if err != nil {
			fatal(fmt.Errorf("failed to expand home directory: %w", err))
			return ""
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
