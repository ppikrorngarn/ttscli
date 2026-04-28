package app

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

func TestRunProfileListShowsProfiles(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	var stdout bytes.Buffer
	if err := runProfileList(&stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "gcp:default") {
		t.Errorf("expected profile key in output, got: %q", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("expected active marker in output, got: %q", out)
	}
}

func TestRunProfileListNoProfiles(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}

	var stdout bytes.Buffer
	if err := runProfileList(&stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "No profiles configured yet") {
		t.Errorf("expected no profiles message, got: %q", stdout.String())
	}
}

func TestRunProfileCommandDispatch(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	var stdout bytes.Buffer
	cfg := cli.Config{DefaultSubcommand: cli.ProfileList}
	if err := runProfileCommand(cfg, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "gcp:default") {
		t.Errorf("expected list output, got: %q", stdout.String())
	}
}

func TestRunProfileCommandUnsupportedSubcommand(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	cfg := cli.Config{DefaultSubcommand: "unknown"}
	err := runProfileCommand(cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unsupported profile subcommand") {
		t.Fatalf("expected unsupported subcommand error, got: %v", err)
	}
}

func TestRunProfileCreateSuccess(t *testing.T) {
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
				return nil, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	cfg := cli.Config{Provider: "gcp", ProfileName: "work", APIKey: "test-key"}
	if err := runProfileCreate(cfg, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Profile created: gcp:work") {
		t.Errorf("expected creation message, got: %q", stdout.String())
	}
}

func TestRunProfileCreateNormalizesProviderName(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		if profile.Provider != "gcp" {
			t.Fatalf("expected normalized provider gcp, got %q", profile.Provider)
		}
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, lang string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-US-Neural2-F"}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, lang, voice, enc string) ([]byte, error) {
				return nil, nil
			},
		}, nil
	}

	var saved config.Config
	saveConfig = func(cfg config.Config) error {
		saved = cfg
		return nil
	}

	var stdout bytes.Buffer
	cfg := cli.Config{Provider: " GCP ", ProfileName: "work", APIKey: "test-key"}
	if err := runProfileCreate(cfg, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := saved.Profiles["gcp:work"]; !exists {
		t.Fatalf("expected normalized profile key gcp:work in saved config, got: %#v", saved.Profiles)
	}
	if !strings.Contains(stdout.String(), "Profile created: gcp:work") {
		t.Errorf("expected normalized creation message, got: %q", stdout.String())
	}
}

