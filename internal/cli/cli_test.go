package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestParseCLIArgsVoices(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"voices", "--lang", "en-GB"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Mode != ModeVoices || !cfg.ListVoices {
		t.Fatalf("expected voices mode config, got %+v", cfg)
	}
	if cfg.Lang != "en-GB" {
		t.Fatalf("expected lang en-GB, got %q", cfg.Lang)
	}
	if !cfg.HasLangFlag {
		t.Fatalf("expected HasLangFlag=true")
	}
}

func TestParseCLIArgsSpeakMissingText(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"speak"}, &stderr)
	if err == nil {
		t.Fatal("expected error for missing text")
	}
	if !strings.Contains(err.Error(), "please provide text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsNoArgsHintsHelp(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{}, &stderr)
	if err == nil {
		t.Fatal("expected error for no command")
	}
	if !strings.Contains(err.Error(), "ttscli --help") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsValidSpeak(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"speak", "--text", "hello"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Mode != ModeSpeak {
		t.Fatalf("expected mode %q, got %+v", ModeSpeak, cfg)
	}
	if cfg.Text != "hello" {
		t.Fatalf("expected text hello, got %q", cfg.Text)
	}
	if cfg.SavePath != "" {
		t.Fatalf("expected empty SavePath for speak, got %q", cfg.SavePath)
	}
	if !cfg.Play {
		t.Fatalf("expected Play=true for speak, got %+v", cfg)
	}
	if cfg.Lang != DefaultLanguage || cfg.Voice != DefaultVoice {
		t.Fatalf("unexpected defaults: lang=%q voice=%q", cfg.Lang, cfg.Voice)
	}
}

func TestParseCLIArgsSpeakTracksVoiceAndLangFlags(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"speak", "--text", "hello", "--voice", "en-US-Chirp3-HD-Achernar", "--lang", "en-US"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if !cfg.HasVoiceFlag || !cfg.HasLangFlag {
		t.Fatalf("expected voice/lang flags to be marked as set, got %+v", cfg)
	}
}

func TestParseCLIArgsSaveMissingText(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"save", "--out", "out.mp3"}, &stderr)
	if err == nil {
		t.Fatal("expected error for missing text")
	}
	if !strings.Contains(err.Error(), "please provide text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsSaveMissingOut(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"save", "--text", "hello"}, &stderr)
	if err == nil {
		t.Fatal("expected error for missing output path")
	}
	if !strings.Contains(err.Error(), "--out") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsValidSave(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"save", "--text", "hello", "--out", "out.mp3"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Mode != ModeSave {
		t.Fatalf("expected mode %q, got %+v", ModeSave, cfg)
	}
	if cfg.Text != "hello" || cfg.SavePath != "out.mp3" {
		t.Fatalf("unexpected save config: %+v", cfg)
	}
	if cfg.Play {
		t.Fatalf("expected Play=false for save, got %+v", cfg)
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
	helpText := stderr.String()
	if !strings.Contains(helpText, "ttscli speak --text") ||
		!strings.Contains(helpText, "ttscli save --text") ||
		!strings.Contains(helpText, "ttscli voices --lang") ||
		!strings.Contains(helpText, "ttscli setup") ||
		!strings.Contains(helpText, "ttscli default <set|get|unset> [flags]") {
		t.Fatalf("help text missing usage examples, got: %q", helpText)
	}
}

func TestParseCLIArgsSpeakUnexpectedPositionalArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"speak", "--text", "hello", "extra"}, &stderr)
	if err == nil {
		t.Fatal("expected unexpected positional arguments error")
	}
	if !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsSpeakRejectsLegacyOutputFlags(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"speak", "--text", "hello", "--play"}, &stderr)
	if err == nil {
		t.Fatal("expected unknown flag error for --play")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsUnsupportedCommand(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"play"}, &stderr)
	if err == nil {
		t.Fatal("expected unsupported command error")
	}
	if !strings.Contains(err.Error(), "unsupported command") {
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

func TestParseCLIArgsDoctor(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"doctor"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Mode != ModeDoctor {
		t.Fatalf("expected mode %q, got %+v", ModeDoctor, cfg)
	}
}

func TestParseCLIArgsDoctorRejectsPositionalArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"doctor", "extra"}, &stderr)
	if err == nil {
		t.Fatal("expected unexpected positional arguments error")
	}
	if !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsCompletion(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"completion", "zsh"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Mode != ModeCompletion || cfg.CompletionShell != "zsh" {
		t.Fatalf("unexpected completion config: %+v", cfg)
	}
}

func TestParseCLIArgsCompletionRequiresShell(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"completion"}, &stderr)
	if err == nil {
		t.Fatal("expected missing shell error")
	}
	if !strings.Contains(err.Error(), "please provide a shell") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsCompletionUnsupportedShell(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"completion", "powershell"}, &stderr)
	if err == nil {
		t.Fatal("expected unsupported shell error")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsCompletionRejectsExtraArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"completion", "zsh", "extra"}, &stderr)
	if err == nil {
		t.Fatal("expected unexpected positional arguments error")
	}
	if !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}
