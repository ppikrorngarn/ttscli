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

func TestParseCLIArgsVoicesShorthandLang(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"voices", "-l", "en-GB"}, &stderr)
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
	if !strings.Contains(err.Error(), "--text or -t") {
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

func TestParseCLIArgsSpeakShorthandFlags(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"speak", "-t", "hello", "-v", "en-US-Chirp3-HD-Achernar", "-l", "en-US"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Mode != ModeSpeak {
		t.Fatalf("expected mode %q, got %+v", ModeSpeak, cfg)
	}
	if cfg.Text != "hello" {
		t.Fatalf("expected text hello, got %q", cfg.Text)
	}
	if cfg.Voice != "en-US-Chirp3-HD-Achernar" || cfg.Lang != "en-US" {
		t.Fatalf("unexpected voice/lang values: %+v", cfg)
	}
	if !cfg.HasVoiceFlag || !cfg.HasLangFlag {
		t.Fatalf("expected voice/lang shorthand flags to be marked as set, got %+v", cfg)
	}
	if !cfg.Play {
		t.Fatalf("expected Play=true for speak, got %+v", cfg)
	}
}

func TestParseCLIArgsSaveMissingText(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"save", "--out", "out.mp3"}, &stderr)
	if err == nil {
		t.Fatal("expected error for missing text")
	}
	if !strings.Contains(err.Error(), "--text or -t") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsSaveMissingOut(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"save", "--text", "hello"}, &stderr)
	if err == nil {
		t.Fatal("expected error for missing output path")
	}
	if !strings.Contains(err.Error(), "--out or -o") {
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

func TestParseCLIArgsValidSaveShorthandFlags(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"save", "-t", "hello", "-o", "out.mp3", "-l", "en-US", "-v", "en-US-Chirp3-HD-Achernar"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Mode != ModeSave {
		t.Fatalf("expected mode %q, got %+v", ModeSave, cfg)
	}
	if cfg.Text != "hello" || cfg.SavePath != "out.mp3" {
		t.Fatalf("unexpected save config: %+v", cfg)
	}
	if cfg.Voice != "en-US-Chirp3-HD-Achernar" || cfg.Lang != "en-US" {
		t.Fatalf("unexpected voice/lang values: %+v", cfg)
	}
	if !cfg.HasVoiceFlag || !cfg.HasLangFlag {
		t.Fatalf("expected voice/lang shorthand flags to be marked as set, got %+v", cfg)
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
		!strings.Contains(helpText, "-t, --text") ||
		!strings.Contains(helpText, "Commands:") {
		t.Fatalf("help text missing usage examples, got: %q", helpText)
	}
}

func TestParseCLIArgsSpeakHelpShowsShorthandFlags(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"speak", "--help"}, &stderr)
	if err == nil {
		t.Fatal("expected help error")
	}
	if err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	helpText := stderr.String()
	if !strings.Contains(helpText, "-t string") ||
		!strings.Contains(helpText, "-text string") ||
		!strings.Contains(helpText, "-v string") ||
		!strings.Contains(helpText, "-voice string") {
		t.Fatalf("speak help text missing shorthand flags, got: %q", helpText)
	}
}

