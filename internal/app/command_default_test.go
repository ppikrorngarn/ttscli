package app

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

func TestRunDefaultGet(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: "default", DefaultSubcommand: "get"}, nil
	}
	loadDefaults = func() (config.Defaults, error) {
		return config.Defaults{Voice: "en-US-Chirp3-HD-Achernar", Lang: "en-US", APIKey: "abcd1234"}, nil
	}

	var stdout bytes.Buffer
	err := Run([]string{"default", "get"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Default voice: en-US-Chirp3-HD-Achernar") ||
		!strings.Contains(out, "Default language: en-US") ||
		!strings.Contains(out, "Default API key: ****1234") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunDefaultUnset(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: "default", DefaultSubcommand: "unset"}, nil
	}
	cleared := false
	clearDefaults = func() error {
		cleared = true
		return nil
	}

	var stdout bytes.Buffer
	err := Run([]string{"default", "unset"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !cleared {
		t.Fatal("expected clearDefaults to be called")
	}
	if !strings.Contains(stdout.String(), "Cleared saved defaults.") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestRunDefaultSetValidatesAndSaves(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{
			Mode:              "default",
			DefaultSubcommand: "set",
			Voice:             "en-US-Chirp3-HD-Achernar",
			HasVoiceFlag:      true,
		}, nil
	}
	lookupEnv = func(_ string) string { return "k" }
	loadDefaults = func() (config.Defaults, error) {
		return config.Defaults{Lang: "en-US"}, nil
	}
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-US-Chirp3-HD-Achernar"}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				t.Fatal("Synthesize should not be called")
				return nil, nil
			},
		}
	}

	var saved config.Defaults
	saveDefaults = func(d config.Defaults) error {
		saved = d
		return nil
	}

	var stdout bytes.Buffer
	err := Run([]string{"default", "set", "--voice", "en-US-Chirp3-HD-Achernar"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if saved.Voice != "en-US-Chirp3-HD-Achernar" || saved.Lang != "en-US" {
		t.Fatalf("unexpected saved defaults: %+v", saved)
	}
	if !strings.Contains(stdout.String(), "Saved defaults:") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestRunDefaultSetAPIKeyValidatesProvidedKey(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{
			Mode:              cli.ModeDefault,
			DefaultSubcommand: cli.DefaultSet,
			APIKey:            "new-key",
			HasAPIKeyFlag:     true,
		}, nil
	}
	lookupEnv = func(_ string) string { return "env-key" }
	loadDefaults = func() (config.Defaults, error) {
		return config.Defaults{Lang: "en-US", Voice: "en-US-Neural2-F", APIKey: "old-key"}, nil
	}

	var usedAPIKey string
	newTTSClient = func(apiKey string) ttsService {
		usedAPIKey = apiKey
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-US-Neural2-F"}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return nil, nil
			},
		}
	}

	var saved config.Defaults
	saveDefaults = func(d config.Defaults) error {
		saved = d
		return nil
	}

	err := Run([]string{"default", "set", "--api-key", "new-key"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if usedAPIKey != "new-key" {
		t.Fatalf("expected new api key to be validated, got %q", usedAPIKey)
	}
	if saved.APIKey != "new-key" {
		t.Fatalf("expected saved api key to be new-key, got %+v", saved)
	}
}

func TestRunDefaultSetVoiceValidationError(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{
			Mode:              "default",
			DefaultSubcommand: "set",
			Voice:             "voice-not-found",
			HasVoiceFlag:      true,
			Lang:              "en-US",
			HasLangFlag:       true,
		}, nil
	}
	lookupEnv = func(_ string) string { return "k" }
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-US-Neural2-F"}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return nil, nil
			},
		}
	}

	err := Run([]string{"default", "set", "--voice", "voice-not-found", "--lang", "en-US"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "is not available for language") {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestRunDefaultUnsetSelectedFieldsOnly(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{
			Mode:              cli.ModeDefault,
			DefaultSubcommand: cli.DefaultUnset,
			HasAPIKeyFlag:     true,
		}, nil
	}
	loadDefaults = func() (config.Defaults, error) {
		return config.Defaults{
			Voice:  "en-US-Chirp3-HD-Achernar",
			Lang:   "en-US",
			APIKey: "old-key",
		}, nil
	}

	var saved config.Defaults
	saveDefaults = func(d config.Defaults) error {
		saved = d
		return nil
	}

	var stdout bytes.Buffer
	err := Run([]string{"default", "unset", "--api-key"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if saved.APIKey != "" || saved.Voice == "" || saved.Lang == "" {
		t.Fatalf("expected only api key to be cleared, got %+v", saved)
	}
	if !strings.Contains(stdout.String(), "Updated saved defaults.") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestRunUsesSavedAPIKeyWhenEnvMissing(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{
			Mode:       cli.ModeSpeak,
			Text:       "hello",
			Play:       true,
			Lang:       cli.DefaultLanguage,
			Voice:      cli.DefaultVoice,
			ListVoices: false,
		}, nil
	}
	lookupEnv = func(_ string) string { return "" }
	loadDefaults = func() (config.Defaults, error) {
		return config.Defaults{APIKey: "saved-key"}, nil
	}

	var usedAPIKey string
	newTTSClient = func(apiKey string) ttsService {
		usedAPIKey = apiKey
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) { return nil, nil },
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return []byte("audio"), nil
			},
		}
	}
	playAudio = func(audioBytes []byte, stdout, stderr io.Writer) error { return nil }

	err := Run([]string{"speak", "--text", "hello"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if usedAPIKey != "saved-key" {
		t.Fatalf("expected saved api key to be used, got %q", usedAPIKey)
	}
}

func TestRunEnvAPIKeyOverridesSavedKey(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{
			Mode:       cli.ModeSpeak,
			Text:       "hello",
			Play:       true,
			Lang:       cli.DefaultLanguage,
			Voice:      cli.DefaultVoice,
			ListVoices: false,
		}, nil
	}
	lookupEnv = func(_ string) string { return "env-key" }
	loadDefaults = func() (config.Defaults, error) {
		return config.Defaults{APIKey: "saved-key"}, nil
	}

	var usedAPIKey string
	newTTSClient = func(apiKey string) ttsService {
		usedAPIKey = apiKey
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) { return nil, nil },
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return []byte("audio"), nil
			},
		}
	}
	playAudio = func(audioBytes []byte, stdout, stderr io.Writer) error { return nil }

	err := Run([]string{"speak", "--text", "hello"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if usedAPIKey != "env-key" {
		t.Fatalf("expected env api key to override saved key, got %q", usedAPIKey)
	}
}

func TestRunUsesPersistedDefaultsWhenFlagsNotProvided(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{
			Mode:       cli.ModeSpeak,
			Text:       "hello",
			Play:       true,
			Lang:       cli.DefaultLanguage,
			Voice:      cli.DefaultVoice,
			ListVoices: false,
		}, nil
	}
	lookupEnv = func(_ string) string { return "k" }
	loadDefaults = func() (config.Defaults, error) {
		return config.Defaults{Voice: "en-US-Chirp3-HD-Achernar", Lang: "en-US"}, nil
	}

	var gotVoice, gotLang string
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) { return nil, nil },
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				gotVoice = voiceName
				gotLang = languageCode
				return []byte("audio"), nil
			},
		}
	}
	playAudio = func(audioBytes []byte, stdout, stderr io.Writer) error { return nil }

	if err := Run([]string{"speak", "--text", "hello"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if gotVoice != "en-US-Chirp3-HD-Achernar" || gotLang != "en-US" {
		t.Fatalf("expected persisted defaults to be applied, got voice=%q lang=%q", gotVoice, gotLang)
	}
}

func TestRunExplicitFlagsOverridePersistedDefaults(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{
			Mode:         cli.ModeSpeak,
			Text:         "hello",
			Play:         true,
			Lang:         "en-GB",
			Voice:        "en-GB-Neural2-B",
			HasVoiceFlag: true,
			HasLangFlag:  true,
		}, nil
	}
	lookupEnv = func(_ string) string { return "k" }
	loadDefaults = func() (config.Defaults, error) {
		return config.Defaults{Voice: "en-US-Chirp3-HD-Achernar", Lang: "en-US"}, nil
	}

	var gotVoice, gotLang string
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) { return nil, nil },
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				gotVoice = voiceName
				gotLang = languageCode
				return []byte("audio"), nil
			},
		}
	}
	playAudio = func(audioBytes []byte, stdout, stderr io.Writer) error { return nil }

	if err := Run([]string{"speak", "--text", "hello", "--voice", "en-GB-Neural2-B", "--lang", "en-GB"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if gotVoice != "en-GB-Neural2-B" || gotLang != "en-GB" {
		t.Fatalf("expected explicit flags to win, got voice=%q lang=%q", gotVoice, gotLang)
	}
}
