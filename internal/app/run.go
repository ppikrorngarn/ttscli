package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/player"
	"github.com/ppikrorngarn/ttscli/internal/tts"

	"github.com/joho/godotenv"
)

const apiKeyEnvVar = "TTSCLI_GOOGLE_API_KEY"

type ttsService interface {
	ListVoices(ctx context.Context, langCode string) ([]tts.Voice, error)
	Synthesize(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error)
}

var (
	parseArgs    = cli.ParseArgs
	loadDotenv   = godotenv.Load
	lookupEnv    = os.Getenv
	newTTSClient = func(apiKey string) ttsService { return tts.NewClient(apiKey, nil) }
	printVoices  = tts.PrintVoices
	writeFile    = os.WriteFile
	playAudio    = player.PlayAudio
	newAppCtx    = func() (context.Context, context.CancelFunc) {
		return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	}
)

func Run(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}

	// Load .env file if it exists, ignoring errors if it doesn't.
	_ = loadDotenv()

	apiKey := lookupEnv(apiKeyEnvVar)
	if apiKey == "" {
		return fmt.Errorf("%s environment variable is not set", apiKeyEnvVar)
	}

	client := newTTSClient(apiKey)
	ctx, stop := newAppCtx()
	defer stop()

	if cfg.ListVoices {
		voices, err := client.ListVoices(ctx, cfg.Lang)
		if err != nil {
			return fmt.Errorf("failed to list voices: %w", err)
		}
		printVoices(stdout, cfg.Lang, voices)
		return nil
	}

	fmt.Fprintln(stdout, "Synthesizing speech...")
	audioBytes, err := client.Synthesize(ctx, cfg.Text, cfg.Lang, cfg.Voice, tts.AudioEncodingMP3)
	if err != nil {
		return fmt.Errorf("failed to synthesize: %w", err)
	}

	if cfg.SavePath != "" {
		if err := writeFile(cfg.SavePath, audioBytes, 0o644); err != nil {
			return fmt.Errorf("failed to save file: %w", err)
		}
		fmt.Fprintf(stdout, "Saved audio to: %s\n", cfg.SavePath)
	}

	if cfg.Play {
		fmt.Fprintln(stdout, "Playing audio...")
		if err := playAudio(audioBytes, stdout, stderr); err != nil {
			return fmt.Errorf("failed to play audio: %w", err)
		}
	}

	return nil
}
