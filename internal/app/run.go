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
)

const apiKeyEnvVar = "TTSCLI_GOOGLE_API_KEY"

type ttsService interface {
	ListVoices(ctx context.Context, langCode string) ([]tts.Voice, error)
	Synthesize(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error)
}

var (
	parseArgs     = cli.ParseArgs
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
		fmt.Fprintf(stdout, "Default API key: %s\n", maskAPIKey(defaults.APIKey))
		return nil
	case cli.DefaultUnset:
		return runDefaultUnset(cfg, stdout)
	case cli.DefaultSet:
		return runDefaultSet(cfg, stdout)
	default:
		return fmt.Errorf("unsupported default subcommand: %s", cfg.DefaultSubcommand)
	}
}

func runDefaultUnset(cfg cli.Config, stdout io.Writer) error {
	if !cfg.HasVoiceFlag && !cfg.HasLangFlag && !cfg.HasAPIKeyFlag {
		if err := clearDefaults(); err != nil {
			return fmt.Errorf("clear defaults: %w", err)
		}
		fmt.Fprintln(stdout, "Cleared saved defaults.")
		return nil
	}

	defaults, err := loadDefaults()
	if err != nil {
		return fmt.Errorf("load defaults: %w", err)
	}

	if cfg.HasVoiceFlag {
		defaults.Voice = ""
	}
	if cfg.HasLangFlag {
		defaults.Lang = ""
	}
	if cfg.HasAPIKeyFlag {
		defaults.APIKey = ""
	}

	if defaultsEmpty(defaults) {
		if err := clearDefaults(); err != nil {
			return fmt.Errorf("clear defaults: %w", err)
		}
	} else {
		if err := saveDefaults(defaults); err != nil {
			return fmt.Errorf("save defaults: %w", err)
		}
	}

	fmt.Fprintln(stdout, "Updated saved defaults.")
	return nil
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
	if cfg.HasAPIKeyFlag {
		merged.APIKey = cfg.APIKey
	}
	if strings.TrimSpace(merged.Lang) == "" {
		merged.Lang = cli.DefaultLanguage
	}
	if strings.TrimSpace(merged.Voice) == "" {
		merged.Voice = cli.DefaultVoice
	}

	validationKey := resolveAPIKey(merged.APIKey)
	if cfg.HasAPIKeyFlag {
		validationKey = strings.TrimSpace(merged.APIKey)
	}
	if validationKey == "" {
		return fmt.Errorf("%s environment variable is not set and no saved API key found", apiKeyEnvVar)
	}

	ctx, stop := newAppCtx()
	defer stop()

	client := newTTSClient(validationKey)
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
	fmt.Fprintf(stdout, "Saved defaults: voice=%s lang=%s apiKey=%s\n", merged.Voice, merged.Lang, maskAPIKey(merged.APIKey))
	return nil
}

func resolveRunDefaults(cfg cli.Config, defaults config.Defaults) cli.Config {
	if !cfg.HasVoiceFlag && strings.TrimSpace(defaults.Voice) != "" {
		cfg.Voice = defaults.Voice
	}
	if !cfg.HasLangFlag && strings.TrimSpace(defaults.Lang) != "" {
		cfg.Lang = defaults.Lang
	}
	return cfg
}

func resolveAPIKey(savedAPIKey string) string {
	if envKey := strings.TrimSpace(lookupEnv(apiKeyEnvVar)); envKey != "" {
		return envKey
	}
	return strings.TrimSpace(savedAPIKey)
}

func defaultsEmpty(d config.Defaults) bool {
	return strings.TrimSpace(d.Voice) == "" &&
		strings.TrimSpace(d.Lang) == "" &&
		strings.TrimSpace(d.APIKey) == ""
}

func maskAPIKey(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "(not set)"
	}
	if len(trimmed) <= 4 {
		return "****"
	}
	return "****" + trimmed[len(trimmed)-4:]
}

func voiceExists(voiceName string, voices []tts.Voice) bool {
	for _, v := range voices {
		if strings.EqualFold(v.Name, voiceName) {
			return true
		}
	}
	return false
}
