package main

import (
	"errors"
	"flag"
	"os"

	"github.com/ppikrorngarn/ttscli/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		app.ExitErr(os.Stderr, err)
	}
}
