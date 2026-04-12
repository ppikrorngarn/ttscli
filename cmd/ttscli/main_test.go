package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestRunMainSuccess(t *testing.T) {
	reset := stubMainDeps()
	defer reset()

	runApp = func(args []string, stdout, stderr io.Writer) error {
		return nil
	}

	code := runMain([]string{"--text", "hello", "--play"}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunMainHelpError(t *testing.T) {
	reset := stubMainDeps()
	defer reset()

	runApp = func(args []string, stdout, stderr io.Writer) error {
		return flag.ErrHelp
	}

	var stderr bytes.Buffer
	code := runMain([]string{"--help"}, &bytes.Buffer{}, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr for help, got %q", stderr.String())
	}
}

func TestRunMainError(t *testing.T) {
	reset := stubMainDeps()
	defer reset()

	runApp = func(args []string, stdout, stderr io.Writer) error {
		return errors.New("boom")
	}

	var stderr bytes.Buffer
	code := runMain([]string{"--bad"}, &bytes.Buffer{}, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Error: boom") {
		t.Fatalf("expected stderr error output, got %q", stderr.String())
	}
}

func stubMainDeps() func() {
	oldRunApp := runApp
	return func() {
		runApp = oldRunApp
	}
}
