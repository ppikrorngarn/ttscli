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

func TestRunSetupUsesBuiltInDefaultsOnEnter(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeSetup}, nil
	}
	lookupEnv = func(_ string) string { return "" }
	setupInput = strings.NewReader("new-key\n\n\nn\n")

	newTTSClient = func(apiKey string) ttsService {
		if apiKey != "new-key" {
			t.Fatalf("unexpected api key: %q", apiKey)
		}
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				if langCode != cli.DefaultLanguage {
					t.Fatalf("unexpected lang: %q", langCode)
				}
				return []tts.Voice{{Name: cli.DefaultVoice}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				t.Fatal("sound check should not run")
				return nil, nil
			},
		}
	}
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		if apiKey, ok := profile.Credentials["apiKey"].(string); ok && apiKey != "new-key" {
			t.Fatalf("unexpected api key: %q", apiKey)
		}
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				if langCode != cli.DefaultLanguage {
					t.Fatalf("unexpected lang: %q", langCode)
				}
				return []tts.Voice{{Name: cli.DefaultVoice}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				t.Fatal("sound check should not run")
				return nil, nil
			},
		}, nil
	}

	var saved config.Defaults
	saveDefaults = func(d config.Defaults) error {
		saved = d
		return nil
	}

	if err := Run([]string{"setup"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if saved.APIKey != "new-key" || saved.Lang != cli.DefaultLanguage || saved.Voice != cli.DefaultVoice {
		t.Fatalf("unexpected saved defaults: %+v", saved)
	}
}

func TestRunSetupUsesEnvAPIKeyWhenPromptEmpty(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeSetup}, nil
	}
	lookupEnv = func(_ string) string { return "env-key" }
	setupInput = strings.NewReader("\n\n\nn\n")

	newTTSClient = func(apiKey string) ttsService {
		if apiKey != "env-key" {
			t.Fatalf("expected env key to be used, got %q", apiKey)
		}
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: cli.DefaultVoice}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return nil, nil
			},
		}
	}
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		if apiKey, ok := profile.Credentials["apiKey"].(string); ok && apiKey != "env-key" {
			t.Fatalf("expected env key to be used, got %q", apiKey)
		}
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: cli.DefaultVoice}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return nil, nil
			},
		}, nil
	}

	err := Run([]string{"setup"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestRunSetupMissingAPIKey(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeSetup}, nil
	}
	lookupEnv = func(_ string) string { return "" }
	loadDefaults = func() (config.Defaults, error) { return config.Defaults{}, nil }
	setupInput = strings.NewReader("\n\n\nn\n")

	err := Run([]string{"setup"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "api key is required") {
		t.Fatalf("expected missing api key error, got: %v", err)
	}
}

func TestRunSetupSoundCheckYes(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeSetup}, nil
	}
	lookupEnv = func(_ string) string { return "" }
	setupInput = strings.NewReader("new-key\n\n\ny\n")

	synthCalled := false
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: cli.DefaultVoice}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				synthCalled = true
				return []byte("audio"), nil
			},
		}
	}
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: cli.DefaultVoice}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				synthCalled = true
				return []byte("audio"), nil
			},
		}, nil
	}

	playCalled := false
	playAudio = func(audioBytes []byte, stdout, stderr io.Writer) error {
		playCalled = true
		return nil
	}

	err := Run([]string{"setup"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !synthCalled || !playCalled {
		t.Fatalf("expected sound check to synthesize and play, got synth=%v play=%v", synthCalled, playCalled)
	}
}
