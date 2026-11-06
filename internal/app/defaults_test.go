package app

import (
	"testing"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
)

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "(not set)"},
		{"   ", "(not set)"},
		{"abc", "****"},
		{"abcd", "****"},
		{"abcde", "****bcde"},
		{"AIzaSyABC123", "****C123"},
	}
	for _, tt := range tests {
		got := maskAPIKey(tt.input)
		if got != tt.want {
			t.Errorf("maskAPIKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCredentialString(t *testing.T) {
	p := config.Profile{Credentials: map[string]interface{}{"apiKey": "  mykey  "}}
	if got := credentialString(p, "apiKey"); got != "mykey" {
		t.Errorf("expected mykey, got %q", got)
	}
	if got := credentialString(config.Profile{}, "apiKey"); got != "" {
		t.Errorf("expected empty string for nil credentials, got %q", got)
	}
	p3 := config.Profile{Credentials: map[string]interface{}{"apiKey": 123}}
	if got := credentialString(p3, "apiKey"); got != "" {
		t.Errorf("expected empty string for wrong type, got %q", got)
	}
}

func TestCredentialLabel(t *testing.T) {
	for key, want := range map[string]string{
		"apiKey":          "API Key",
		"region":          "Region",
		"subscriptionKey": "Subscription Key",
		"token":           "Token",
	} {
		if got := credentialLabel(key); got != want {
			t.Errorf("credentialLabel(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestCredentialLinesKnownProviderMasksSecrets(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	p := config.Profile{Provider: "gcp", Credentials: map[string]interface{}{"apiKey": "secret-1234"}}
	lines := credentialLines(p)
	if len(lines) != 1 || lines[0].label != "API Key" || lines[0].value != "****1234" {
		t.Fatalf("unexpected lines: %+v", lines)
	}
}

func TestCredentialLinesSecretAndPlainFields(t *testing.T) {
	reset := stubAppDeps()
	defer reset()
	stubFakeProvider()

	p := config.Profile{Provider: "fake", Credentials: map[string]interface{}{"token": "tok-9999", "region": "eastus"}}
	lines := credentialLines(p)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %+v", lines)
	}
	if lines[0].label != "Token" || lines[0].value != "****9999" {
		t.Errorf("secret field not masked: %+v", lines[0])
	}
	if lines[1].label != "Region" || lines[1].value != "eastus" {
		t.Errorf("plain field should be shown: %+v", lines[1])
	}

	missing := credentialLines(config.Profile{Provider: "fake", Credentials: map[string]interface{}{"token": "tok-9999"}})
	if missing[1].value != "(not set)" {
		t.Errorf("missing plain field should read (not set), got %+v", missing[1])
	}
}

func TestCredentialLinesUnknownProviderMasksEverything(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	p := config.Profile{Provider: "mystery", Credentials: map[string]interface{}{"zeta": "zz-1111", "alpha": "aa-2222"}}
	lines := credentialLines(p)
	if len(lines) != 2 || lines[0].label != "Alpha" || lines[0].value != "****2222" || lines[1].label != "Zeta" || lines[1].value != "****1111" {
		t.Fatalf("expected sorted, fully masked lines, got %+v", lines)
	}
}

func TestResolveProfileDefaults(t *testing.T) {
	profile := config.Profile{
		Defaults: map[string]string{
			"voice": "en-US-Neural2-F",
			"lang":  "en-US",
		},
	}

	// Neither flag set — profile defaults applied.
	cfg := resolveProfileDefaults(cli.Config{}, profile, nil)
	if cfg.Voice != "en-US-Neural2-F" || cfg.Lang != "en-US" {
		t.Errorf("expected profile defaults, got voice=%q lang=%q", cfg.Voice, cfg.Lang)
	}

	// Voice flag set — voice kept, lang from profile.
	cfg2 := resolveProfileDefaults(cli.Config{Voice: "custom", HasVoiceFlag: true}, profile, nil)
	if cfg2.Voice != "custom" {
		t.Errorf("expected custom voice to be kept, got %q", cfg2.Voice)
	}
	if cfg2.Lang != "en-US" {
		t.Errorf("expected lang from profile, got %q", cfg2.Lang)
	}

	// Lang flag set — lang kept, voice from profile.
	cfg3 := resolveProfileDefaults(cli.Config{Lang: "en-GB", HasLangFlag: true}, profile, nil)
	if cfg3.Lang != "en-GB" {
		t.Errorf("expected en-GB to be kept, got %q", cfg3.Lang)
	}
	if cfg3.Voice != "en-US-Neural2-F" {
		t.Errorf("expected voice from profile, got %q", cfg3.Voice)
	}

	// Both flags set — both kept.
	cfg4 := resolveProfileDefaults(cli.Config{Voice: "custom", Lang: "fr-FR", HasVoiceFlag: true, HasLangFlag: true}, profile, nil)
	if cfg4.Voice != "custom" || cfg4.Lang != "fr-FR" {
		t.Errorf("expected flags to take precedence, got voice=%q lang=%q", cfg4.Voice, cfg4.Lang)
	}
}

func TestResolveProfileDefaultsEmptyProfileDefaults(t *testing.T) {
	profile := config.Profile{Defaults: map[string]string{}}
	cfg := resolveProfileDefaults(cli.Config{Voice: "v", Lang: "l"}, profile, nil)
	if cfg.Voice != "v" || cfg.Lang != "l" {
		t.Errorf("expected original values kept when profile has no defaults, got voice=%q lang=%q", cfg.Voice, cfg.Lang)
	}
}

func TestResolveProfileDefaultsFallsBackToProviderVoice(t *testing.T) {
	provider := &fakeTTSClient{} // DefaultVoice: en-US -> en-US-Neural2-F, else ""
	profile := config.Profile{Defaults: map[string]string{"lang": "en-US"}}

	// No voice anywhere: provider default for the resolved language.
	cfg := resolveProfileDefaults(cli.Config{Lang: "en-US"}, profile, provider)
	if cfg.Voice != "en-US-Neural2-F" {
		t.Errorf("expected provider default voice, got %q", cfg.Voice)
	}

	// Provider has no default for the language: voice stays empty so the
	// provider API can pick one.
	cfg2 := resolveProfileDefaults(cli.Config{Lang: "fr-FR", HasLangFlag: true}, profile, provider)
	if cfg2.Voice != "" {
		t.Errorf("expected empty voice for fr-FR, got %q", cfg2.Voice)
	}

	// Profile voice wins over the provider default.
	withVoice := config.Profile{Defaults: map[string]string{"lang": "en-US", "voice": "en-US-Chirp3-HD-Kore"}}
	cfg3 := resolveProfileDefaults(cli.Config{Lang: "en-US"}, withVoice, provider)
	if cfg3.Voice != "en-US-Chirp3-HD-Kore" {
		t.Errorf("expected profile voice to win, got %q", cfg3.Voice)
	}

	// Explicit flag wins over everything.
	cfg4 := resolveProfileDefaults(cli.Config{Lang: "en-US", Voice: "custom", HasVoiceFlag: true}, withVoice, provider)
	if cfg4.Voice != "custom" {
		t.Errorf("expected flag voice to win, got %q", cfg4.Voice)
	}
}
