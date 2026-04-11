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
	if cfg.Lang != defaultLanguage || cfg.Voice != defaultVoice {
		t.Fatalf("unexpected defaults: lang=%q voice=%q", cfg.Lang, cfg.Voice)
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
