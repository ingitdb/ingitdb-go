package main

import (
	"errors"
	"io"
	"log"
	"os"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestRun_Version(t *testing.T) {
	t.Parallel()

	args := []string{"ingitdb", "--version"}
	readCalled := false
	fatalCalled := false
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		readCalled = true
		return nil, nil
	}
	fatal := func(error) {
		fatalCalled = true
	}
	homeDir := func() (string, error) {
		return "/tmp/home", nil
	}
	logf := func(...any) {}

	run(args, homeDir, readDefinition, fatal, logf)
	if readCalled {
		t.Fatal("readDefinition should not be called for --version")
	}
	if fatalCalled {
		t.Fatal("fatal should not be called for --version")
	}
}

func TestMain_VersionFlag(t *testing.T) {
	args := os.Args
	os.Args = []string{"ingitdb", "--version"}
	t.Cleanup(func() {
		os.Args = args
	})

	main()
}

func TestMain_ReadDefinitionError(t *testing.T) {
	args := os.Args
	os.Args = []string{"ingitdb", t.TempDir()}
	t.Cleanup(func() {
		os.Args = args
	})

	oldExit := exit
	exitCalled := false
	exit = func(int) {
		exitCalled = true
	}
	t.Cleanup(func() {
		exit = oldExit
	})

	oldOutput := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() {
		log.SetOutput(oldOutput)
	})

	main()

	if !exitCalled {
		t.Fatal("expected exit to be called")
	}
}

func TestRun_ReadDefinitionError(t *testing.T) {
	t.Parallel()

	args := []string{"ingitdb", "/tmp/db"}
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("boom")
	}
	fatalCalled := false
	fatal := func(error) {
		fatalCalled = true
	}
	homeDir := func() (string, error) {
		return "/tmp/home", nil
	}
	logf := func(...any) {}

	run(args, homeDir, readDefinition, fatal, logf)
	if !fatalCalled {
		t.Fatal("expected fatal to be called on readDefinition error")
	}
}

func TestExpandHome_NoTilde(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) {
		return "/tmp/home", nil
	}
	fatal := func(error) {}

	got := expandHome("/tmp/db", homeDir, fatal)
	if got != "/tmp/db" {
		t.Fatalf("expected /tmp/db, got %s", got)
	}
}

func TestExpandHome_Tilde(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) {
		return "/tmp/home", nil
	}
	fatal := func(error) {}

	got := expandHome("~", homeDir, fatal)
	if got != "/tmp/home" {
		t.Fatalf("expected /tmp/home, got %s", got)
	}
}

func TestExpandHome_HomeDirError(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) {
		return "", errors.New("no home")
	}
	fatalCalled := false
	fatal := func(error) {
		fatalCalled = true
	}

	got := expandHome("~", homeDir, fatal)
	if got != "" {
		t.Fatalf("expected empty result, got %s", got)
	}
	if !fatalCalled {
		t.Fatal("expected fatal to be called on homeDir error")
	}
}
