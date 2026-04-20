package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ppikrorngarn/ttscli/internal/app"
)

var runApp = app.Run

// Build metadata injected at link time via ldflags.
// Values default to "dev"/"none"/"unknown" for local builds.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdout, os.Stderr))
}

func runMain(args []string, stdout, stderr io.Writer) int {
	if hasVersionFlag(args) {
		fmt.Fprintf(stdout, "ttscli version=%s commit=%s date=%s\n", version, commit, date)
		return 0
	}

	err := runApp(args, stdout, stderr)
	if err == nil {
		return 0
	}
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintln(stderr, "Error: "+err.Error())
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Run 'ttscli --help' for usage information.")
	return 1
}

func hasVersionFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--version" {
			return true
		}
		// Handle common shell forms like --version=true/false.
		if strings.HasPrefix(arg, "--version=") {
			return arg != "--version=false"
		}
	}
	return false
}
