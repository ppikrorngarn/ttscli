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

	code := runMain([]string{"speak", "--text", "hello"}, &bytes.Buffer{}, &bytes.Buffer{})
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

func TestRunMainVersionSkipsAppRun(t *testing.T) {
	reset := stubMainDeps()
	defer reset()

	version = "1.2.3"
	commit = "abc1234"
	date = "2026-04-12T00:00:00Z"

	runAppCalled := false
	runApp = func(args []string, stdout, stderr io.Writer) error {
		runAppCalled = true
		return nil
	}

	var stdout bytes.Buffer
	code := runMain([]string{"--version"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if runAppCalled {
		t.Fatal("expected runApp not to be called for --version")
	}
	out := stdout.String()
	if !strings.Contains(out, "version=1.2.3") || !strings.Contains(out, "commit=abc1234") {
		t.Fatalf("unexpected version output: %q", out)
	}
}

func TestHasVersionFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "exact flag", args: []string{"--version"}, want: true},
		{name: "assigned true", args: []string{"--version=true"}, want: true},
		{name: "assigned false", args: []string{"--version=false"}, want: false},
		{name: "missing", args: []string{"speak", "--text", "hello"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasVersionFlag(tt.args); got != tt.want {
				t.Fatalf("hasVersionFlag(%v)=%v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func stubMainDeps() func() {
	oldRunApp := runApp
	oldVersion := version
	oldCommit := commit
	oldDate := date
	return func() {
		runApp = oldRunApp
		version = oldVersion
		commit = oldCommit
		date = oldDate
	}
}
