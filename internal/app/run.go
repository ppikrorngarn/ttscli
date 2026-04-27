package app

import (
	"fmt"
	"io"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

// Run is the public entry point for the CLI.
func Run(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}

	switch cfg.Mode {
	case cli.ModeSetup:
		return runSetupCommand(stdout, stderr)
	case cli.ModeDoctor:
		return runDoctorCommand(stdout)
	case cli.ModeCompletion:
		return runCompletionCommand(cfg, stdout)
	case cli.ModeProfile:
		return runProfileCommand(cfg, stdout)
	case "":
		// Test compatibility when ParseArgs is stubbed without mode.
		cfg.Mode = cli.ModeSpeak
	case cli.ModeSpeak, cli.ModeSave, cli.ModeVoices:
		// Continue with synth/list flow below.
	default:
		return fmt.Errorf("unsupported mode: %s. Run 'ttscli --help' for available commands", cfg.Mode)
	}

	// Load profile-based config
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Determine which profile to use
	var profileKey string
	if cfg.Profile != "" {
		var err error
		profileKey, _, _, err = config.ParseProfileKey(cfg.Profile)
		if err != nil {
			return err
		}
	} else {
		// Use active profile from config
		if appCfg.ActiveProvider == "" || appCfg.ActiveProfile == "" {
			return fmt.Errorf("no active profile set. Run 'ttscli profile use <provider:name>' or use --profile flag")
		}
		profileKey = appCfg.ActiveProvider + ":" + appCfg.ActiveProfile
	}

	// Get the profile
	profile, err := getProfile(appCfg, profileKey)
	if err != nil {
		return fmt.Errorf("get profile %q: %w", profileKey, err)
	}

	// Create provider
	provider, err := newProvider(profile)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}
	cfg = resolveProfileDefaults(cfg, profile)

	// Execute with provider
	return runWithProvider(cfg, provider, stdout, stderr)
}

func runWithProvider(cfg cli.Config, provider tts.Provider, stdout, stderr io.Writer) error {
	ctx, stop := newAppCtx()
	defer stop()

	if cfg.ListVoices {
		voices, err := provider.ListVoices(ctx, cfg.Lang)
		if err != nil {
			return fmt.Errorf("failed to list voices: %w", err)
		}
		printVoices(stdout, cfg.Lang, voices)
		return nil
	}

	fmt.Fprintln(stdout, "Synthesizing speech...")
	req := tts.SynthRequest{
		Text:          cfg.Text,
		LanguageCode:  cfg.Lang,
		VoiceName:     cfg.Voice,
		AudioEncoding: tts.AudioEncodingMP3,
	}
	audioBytes, err := provider.SynthesizeRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to synthesize speech: %w", err)
	}

	if cfg.SavePath != "" {
		if err := writeFile(cfg.SavePath, audioBytes, 0o644); err != nil {
			return fmt.Errorf("failed to save audio file: %w", err)
		}
		fmt.Fprintf(stdout, "✓ Saved audio to: %s\n", cfg.SavePath)
	}

	if cfg.Play {
		fmt.Fprintln(stdout, "Playing audio...")
		if err := playAudio(audioBytes, stdout, stderr); err != nil {
			return fmt.Errorf("failed to play audio: %w", err)
		}
		fmt.Fprintln(stdout, "Playback complete.")
	}

	return nil
}
