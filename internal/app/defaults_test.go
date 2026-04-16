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

func TestResolveProfileAPIKey(t *testing.T) {
	p := config.Profile{Credentials: map[string]interface{}{"apiKey": "  mykey  "}}
	if got := resolveProfileAPIKey(p); got != "mykey" {
		t.Errorf("expected mykey, got %q", got)
	}

	p2 := config.Profile{Credentials: map[string]interface{}{}}
	if got := resolveProfileAPIKey(p2); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}

	p3 := config.Profile{Credentials: map[string]interface{}{"apiKey": 123}}
	if got := resolveProfileAPIKey(p3); got != "" {
		t.Errorf("expected empty string for wrong type, got %q", got)
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
	cfg := resolveProfileDefaults(cli.Config{}, profile)
	if cfg.Voice != "en-US-Neural2-F" || cfg.Lang != "en-US" {
		t.Errorf("expected profile defaults, got voice=%q lang=%q", cfg.Voice, cfg.Lang)
	}

	// Voice flag set — voice kept, lang from profile.
	cfg2 := resolveProfileDefaults(cli.Config{Voice: "custom", HasVoiceFlag: true}, profile)
	if cfg2.Voice != "custom" {
		t.Errorf("expected custom voice to be kept, got %q", cfg2.Voice)
	}
	if cfg2.Lang != "en-US" {
		t.Errorf("expected lang from profile, got %q", cfg2.Lang)
	}

	// Lang flag set — lang kept, voice from profile.
	cfg3 := resolveProfileDefaults(cli.Config{Lang: "en-GB", HasLangFlag: true}, profile)
	if cfg3.Lang != "en-GB" {
		t.Errorf("expected en-GB to be kept, got %q", cfg3.Lang)
	}
	if cfg3.Voice != "en-US-Neural2-F" {
		t.Errorf("expected voice from profile, got %q", cfg3.Voice)
	}

	// Both flags set — both kept.
	cfg4 := resolveProfileDefaults(cli.Config{Voice: "custom", Lang: "fr-FR", HasVoiceFlag: true, HasLangFlag: true}, profile)
	if cfg4.Voice != "custom" || cfg4.Lang != "fr-FR" {
		t.Errorf("expected flags to take precedence, got voice=%q lang=%q", cfg4.Voice, cfg4.Lang)
	}
}

func TestResolveProfileDefaultsEmptyProfileDefaults(t *testing.T) {
	profile := config.Profile{Defaults: map[string]string{}}
	cfg := resolveProfileDefaults(cli.Config{Voice: "v", Lang: "l"}, profile)
	if cfg.Voice != "v" || cfg.Lang != "l" {
		t.Errorf("expected original values kept when profile has no defaults, got voice=%q lang=%q", cfg.Voice, cfg.Lang)
	}
}
