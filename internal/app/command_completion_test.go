package app

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/ppikrorngarn/ttscli/internal/cli"
)

func TestRunCompletionBash(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeCompletion, CompletionShell: "bash"}, nil
	}

	var stdout bytes.Buffer
	if err := Run([]string{"completion", "bash"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "complete -F _ttscli_completion ttscli") ||
		!strings.Contains(out, "speak save voices setup") ||
		!strings.Contains(out, "--text -t --lang -l --voice -v --help") ||
		!strings.Contains(out, "--voice -v --lang -l --api-key -k") {
		t.Fatalf("unexpected completion output: %q", out)
	}
}

func TestRunCompletionZsh(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeCompletion, CompletionShell: "zsh"}, nil
	}

	var stdout bytes.Buffer
	if err := Run([]string{"completion", "zsh"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "#compdef ttscli") ||
		!strings.Contains(out, "completion:Generate shell completions") ||
		!strings.Contains(out, "'-t[Text to convert to speech]:text:'") ||
		!strings.Contains(out, "_values 'flags' --voice -v --lang -l --api-key -k") {
		t.Fatalf("unexpected completion output: %q", out)
	}
}

func TestRunCompletionFish(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeCompletion, CompletionShell: "fish"}, nil
	}

	var stdout bytes.Buffer
	if err := Run([]string{"completion", "fish"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "complete -c ttscli") ||
		!strings.Contains(out, "-a completion") ||
		!strings.Contains(out, "-l text -s t") ||
		!strings.Contains(out, "-l api-key -s k") {
		t.Fatalf("unexpected completion output: %q", out)
	}
}

func TestRunCompletionUnsupportedShell(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeCompletion, CompletionShell: "invalid"}, nil
	}

	err := Run([]string{"completion", "invalid"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("expected unsupported shell error, got: %v", err)
	}
}
