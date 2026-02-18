package main

import (
	"errors"
	"os"
	"testing"

	"github.com/ingitdb/ingitdb-go/pkg/ingitdb"
)

func TestRun_Version(t *testing.T) {
	t.Parallel()

	args := []string{"ingitdb", "version"}
	readCalled := false
	fatalCalled := false
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		readCalled = true
		return nil, nil
	}
	fatal := func(error) {
		fatalCalled = true
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	logf := func(...any) {}

	run(args, homeDir, getWd, readDefinition, fatal, logf)
	if readCalled {
		t.Fatal("readDefinition should not be called for version")
	}
	if fatalCalled {
		t.Fatal("fatal should not be called for version")
	}
}

func TestRun_NoSubcommand(t *testing.T) {
	t.Parallel()

	args := []string{"ingitdb"}
	fatalCalled := false
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, nil
	}
	fatal := func(error) {
		fatalCalled = true
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	logf := func(...any) {}

	run(args, homeDir, getWd, readDefinition, fatal, logf)
	if fatalCalled {
		t.Fatal("fatal should not be called when no subcommand given")
	}
}

func TestRun_ValidateSuccess(t *testing.T) {
	t.Parallel()

	readCalled := false
	var readPath string
	readDefinition := func(path string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		readCalled = true
		readPath = path
		return &ingitdb.Definition{}, nil
	}
	fatalCalled := false
	fatal := func(error) { fatalCalled = true }
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	logf := func(...any) {}

	run([]string{"ingitdb", "validate", "--path=/valid/dir"}, homeDir, getWd, readDefinition, fatal, logf)
	if !readCalled {
		t.Fatal("readDefinition should be called")
	}
	if readPath != "/valid/dir" {
		t.Fatalf("expected path /valid/dir, got %s", readPath)
	}
	if fatalCalled {
		t.Fatal("fatal should not be called on success")
	}
}

func TestRun_ValidateError(t *testing.T) {
	t.Parallel()

	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("boom")
	}
	fatalCalled := false
	fatal := func(error) { fatalCalled = true }
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	logf := func(...any) {}

	run([]string{"ingitdb", "validate", "--path=/x"}, homeDir, getWd, readDefinition, fatal, logf)
	if !fatalCalled {
		t.Fatal("fatal should be called on readDefinition error")
	}
}

func TestRun_ValidateDefaultPath(t *testing.T) {
	t.Parallel()

	var readPath string
	readDefinition := func(path string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		readPath = path
		return &ingitdb.Definition{}, nil
	}
	fatalCalled := false
	fatal := func(error) { fatalCalled = true }
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/wd", nil }
	logf := func(...any) {}

	run([]string{"ingitdb", "validate"}, homeDir, getWd, readDefinition, fatal, logf)
	if fatalCalled {
		t.Fatal("fatal should not be called")
	}
	if readPath != "/wd" {
		t.Fatalf("expected path /wd, got %s", readPath)
	}
}

func TestRun_ValidateHomePath(t *testing.T) {
	t.Parallel()

	var readPath string
	readDefinition := func(path string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		readPath = path
		return &ingitdb.Definition{}, nil
	}
	fatalCalled := false
	fatal := func(error) { fatalCalled = true }
	homeDir := func() (string, error) { return "/home/user", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	logf := func(...any) {}

	run([]string{"ingitdb", "validate", "--path=~/db"}, homeDir, getWd, readDefinition, fatal, logf)
	if fatalCalled {
		t.Fatal("fatal should not be called")
	}
	if readPath != "/home/user/db" {
		t.Fatalf("expected /home/user/db, got %s", readPath)
	}
}

func TestExpandHome_NoTilde(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }

	got, err := expandHome("/tmp/db", homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/db" {
		t.Fatalf("expected /tmp/db, got %s", got)
	}
}

func TestExpandHome_Tilde(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }

	got, err := expandHome("~", homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/home" {
		t.Fatalf("expected /tmp/home, got %s", got)
	}
}

func TestExpandHome_Error(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "", errors.New("no home") }

	got, err := expandHome("~", homeDir)
	if err == nil {
		t.Fatal("expected error")
	}
	if got != "" {
		t.Fatalf("expected empty result, got %s", got)
	}
}

func TestMain_VersionCmd(t *testing.T) {
	args := os.Args
	os.Args = []string{"ingitdb", "version"}
	t.Cleanup(func() {
		os.Args = args
	})

	main()
}

func TestMain_ReadDefinitionError(t *testing.T) {
	args := os.Args
	os.Args = []string{"ingitdb", "validate", "--path=" + t.TempDir()}
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

	oldStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = devNull.Close()
	})

	main()

	if !exitCalled {
		t.Fatal("expected exit to be called")
	}
}

