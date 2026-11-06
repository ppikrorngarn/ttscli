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

// These tests register a fake second provider and drive setup and the profile
// commands through it. Nothing here mentions gcp-specific prompts or fields,
// which is the point: adding a provider must not require touching app code.

func TestRunSetupCommandPromptsForProviderWhenSeveral(t *testing.T) {
	reset := stubAppDeps()
	defer reset()
	stubFakeProvider()

	var saved config.Config
	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	saveConfig = func(cfg config.Config) error { saved = cfg; return nil }
	var builtFrom config.Profile
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		builtFrom = profile
		return fakeSpec.New(nil, nil)
	}
	readPassword = func() ([]byte, error) { return []byte("tok-1234"), nil }
	// provider, region, default lang (enter), default voice (enter), no sound check
	setupInput = strings.NewReader("fake\neastus\n\n\nn\n")

	var stdout bytes.Buffer
	if err := runSetupCommand(&stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{
		"Available providers: fake, gcp",
		"Provider [gcp]:",
		"create a Fake Speech profile",
		"Enter your Fake Speech token:",
		"Fake Speech region:",
		"built-in defaults (xx-XX / xx-XX-Fake-1)",
		"Created profile: fake:default",
		"Token:     ****1234",
		"Region:    eastus",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "tok-1234") {
		t.Error("secret credential leaked into setup output")
	}
	if builtFrom.Provider != "fake" || builtFrom.Credentials["token"] != "tok-1234" || builtFrom.Credentials["region"] != "eastus" {
		t.Errorf("provider built from unexpected profile: %+v", builtFrom)
	}
	if saved.ActiveProvider != "fake" || saved.ActiveProfile != "default" {
		t.Errorf("expected fake:default to become active, got %s:%s", saved.ActiveProvider, saved.ActiveProfile)
	}
	if got := saved.Profiles["fake:default"].Defaults["voice"]; got != "xx-XX-Fake-1" {
		t.Errorf("expected provider default voice, got %q", got)
	}
}

func TestRunSetupCommandProviderDefaultsToGCPOnEnter(t *testing.T) {
	reset := stubAppDeps()
	defer reset()
	stubFakeProvider()

	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		return &fakeTTSClient{
			listVoicesFn: func(context.Context, string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-US-Neural2-F"}}, nil
			},
		}, nil
	}
	readPassword = func() ([]byte, error) { return []byte("my-api-key"), nil }
	// provider (enter = gcp), default lang, default voice, no sound check
	setupInput = strings.NewReader("\n\n\nn\n")

	var stdout bytes.Buffer
	if err := runSetupCommand(&stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Created profile: gcp:default") {
		t.Errorf("expected gcp profile, got: %s", stdout.String())
	}
}

func TestRunSetupCommandRejectsUnknownProvider(t *testing.T) {
	reset := stubAppDeps()
	defer reset()
	stubFakeProvider()

	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	setupInput = strings.NewReader("openai\n")

	err := runSetupCommand(&bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), `unsupported provider "openai"`) {
		t.Fatalf("expected unsupported provider error, got: %v", err)
	}
}

func TestRunSetupCommandEmptyPlainCredential(t *testing.T) {
	reset := stubAppDeps()
	defer reset()
	stubFakeProvider()

	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	readPassword = func() ([]byte, error) { return []byte("tok-1234"), nil }
	setupInput = strings.NewReader("fake\n\n") // empty region

	err := runSetupCommand(&bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "Region is required") || !strings.Contains(err.Error(), "Pick a region") {
		t.Fatalf("expected region required error with help, got: %v", err)
	}
}

