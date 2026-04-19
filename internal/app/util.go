package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"golang.org/x/term"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/player"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

const setupSoundCheckText = "Setup complete. This is a sound check from ttscli."

var (
	parseArgs             = cli.ParseArgs
	lookupEnv             = os.Getenv
	currentGOOS           = func() string { return runtime.GOOS }
	lookPathCmd           = exec.LookPath
	newProvider           = newProviderImpl
	loadConfig            = config.LoadConfig
	saveConfig            = config.SaveConfig
	getProfile            = config.GetProfile
	printVoices           = tts.PrintVoices
	writeFile             = os.WriteFile
	playAudio             = player.PlayAudio
	setupInput  io.Reader = os.Stdin
	readPassword          = readPasswordImpl
	newAppCtx             = func() (context.Context, context.CancelFunc) {
		return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	}
)

func readPasswordImpl() ([]byte, error) {
	return term.ReadPassword(int(os.Stdin.Fd()))
}

func newProviderImpl(profile config.Profile) (tts.Provider, error) {
	var creds interface{}
	if profile.Provider == "gcp" {
		if apiKey, ok := profile.Credentials["apiKey"].(string); ok {
			creds = apiKey
		} else {
			return nil, fmt.Errorf("gcp profile missing apiKey in credentials")
		}
	}
	return tts.NewProvider(profile.Provider, creds)
}
