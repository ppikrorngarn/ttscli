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

	// Try to use profile-based config first
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var provider tts.Provider
	var profileKey string

	// Check if user specified a profile via flag or env
	if cfg.Profile != "" {
		profileKey = cfg.Profile
	} else if envProfile := lookupEnv("TTSCLI_PROFILE"); envProfile != "" {
		profileKey = envProfile
	} else {
		// Use active profile from config
		profileKey = appCfg.ActiveProvider + ":" + appCfg.ActiveProfile
	}

	// Try to get the profile
	profile, err := getProfile(appCfg, profileKey)
	if err != nil {
		// Fall back to legacy defaults if no profiles exist
		defaults, loadErr := loadDefaults()
		if loadErr != nil {
			return fmt.Errorf("load config and defaults both failed: %w (config error: %v)", loadErr, err)
		}
		apiKey := resolveAPIKey(defaults.APIKey)
		if apiKey == "" {
			return fmt.Errorf("%s environment variable is not set and no saved API key found", apiKeyEnvVar)
		}
		client := newTTSClient(apiKey)
		cfg = resolveRunDefaults(cfg, defaults)
		return runWithClient(cfg, client, stdout, stderr)
	}

	// Use profile-based provider
	provider, err = newProvider(profile)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}
	cfg = resolveProfileDefaults(cfg, profile)
	return runWithProvider(cfg, provider, stdout, stderr)
}

func runWithClient(cfg cli.Config, client ttsService, stdout, stderr io.Writer) error {
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