func TestRunProfileCreateSecondProviderWithCredentialFlags(t *testing.T) {
	reset := stubAppDeps()
	defer reset()
	stubFakeProvider()

	var saved config.Config
	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	saveConfig = func(cfg config.Config) error { saved = cfg; return nil }
	var builtFrom config.Profile
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		builtFrom = profile
		return fakeSpec.New(nil, nil)
	}

	var stdout bytes.Buffer
	cfg := cli.Config{Provider: "fake", ProfileName: "work", Credentials: map[string]string{"token": "tok-1234", "region": "eastus"}}
	if err := runProfileCreate(cfg, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Profile created: fake:work") {
		t.Errorf("expected creation message, got: %q", stdout.String())
	}
	if builtFrom.Credentials["token"] != "tok-1234" || builtFrom.Credentials["region"] != "eastus" {
		t.Errorf("credentials not passed through: %+v", builtFrom.Credentials)
	}
	stored := saved.Profiles["fake:work"]
	if stored.Provider != "fake" || stored.Credentials["region"] != "eastus" {
		t.Errorf("stored profile unexpected: %+v", stored)
	}
}

func TestRunProfileCreateMissingDeclaredCredential(t *testing.T) {
	reset := stubAppDeps()
	defer reset()
	stubFakeProvider()

	cfg := cli.Config{Provider: "fake", ProfileName: "work", Credentials: map[string]string{"token": "tok-1234"}}
	err := runProfileCreate(cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--credential region=VALUE is required") || !strings.Contains(err.Error(), "Pick a region") {
		t.Fatalf("expected missing region error with help, got: %v", err)
	}
}

func TestRunProfileCreateAPIKeyFlagConflict(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	cfg := cli.Config{Provider: "gcp", ProfileName: "work", APIKey: "a", Credentials: map[string]string{"apiKey": "b"}}
	err := runProfileCreate(cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "both as --api-key and --credential") {
		t.Fatalf("expected conflict error, got: %v", err)
	}
}

func TestRunProfileCreateAPIKeyViaCredentialFlag(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	var builtFrom config.Profile
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		builtFrom = profile
		return &fakeTTSClient{
			listVoicesFn: func(context.Context, string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-US-Neural2-F"}}, nil
			},
		}, nil
	}

	cfg := cli.Config{Provider: "gcp", ProfileName: "work", Credentials: map[string]string{"apiKey": "via-flag"}}
	if err := runProfileCreate(cfg, &bytes.Buffer{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if builtFrom.Credentials["apiKey"] != "via-flag" {
		t.Errorf("expected apiKey from --credential, got %+v", builtFrom.Credentials)
	}
}

func TestRunProfileGetSecondProviderMasksOnlySecrets(t *testing.T) {
	reset := stubAppDeps()
	defer reset()
	stubFakeProvider()

	loadConfig = func() (config.Config, error) {
		return config.Config{
			ActiveProvider: "fake",
			ActiveProfile:  "work",
			Profiles: map[string]config.Profile{
				"fake:work": {
					Provider:    "fake",
					Name:        "work",
					Credentials: map[string]interface{}{"token": "tok-1234", "region": "eastus"},
					Defaults:    map[string]string{"lang": "xx-XX"},
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	if err := runProfileGet(cli.Config{Profile: "fake:work"}, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Token:         ****1234") {
		t.Errorf("expected masked token line, got:\n%s", out)
	}
	if !strings.Contains(out, "Region:        eastus") {
		t.Errorf("expected plain region line, got:\n%s", out)
	}
	if strings.Contains(out, "tok-1234") {
		t.Error("secret leaked into profile get output")
	}
}

func TestRunDoctorCommandUsesProviderDefaultLang(t *testing.T) {
	reset := stubAppDeps()
	defer reset()
	stubFakeProvider()

	loadConfig = func() (config.Config, error) {
		return config.Config{
			ActiveProvider: "fake",
			ActiveProfile:  "work",
			Profiles: map[string]config.Profile{
				"fake:work": {Provider: "fake", Name: "work", Credentials: map[string]interface{}{"token": "t", "region": "r"}},
			},
		}, nil
	}
	var askedLang string
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		return &fakeTTSClient{
			listVoicesFn: func(_ context.Context, lang string) ([]tts.Voice, error) {
				askedLang = lang
				return []tts.Voice{{Name: "xx-XX-Fake-1"}}, nil
			},
		}, nil
	}

	_ = runDoctorCommand(&bytes.Buffer{})
	if askedLang != "xx-XX" {
		t.Errorf("expected doctor to probe with provider default lang xx-XX, got %q", askedLang)
	}
}
