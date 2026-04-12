package app

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/player"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

const apiKeyEnvVar = "TTSCLI_GOOGLE_API_KEY"
const setupSoundCheckText = "Setup complete. This is a sound check from ttscli."

type ttsService interface {
	ListVoices(ctx context.Context, langCode string) ([]tts.Voice, error)
	Synthesize(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error)
}

var (
	parseArgs               = cli.ParseArgs
	lookupEnv               = os.Getenv
	currentGOOS             = func() string { return runtime.GOOS }
	lookPathCmd             = exec.LookPath
	newTTSClient            = func(apiKey string) ttsService { return tts.NewClient(apiKey, nil) }
	loadDefaults            = config.LoadDefaults
	saveDefaults            = config.SaveDefaults
	clearDefaults           = config.ClearDefaults
	printVoices             = tts.PrintVoices
	writeFile               = os.WriteFile
	playAudio               = player.PlayAudio
	setupInput    io.Reader = os.Stdin
	newAppCtx               = func() (context.Context, context.CancelFunc) {
		return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	}
)
