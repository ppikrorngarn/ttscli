package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

const (
	appName           = "ttscli"
	defaultLanguage   = "en-US"
	defaultVoice      = "en-US-Neural2-F"
	helpTitle         = "GCP Text-to-Speech CLI"
	helpExampleSpeak  = `  ttscli --text "Hello world" --play`
	helpExampleVoices = "  ttscli --list-voices --lang en-GB"
)

type Config struct {
	Text       string
	SavePath   string
	Play       bool
	Lang       string
	Voice      string
	ListVoices bool
}

func ParseArgs(args []string, stderr io.Writer) (Config, error) {
	cfg := Config{}
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.StringVar(&cfg.Text, "text", "", "Text to convert to speech")
	fs.StringVar(&cfg.SavePath, "save", "", "Path to save the output MP3 file (e.g., output.mp3)")
	fs.BoolVar(&cfg.Play, "play", false, "Play the audio immediately")
	fs.StringVar(&cfg.Lang, "lang", defaultLanguage, "Language code")
	fs.StringVar(&cfg.Voice, "voice", defaultVoice, "Voice name")
	fs.BoolVar(&cfg.ListVoices, "list-voices", false, "List available voices (filtered by --lang)")

	fs.Usage = func() {
		fmt.Fprintln(stderr, helpTitle)
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Usage:")
		fs.PrintDefaults()
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Examples:")
		fmt.Fprintln(stderr, helpExampleSpeak)
		fmt.Fprintln(stderr, helpExampleVoices)
	}

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	if fs.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	if cfg.ListVoices {
		return cfg, nil
	}

	if cfg.Text == "" {
		return cfg, fmt.Errorf("please provide text using the --text flag")
	}

	if cfg.SavePath == "" && !cfg.Play {
		return cfg, fmt.Errorf("please specify either --save <path> or --play (or both)")
	}

	return cfg, nil
}
