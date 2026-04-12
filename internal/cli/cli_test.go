package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestParseCLIArgsListVoices(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"--list-voices", "--lang", "en-GB"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if !cfg.ListVoices {
		t.Fatalf("expected ListVoices=true")
	}
	if cfg.Lang != "en-GB" {
		t.Fatalf("expected lang en-GB, got %q", cfg.Lang)
	}
	if !cfg.HasLangFlag {
		t.Fatalf("expected HasLangFlag=true")
	}
}

func TestParseCLIArgsMissingText(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"--play"}, &stderr)
	if err == nil {
		t.Fatal("expected error for missing text")
	}
	if !strings.Contains(err.Error(), "please provide text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsMissingOutputMode(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"--text", "hello"}, &stderr)
	if err == nil {
		t.Fatal("expected error for missing output mode")
	}
	if !strings.Contains(err.Error(), "please specify either --save") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsValidSynthesize(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"--text", "hello", "--save", "out.mp3"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Text != "hello" {
		t.Fatalf("expected text hello, got %q", cfg.Text)
	}
	if cfg.SavePath != "out.mp3" {
		t.Fatalf("expected SavePath out.mp3, got %q", cfg.SavePath)
	}
	if cfg.Lang != DefaultLanguage || cfg.Voice != DefaultVoice {
		t.Fatalf("unexpected defaults: lang=%q voice=%q", cfg.Lang, cfg.Voice)
	}
}

func TestParseCLIArgsTracksVoiceAndLangFlags(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"--text", "hello", "--save", "out.mp3", "--voice", "en-US-Chirp3-HD-Achernar", "--lang", "en-US"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if !cfg.HasVoiceFlag || !cfg.HasLangFlag {
		t.Fatalf("expected voice/lang flags to be marked as set, got %+v", cfg)
	}
}

func TestParseCLIArgsHelp(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"--help"}, &stderr)
	if err == nil {
		t.Fatal("expected help error")
	}
	if err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestParseCLIArgsUnexpectedPositionalArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"--text", "hello", "--play", "extra"}, &stderr)
	if err == nil {
		t.Fatal("expected unexpected positional arguments error")
	}
	if !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsDefaultSetVoiceOnly(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"default", "set", "--voice", "en-US-Chirp3-HD-Achernar"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Mode != "default" || cfg.DefaultSubcommand != "set" {
		t.Fatalf("unexpected default command config: %+v", cfg)
	}
	if !cfg.HasVoiceFlag || cfg.HasLangFlag {
		t.Fatalf("expected voice-only flags, got %+v", cfg)
	}
}

func TestParseCLIArgsDefaultSetAPIKeyOnly(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"default", "set", "--api-key", "k-12345"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if !cfg.HasAPIKeyFlag || cfg.APIKey != "k-12345" {
		t.Fatalf("expected api key flag/value to be parsed, got %+v", cfg)
	}
}

func TestParseCLIArgsDefaultSetRequiresAtLeastOneFlag(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"default", "set"}, &stderr)
	if err == nil {
		t.Fatal("expected error when no default set flags are provided")
	}
	if !strings.Contains(err.Error(), "please provide --voice, --lang, and/or --api-key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsDefaultGetAndUnset(t *testing.T) {
	var stderr bytes.Buffer

	getCfg, err := ParseArgs([]string{"default", "get"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs(default get) returned error: %v", err)
	}
	if getCfg.DefaultSubcommand != "get" {
		t.Fatalf("expected get subcommand, got %+v", getCfg)
	}

	unsetCfg, err := ParseArgs([]string{"default", "unset"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs(default unset) returned error: %v", err)
	}
	if unsetCfg.DefaultSubcommand != "unset" {
		t.Fatalf("expected unset subcommand, got %+v", unsetCfg)
	}
}

func TestParseCLIArgsDefaultUnknownSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"default", "whoami"}, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown default subcommand")
	}
	if !strings.Contains(err.Error(), "unsupported default subcommand") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsDefaultUnsetSelectors(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"default", "unset", "--voice", "--api-key"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if !cfg.HasVoiceFlag || !cfg.HasAPIKeyFlag || cfg.HasLangFlag {
		t.Fatalf("unexpected unset selectors: %+v", cfg)
	}
}

func TestParseCLIArgsSetup(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"setup"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Mode != ModeSetup {
		t.Fatalf("expected mode %q, got %+v", ModeSetup, cfg)
	}
}

func TestParseCLIArgsSetupRejectsPositionalArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"setup", "extra"}, &stderr)
	if err == nil {
		t.Fatal("expected unexpected positional arguments error")
	}
	if !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}