func TestRunProfileCreateAlreadyExists(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	// stubAppDeps has gcp:default — try to create it again.
	cfg := cli.Config{Provider: "gcp", ProfileName: "default", APIKey: "test-key"}
	err := runProfileCreate(cfg, &bytes.Buffer{})
	want := `profile "gcp:default" already exists. Choose a different name or delete it first`
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestRunProfileCreateMissingProvider(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	cfg := cli.Config{ProfileName: "work", APIKey: "test-key"}
	err := runProfileCreate(cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--provider is required") {
		t.Fatalf("expected provider required error, got: %v", err)
	}
}

func TestRunProfileCreateMissingName(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	cfg := cli.Config{Provider: "gcp", APIKey: "test-key"}
	err := runProfileCreate(cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--name is required") {
		t.Fatalf("expected name required error, got: %v", err)
	}
}

func TestRunProfileCreateRejectsInvalidProvider(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	cfg := cli.Config{Provider: "gc:p", ProfileName: "work", APIKey: "test-key"}
	err := runProfileCreate(cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "invalid provider") {
		t.Fatalf("expected invalid provider error, got: %v", err)
	}
}

func TestRunProfileCreateRejectsInvalidProfileName(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	cfg := cli.Config{Provider: "gcp", ProfileName: "wo:rk", APIKey: "test-key"}
	err := runProfileCreate(cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "invalid profile name") {
		t.Fatalf("expected invalid profile name error, got: %v", err)
	}
}

func TestRunProfileCreateRejectsWhitespaceOnlyName(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	cfg := cli.Config{Provider: "gcp", ProfileName: "   ", APIKey: "test-key"}
	err := runProfileCreate(cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--name is required") {
		t.Fatalf("expected name required error, got: %v", err)
	}
}

func TestRunProfileCreateMissingAPIKey(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	cfg := cli.Config{Provider: "gcp", ProfileName: "work"}
	err := runProfileCreate(cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--api-key is required") {
		t.Fatalf("expected api-key required error, got: %v", err)
	}
}

func TestRunProfileDeleteSuccess(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	var saved config.Config
	saveConfig = func(cfg config.Config) error {
		saved = cfg
		return nil
	}

	var stdout bytes.Buffer
	if err := runProfileDelete(cli.Config{Profile: "gcp:default"}, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := saved.Profiles["gcp:default"]; exists {
		t.Error("expected profile to be removed from saved config")
	}
	if !strings.Contains(stdout.String(), "Profile deleted: gcp:default") {
		t.Errorf("expected deletion message, got: %q", stdout.String())
	}
}

func TestRunProfileDeleteNotFound(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	err := runProfileDelete(cli.Config{Profile: "gcp:nonexistent"}, &bytes.Buffer{})
	want := `profile "gcp:nonexistent" not found. Run 'ttscli profile list' to see available profiles`
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestRunProfileDeleteActiveProfileCleared(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	// Only one profile, active.
	loadConfig = func() (config.Config, error) {
		return config.Config{
			ActiveProvider: "gcp",
			ActiveProfile:  "default",
			Profiles: map[string]config.Profile{
				"gcp:default": {Provider: "gcp", Name: "default"},
			},
		}, nil
	}
	var saved config.Config
	saveConfig = func(cfg config.Config) error {
		saved = cfg
		return nil
	}

	var stdout bytes.Buffer
	if err := runProfileDelete(cli.Config{Profile: "gcp:default"}, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.ActiveProvider != "" || saved.ActiveProfile != "" {
		t.Errorf("expected active profile to be cleared, got %s:%s", saved.ActiveProvider, saved.ActiveProfile)
	}
}

func TestRunProfileDeleteActiveSwitchesToRemaining(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	// Two profiles; the active one is deleted. Post-delete only gcp:work remains,
	// so the map iteration target is deterministic.
	loadConfig = func() (config.Config, error) {
		return config.Config{
			ActiveProvider: "gcp",
			ActiveProfile:  "default",
			Profiles: map[string]config.Profile{
				"gcp:default": {Provider: "gcp", Name: "default"},
				"gcp:work":    {Provider: "gcp", Name: "work"},
			},
		}, nil
	}
	var saved config.Config
	saveConfig = func(cfg config.Config) error {
		saved = cfg
		return nil
	}

	var stdout bytes.Buffer
	if err := runProfileDelete(cli.Config{Profile: "gcp:default"}, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.ActiveProvider != "gcp" || saved.ActiveProfile != "work" {
		t.Errorf("expected active profile switched to gcp:work, got %s:%s", saved.ActiveProvider, saved.ActiveProfile)
	}
	if _, exists := saved.Profiles["gcp:default"]; exists {
		t.Error("expected gcp:default to be removed from saved config")
	}
	if _, exists := saved.Profiles["gcp:work"]; !exists {
		t.Error("expected gcp:work to remain in saved config")
	}
	if !strings.Contains(stdout.String(), "Switched active profile to: gcp:work") {
		t.Errorf("expected switch message, got: %q", stdout.String())
	}
}

func TestRunProfileUseSuccess(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	loadConfig = func() (config.Config, error) {
		return config.Config{
			Profiles: map[string]config.Profile{
				"gcp:default": {Provider: "gcp", Name: "default"},
				"gcp:work":    {Provider: "gcp", Name: "work"},
			},
		}, nil
	}
	var saved config.Config
	saveConfig = func(cfg config.Config) error {
		saved = cfg
		return nil
	}

	var stdout bytes.Buffer
	if err := runProfileUse(cli.Config{Profile: "gcp:work"}, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.ActiveProvider != "gcp" || saved.ActiveProfile != "work" {
		t.Errorf("expected active profile gcp:work, got %s:%s", saved.ActiveProvider, saved.ActiveProfile)
	}
	if !strings.Contains(stdout.String(), "Active profile set to: gcp:work") {
		t.Errorf("expected confirmation message, got: %q", stdout.String())
	}
}

func TestRunProfileUseInvalidFormat(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	err := runProfileUse(cli.Config{Profile: "invalid-format"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "invalid profile key") {
		t.Fatalf("expected invalid format error, got: %v", err)
	}
}

func TestRunProfileDeleteInvalidFormat(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	err := runProfileDelete(cli.Config{Profile: "gcp"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "invalid profile key") {
		t.Fatalf("expected invalid profile key error, got: %v", err)
	}
}

func TestRunProfileUseNotFound(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	err := runProfileUse(cli.Config{Profile: "gcp:nonexistent"}, &bytes.Buffer{})
	want := `profile "gcp:nonexistent" not found. Run 'ttscli profile list' to see available profiles`
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestRunProfileGetSuccess(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	var stdout bytes.Buffer
	if err := runProfileGet(cli.Config{Profile: "gcp:default"}, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Provider:") || !strings.Contains(out, "Name:") {
		t.Errorf("expected profile details, got: %q", out)
	}
}

func TestRunProfileGetNotFound(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	err := runProfileGet(cli.Config{Profile: "gcp:nonexistent"}, &bytes.Buffer{})
	want := `profile "gcp:nonexistent" not found. Run 'ttscli profile list' to see available profiles`
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestRunProfileGetInvalidFormat(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	err := runProfileGet(cli.Config{Profile: "gcp:"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "invalid profile key") {
		t.Fatalf("expected invalid profile key error, got: %v", err)
	}
}

func TestRunProfileGetMasksAPIKey(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	var stdout bytes.Buffer
	if err := runProfileGet(cli.Config{Profile: "gcp:default"}, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(stdout.String(), "test-api-key") {
		t.Error("expected API key to be masked in output")
	}
}
