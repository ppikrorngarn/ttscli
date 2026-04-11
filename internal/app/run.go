package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/player"
	"github.com/ppikrorngarn/ttscli/internal/tts"

	"github.com/joho/godotenv"
)

func Run(args []string, stdout, stderr io.Writer) error {
	cfg, err := cli.ParseArgs(args, stderr)
	if err != nil {
		return err
	}

	// Load .env file if it exists, ignoring errors if it doesn't.
	_ = godotenv.Load()

	apiKey := os.Getenv("TTSCLI_GOOGLE_API_KEY")
	if apiKey == "" {
		return errors.New("TTSCLI_GOOGLE_API_KEY environment variable is not set")
	}

	client := tts.NewClient(apiKey, nil)
	ctx := context.Background()

	if cfg.ListVoices {
		voices, err := client.ListVoices(ctx, cfg.Lang)
		if err != nil {
			return fmt.Errorf("failed to list voices: %w", err)
		}
		tts.PrintVoices(stdout, cfg.Lang, voices)
		return nil
	}

	fmt.Fprintln(stdout, "Synthesizing speech...")
	audioBytes, err := client.Synthesize(ctx, cfg.Text, cfg.Lang, cfg.Voice, "MP3")
	if err != nil {
		return fmt.Errorf("failed to synthesize: %w", err)
	}

	if cfg.SavePath != "" {
		if err := os.WriteFile(cfg.SavePath, audioBytes, 0o644); err != nil {
			return fmt.Errorf("failed to save file: %w", err)
		}
		fmt.Fprintf(stdout, "Saved audio to: %s\n", cfg.SavePath)
	}

	if cfg.Play {
		fmt.Fprintln(stdout, "Playing audio...")
		if err := player.PlayAudio(audioBytes, stdout, stderr); err != nil {
			return fmt.Errorf("failed to play audio: %w", err)
		}
	}

	return nil
}

func ExitErr(stderr io.Writer, err error) {
	fmt.Fprintln(stderr, "Error:", err)
	os.Exit(1)
}
