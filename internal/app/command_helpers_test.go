package app

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/ppikrorngarn/ttscli/internal/tts"
)

func TestVoiceExists(t *testing.T) {
	voices := []tts.Voice{{Name: "en-US-Neural2-F"}, {Name: "en-GB-Neural2-B"}}

	if !voiceExists("en-US-Neural2-F", voices) {
		t.Error("expected voice to be found")
	}
	if !voiceExists("EN-US-NEURAL2-F", voices) {
		t.Error("expected case-insensitive match")
	}
	if voiceExists("fr-FR-Neural2-A", voices) {
		t.Error("expected voice to not be found")
	}
	if voiceExists("anything", nil) {
		t.Error("expected false for nil voices slice")
	}
}

func TestPromptLine(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("hello world\n"))
	var stdout bytes.Buffer

	got, err := promptLine(reader, &stdout, "Enter: ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", got)
	}
	if stdout.String() != "Enter: " {
		t.Errorf("expected prompt to be printed, got %q", stdout.String())
	}
}

func TestPromptLineTrimsWhitespace(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("  value  \n"))
	got, err := promptLine(reader, &bytes.Buffer{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "value" {
		t.Errorf("expected trimmed value, got %q", got)
	}
}

func TestPromptYesNoAcceptsYes(t *testing.T) {
	for _, input := range []string{"y\n", "Y\n", "yes\n", "YES\n"} {
		reader := bufio.NewReader(strings.NewReader(input))
		got, err := promptYesNo(reader, &bytes.Buffer{}, "?", false)
		if err != nil || !got {
			t.Errorf("input %q: expected true, got %v (err: %v)", input, got, err)
		}
	}
}

func TestPromptYesNoAcceptsNo(t *testing.T) {
	for _, input := range []string{"n\n", "N\n", "no\n", "NO\n"} {
		reader := bufio.NewReader(strings.NewReader(input))
		got, err := promptYesNo(reader, &bytes.Buffer{}, "?", true)
		if err != nil || got {
			t.Errorf("input %q: expected false, got %v (err: %v)", input, got, err)
		}
	}
}

func TestPromptYesNoDefaultYes(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\n"))
	got, err := promptYesNo(reader, &bytes.Buffer{}, "?", true)
	if err != nil || !got {
		t.Fatalf("expected default true, got %v (err: %v)", got, err)
	}
}

func TestPromptYesNoDefaultNo(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\n"))
	got, err := promptYesNo(reader, &bytes.Buffer{}, "?", false)
	if err != nil || got {
		t.Fatalf("expected default false, got %v (err: %v)", got, err)
	}
}

func TestPromptYesNoInvalidThenValid(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("maybe\nyes\n"))
	var stdout bytes.Buffer

	got, err := promptYesNo(reader, &stdout, "?", false)
	if err != nil || !got {
		t.Fatalf("expected true after retry, got %v (err: %v)", got, err)
	}
	if !strings.Contains(stdout.String(), "Please answer y or n.") {
		t.Error("expected retry message to be printed")
	}
}
