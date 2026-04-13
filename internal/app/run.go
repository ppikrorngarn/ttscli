package app

import (
	"fmt"
	"io"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

// Run is the public entry point for the CLI.
func Run(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}

	switch cfg.Mode {
	case cli.ModeDefault:
		return runDefaultCommand(cfg, stdout)
	case cli.ModeSetup:
		return runSetupCommand(stdout, stderr)
	case cli.ModeDoctor:
		return runDoctorCommand(stdout)
	case cli.ModeCompletion:
		return runCompletionCommand(cfg, stdout)
	case "":
		// Test compatibility when ParseArgs is stubbed without mode.
		cfg.Mode = cli.ModeSpeak
	case cli.ModeSpeak, cli.ModeSave, cli.ModeVoices:
		// Continue with synth/list flow below.
	default:
		return fmt.Errorf("unsupported mode: %s", cfg.Mode)
	}

	defaults, err := loadDefaults()
	if err != nil {
		return fmt.Errorf("load defaults: %w", err)
	}
	apiKey := resolveAPIKey(defaults.APIKey)
	if apiKey == "" {
		return fmt.Errorf("%s environment variable is not set and no saved API key found", apiKeyEnvVar)
	}

	client := newTTSClient(apiKey)
	cfg = resolveRunDefaults(cfg, defaults)
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
