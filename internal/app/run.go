package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
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
	parseArgs     = cli.ParseArgs
	loadDotenv    = godotenv.Load
	lookupEnv     = os.Getenv
	newTTSClient  = func(apiKey string) ttsService { return tts.NewClient(apiKey, nil) }
	loadDefaults  = config.LoadDefaults
	saveDefaults  = config.SaveDefaults
	clearDefaults = config.ClearDefaults
	printVoices   = tts.PrintVoices
	writeFile     = os.WriteFile
	playAudio     = player.PlayAudio
	newAppCtx     = func() (context.Context, context.CancelFunc) {
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

	switch cfg.Mode {
	case cli.ModeDefault:
		return runDefaultCommand(cfg, stdout)
	case "":
		// For backward compatibility in tests that may stub ParseArgs manually.
		cfg.Mode = cli.ModeRun
	case cli.ModeRun:
		// Continue with synth/list flow below.
	default:
		return fmt.Errorf("unsupported mode: %s", cfg.Mode)
	}

	apiKey := lookupEnv(apiKeyEnvVar)
	if apiKey == "" {
		return fmt.Errorf("%s environment variable is not set", apiKeyEnvVar)
	}
	client := newTTSClient(apiKey)
	ctx, stop := newAppCtx()
	defer stop()

	cfg, err = resolveRunDefaults(cfg)
	if err != nil {
		return err
	}

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

func runDefaultCommand(cfg cli.Config, stdout io.Writer) error {
	switch cfg.DefaultSubcommand {
	case cli.DefaultGet:
		defaults, err := loadDefaults()
		if err != nil {
			return fmt.Errorf("load defaults: %w", err)
		}
		voice := defaults.Voice
		lang := defaults.Lang
		if strings.TrimSpace(voice) == "" {
			voice = "(not set)"
		}
		if strings.TrimSpace(lang) == "" {
			lang = "(not set)"
		}
		fmt.Fprintf(stdout, "Default voice: %s\n", voice)
		fmt.Fprintf(stdout, "Default language: %s\n", lang)
		return nil
	case cli.DefaultUnset:
		if err := clearDefaults(); err != nil {
			return fmt.Errorf("clear defaults: %w", err)
		}
		fmt.Fprintln(stdout, "Cleared saved defaults.")
		return nil
	case cli.DefaultSet:
		return runDefaultSet(cfg, stdout)
	default:
		return fmt.Errorf("unsupported default subcommand: %s", cfg.DefaultSubcommand)
	}
}

func runDefaultSet(cfg cli.Config, stdout io.Writer) error {
	existing, err := loadDefaults()
	if err != nil {
		return fmt.Errorf("load defaults: %w", err)
	}

	merged := existing
	if cfg.HasVoiceFlag {
		merged.Voice = cfg.Voice
	}
	if cfg.HasLangFlag {
		merged.Lang = cfg.Lang
	}
	if strings.TrimSpace(merged.Lang) == "" {
		merged.Lang = cli.DefaultLanguage
	}
	if strings.TrimSpace(merged.Voice) == "" {
		merged.Voice = cli.DefaultVoice
	}

	apiKey := lookupEnv(apiKeyEnvVar)
	if apiKey == "" {
		return fmt.Errorf("%s environment variable is not set", apiKeyEnvVar)
	}
	ctx, stop := newAppCtx()
	defer stop()

	client := newTTSClient(apiKey)
	voices, err := client.ListVoices(ctx, merged.Lang)
	if err != nil {
		return fmt.Errorf("validate defaults via list voices: %w", err)
	}
	if !voiceExists(merged.Voice, voices) {
		return fmt.Errorf("voice %q is not available for language %q", merged.Voice, merged.Lang)
	}

	if err := saveDefaults(merged); err != nil {
		return fmt.Errorf("save defaults: %w", err)
	}
	fmt.Fprintf(stdout, "Saved defaults: voice=%s lang=%s\n", merged.Voice, merged.Lang)
	return nil
}

func resolveRunDefaults(cfg cli.Config) (cli.Config, error) {
	defaults, err := loadDefaults()
	if err != nil {
		return cfg, fmt.Errorf("load defaults: %w", err)
	}

	if !cfg.HasVoiceFlag && strings.TrimSpace(defaults.Voice) != "" {
		cfg.Voice = defaults.Voice
	}
	if !cfg.HasLangFlag && strings.TrimSpace(defaults.Lang) != "" {
		cfg.Lang = defaults.Lang
	}
	return cfg, nil
}

func voiceExists(voiceName string, voices []tts.Voice) bool {
	for _, v := range voices {
		if strings.EqualFold(v.Name, voiceName) {
			return true
		}
	}
	return false
}
