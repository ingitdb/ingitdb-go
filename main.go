package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func main() {
	dirPath := expandHome(os.Args[1])
	log.Println("inGitDB db path: ", dirPath)

	err := ingitdb.Validate(dirPath)
	if err != nil {
		log.Fatal(fmt.Errorf("inGitDB database validation failed: %w", err))
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(fmt.Errorf("failed to expand home directory: %w", err))
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
