package app

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

func TestRunSetupCommandSuccess(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, lang string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-US-Neural2-F"}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, lang, voice, enc string) ([]byte, error) {
				return []byte("audio"), nil
			},
		}, nil
	}
	playAudio = func(audio []byte, stdout, stderr io.Writer) error { return nil }
	readPassword = func() ([]byte, error) { return []byte("my-api-key"), nil }
	// default lang (enter), default voice (enter), no sound check
	setupInput = strings.NewReader("\n\nn\n")

	var stdout bytes.Buffer
	if err := runSetupCommand(&stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Setup complete!") {
		t.Errorf("expected setup complete! message, got: %q", out)
	}
	if !strings.Contains(out, "Created profile: gcp:default") {
		t.Errorf("expected profile creation message, got: %q", out)
	}
}

func TestRunSetupCommandEmptyAPIKey(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	readPassword = func() ([]byte, error) { return []byte(""), nil } // empty API key

	err := runSetupCommand(&bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "API key is required") {
		t.Fatalf("expected api key required error, got: %v", err)
	}
}

func TestRunSetupCommandProfileAlreadyExists(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	// stubAppDeps has gcp:default — entering "default" as profile name hits the already-exists path.
	setupInput = strings.NewReader("default\n")

	var stdout bytes.Buffer
	if err := runSetupCommand(&stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "already exists") {
		t.Errorf("expected already exists message, got: %q", stdout.String())
	}
}

func TestRunSetupCommandVoiceNotAvailable(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, lang string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-US-Neural2-A"}}, nil // different voice
			},
			synthesizeFn: nil,
		}, nil
	}
	readPassword = func() ([]byte, error) { return []byte("my-api-key"), nil }
	// requests en-US-Neural2-F which won't be in the list above
	setupInput = strings.NewReader("\nen-US-Neural2-F\n")

	err := runSetupCommand(&bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected voice not available error, got: %v", err)
	}
}

func TestRunSetupCommandSoundCheck(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, lang string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-US-Neural2-F"}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, lang, voice, enc string) ([]byte, error) {
				return []byte("audio"), nil
			},
		}, nil
	}
	playCalled := false
	playAudio = func(audio []byte, stdout, stderr io.Writer) error {
		playCalled = true
		return nil
	}
	readPassword = func() ([]byte, error) { return []byte("my-api-key"), nil }
	setupInput = strings.NewReader("\n\ny\n") // say yes to sound check

	if err := runSetupCommand(&bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !playCalled {
		t.Error("expected playAudio to be called for sound check")
	}
}