func TestParseCLIArgsSpeakUnexpectedPositionalArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"speak", "--text", "hello", "extra"}, &stderr)
	if err == nil {
		t.Fatal("expected unexpected positional arguments error")
	}
	if !strings.Contains(err.Error(), "unexpected arguments") {
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
		t.Fatal("expected unknown command error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsSetupRejectsPositionalArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"setup", "extra"}, &stderr)
	if err == nil {
		t.Fatal("expected unexpected positional arguments error")
	}
	if !strings.Contains(err.Error(), "does not accept arguments") {
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
	if !strings.Contains(err.Error(), "does not accept arguments") {
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
	if !strings.Contains(err.Error(), "please specify a shell") {
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
	if !strings.Contains(err.Error(), "too many arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCLIArgsSpeakWithProfileFlag(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"speak", "--text", "hello", "--profile", "gcp:work"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Profile != "gcp:work" {
		t.Errorf("expected profile gcp:work, got %q", cfg.Profile)
	}
}

func TestParseCLIArgsSpeakWithProfileShorthand(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"speak", "--text", "hello", "-p", "gcp:work"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Profile != "gcp:work" {
		t.Errorf("expected profile gcp:work, got %q", cfg.Profile)
	}
}

func TestParseCLIArgsSaveWithProfileFlag(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"save", "--text", "hello", "--out", "out.mp3", "--profile", "gcp:work"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Profile != "gcp:work" {
		t.Errorf("expected profile gcp:work, got %q", cfg.Profile)
	}
}

func TestParseCLIArgsVoicesWithProfileFlag(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"voices", "--profile", "gcp:work"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Profile != "gcp:work" {
		t.Errorf("expected profile gcp:work, got %q", cfg.Profile)
	}
}

func TestParseCLIArgsProfileNoSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"profile"}, &stderr)
	if err == nil || !strings.Contains(err.Error(), "profile subcommand required") {
		t.Fatalf("expected missing subcommand error, got: %v", err)
	}
}

func TestParseCLIArgsProfileList(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"profile", "list"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Mode != ModeProfile || cfg.DefaultSubcommand != ProfileList {
		t.Errorf("unexpected profile list config: %+v", cfg)
	}
}

func TestParseCLIArgsProfileListRejectsExtraArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"profile", "list", "extra"}, &stderr)
	if err == nil || !strings.Contains(err.Error(), "does not accept arguments") {
		t.Fatalf("expected unexpected positional args error, got: %v", err)
	}
}

func TestParseCLIArgsProfileCreate(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"profile", "create", "--provider", "gcp", "--name", "work", "--api-key", "mykey"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.Mode != ModeProfile || cfg.DefaultSubcommand != ProfileCreate {
		t.Errorf("unexpected mode/subcommand: %+v", cfg)
	}
	if cfg.Provider != "gcp" {
		t.Errorf("expected provider gcp, got %q", cfg.Provider)
	}
	if cfg.ProfileName != "work" {
		t.Errorf("expected name work, got %q", cfg.ProfileName)
	}
	if cfg.APIKey != "mykey" {
		t.Errorf("expected api-key mykey, got %q", cfg.APIKey)
	}
}

func TestParseCLIArgsProfileDelete(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"profile", "delete", "gcp:default"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.DefaultSubcommand != ProfileDelete || cfg.Profile != "gcp:default" {
		t.Errorf("unexpected profile delete config: %+v", cfg)
	}
}

func TestParseCLIArgsProfileDeleteMissingKey(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"profile", "delete"}, &stderr)
	if err == nil || !strings.Contains(err.Error(), "profile key required") {
		t.Fatalf("expected missing key error, got: %v", err)
	}
}

func TestParseCLIArgsProfileUse(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"profile", "use", "gcp:default"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.DefaultSubcommand != ProfileUse || cfg.Profile != "gcp:default" {
		t.Errorf("unexpected profile use config: %+v", cfg)
	}
}

func TestParseCLIArgsProfileUseMissingKey(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"profile", "use"}, &stderr)
	if err == nil || !strings.Contains(err.Error(), "profile key required") {
		t.Fatalf("expected missing key error, got: %v", err)
	}
}

func TestParseCLIArgsProfileGet(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := ParseArgs([]string{"profile", "get", "gcp:default"}, &stderr)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.DefaultSubcommand != ProfileGet || cfg.Profile != "gcp:default" {
		t.Errorf("unexpected profile get config: %+v", cfg)
	}
}

func TestParseCLIArgsProfileGetMissingKey(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"profile", "get"}, &stderr)
	if err == nil || !strings.Contains(err.Error(), "profile key required") {
		t.Fatalf("expected missing key error, got: %v", err)
	}
}

func TestParseCLIArgsProfileUnsupportedSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	_, err := ParseArgs([]string{"profile", "rename"}, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unknown profile subcommand") {
		t.Fatalf("expected unsupported subcommand error, got: %v", err)
	}
}
