package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ppikrorngarn/ttscli/internal/app"
)

var runApp = app.Run

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdout, os.Stderr))
}

func runMain(args []string, stdout, stderr io.Writer) int {
	err := runApp(args, stdout, stderr)
	if err == nil {
		return 0
	}
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintln(stderr, "Error:", err)
	return 1
}
