package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

const (
	appName               = "ttscli"
	DefaultLanguage       = "en-US"
	DefaultVoice          = "en-US-Neural2-F"
	helpTitle             = "GCP Text-to-Speech CLI"
	helpUsageRun          = `  ttscli --text "Hello world" --play`
	helpUsageList         = "  ttscli --list-voices --lang en-GB"
	helpUsageSetup        = "  ttscli setup"
	helpUsageDoctor       = "  ttscli doctor"
	helpUsageCompletion   = "  ttscli completion <bash|zsh|fish>"
	helpUsageDefault      = "  ttscli default <set|get|unset> [flags]"
	helpExampleSpeak      = `  ttscli --text "Hello world" --play`
	helpExampleVoices     = "  ttscli --list-voices --lang en-GB"
	helpExampleSetup      = "  ttscli setup"
	helpExampleDoctor     = "  ttscli doctor"
	helpExampleCompletion = "  ttscli completion zsh"
	helpExampleDefault    = "  ttscli default set --voice en-US-Chirp3-HD-Achernar --lang en-US"
	ModeRun               = "run"
	ModeDefault           = "default"
	ModeSetup             = "setup"
	ModeDoctor            = "doctor"
	ModeCompletion        = "completion"
	DefaultSet            = "set"
	DefaultGet            = "get"
	DefaultUnset          = "unset"
)

var supportedCompletionShells = map[string]struct{}{
	"bash": {},
	"zsh":  {},
	"fish": {},
}

type Config struct {
	Mode              string
	Text              string
	SavePath          string
	Play              bool
	Lang              string
	Voice             string
	APIKey            string
	CompletionShell   string
	ListVoices        bool
	HasVoiceFlag      bool
	HasLangFlag       bool
	HasAPIKeyFlag     bool
	DefaultSubcommand string
}

func ParseArgs(args []string, stderr io.Writer) (Config, error) {
	if len(args) > 0 && args[0] == ModeDefault {
		return parseDefaultCommand(args[1:], stderr)
	}
	if len(args) > 0 && args[0] == ModeSetup {
		cfg := Config{Mode: ModeSetup}
		if len(args) > 1 {
			return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(args[1:], " "))
		}
		return cfg, nil
	}
	if len(args) > 0 && args[0] == ModeDoctor {
		cfg := Config{Mode: ModeDoctor}
		if len(args) > 1 {
			return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(args[1:], " "))
		}
		return cfg, nil
	}
	if len(args) > 0 && args[0] == ModeCompletion {
		return parseCompletionCommand(args[1:])
	}

	cfg := Config{}
	cfg.Mode = ModeRun
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.StringVar(&cfg.Text, "text", "", "Text to convert to speech")
	fs.StringVar(&cfg.SavePath, "save", "", "Path to save the output MP3 file (e.g., output.mp3)")
	fs.BoolVar(&cfg.Play, "play", false, "Play the audio immediately")
	fs.StringVar(&cfg.Lang, "lang", DefaultLanguage, "Language code")
	fs.StringVar(&cfg.Voice, "voice", DefaultVoice, "Voice name")
	fs.BoolVar(&cfg.ListVoices, "list-voices", false, "List available voices (filtered by --lang)")

	fs.Usage = func() {
		fmt.Fprintln(stderr, helpTitle)
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, helpUsageSetup)
		fmt.Fprintln(stderr, helpUsageDoctor)
		fmt.Fprintln(stderr, helpUsageCompletion)
		fmt.Fprintln(stderr, helpUsageDefault)
		fmt.Fprintln(stderr, helpUsageRun)
		fmt.Fprintln(stderr, helpUsageList)
		fmt.Fprintln(stderr, "  ttscli --version")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Run Flags:")
		fs.PrintDefaults()
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Examples:")
		fmt.Fprintln(stderr, helpExampleSetup)
		fmt.Fprintln(stderr, helpExampleDoctor)
		fmt.Fprintln(stderr, helpExampleCompletion)
		fmt.Fprintln(stderr, helpExampleDefault)
		fmt.Fprintln(stderr, helpExampleSpeak)
		fmt.Fprintln(stderr, helpExampleVoices)
	}

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "voice":
			cfg.HasVoiceFlag = true
		case "lang":
			cfg.HasLangFlag = true
		}
	})

	if fs.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	if cfg.ListVoices {
		return cfg, nil
	}

	if len(args) == 0 {
		return cfg, fmt.Errorf("no arguments provided; run \"ttscli --help\" for usage")
	}

	if cfg.Text == "" {
		return cfg, fmt.Errorf("please provide text using the --text flag")
	}

	if cfg.SavePath == "" && !cfg.Play {
		return cfg, fmt.Errorf("please specify either --save <path> or --play (or both)")
	}

	return cfg, nil
}

func parseCompletionCommand(args []string) (Config, error) {
	cfg := Config{Mode: ModeCompletion}
	if len(args) == 0 {
		return cfg, fmt.Errorf("please provide a shell: bash, zsh, or fish")
	}
	if len(args) > 1 {
		return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(args[1:], " "))
	}

	shell := strings.ToLower(strings.TrimSpace(args[0]))
	if _, ok := supportedCompletionShells[shell]; !ok {
		return cfg, fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish)", args[0])
	}
	cfg.CompletionShell = shell
	return cfg, nil
}

func parseDefaultCommand(args []string, stderr io.Writer) (Config, error) {
	cfg := Config{Mode: ModeDefault}
	if len(args) == 0 {
		return cfg, fmt.Errorf("please provide a default subcommand: set, get, or unset")
	}

	cfg.DefaultSubcommand = args[0]
	switch cfg.DefaultSubcommand {
	case DefaultGet:
		if len(args) > 1 {
			return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(args[1:], " "))
		}
		return cfg, nil
	case DefaultSet:
		fs := flag.NewFlagSet(appName+" default set", flag.ContinueOnError)
		fs.SetOutput(stderr)
		fs.StringVar(&cfg.Voice, "voice", "", "Default voice name")
		fs.StringVar(&cfg.Lang, "lang", "", "Default language code")
		fs.StringVar(&cfg.APIKey, "api-key", "", "Default Google Cloud Text-to-Speech API key")
		if err := fs.Parse(args[1:]); err != nil {
			return cfg, err
		}
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "voice":
				cfg.HasVoiceFlag = true
			case "lang":
				cfg.HasLangFlag = true
			case "api-key":
				cfg.HasAPIKeyFlag = true
			}
		})
		if fs.NArg() > 0 {
			return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
		}
		if !cfg.HasVoiceFlag && !cfg.HasLangFlag && !cfg.HasAPIKeyFlag {
			return cfg, fmt.Errorf("please provide --voice, --lang, and/or --api-key")
		}
		return cfg, nil
	case DefaultUnset:
		fs := flag.NewFlagSet(appName+" default unset", flag.ContinueOnError)
		fs.SetOutput(stderr)
		fs.BoolVar(&cfg.HasVoiceFlag, "voice", false, "Unset saved default voice")
		fs.BoolVar(&cfg.HasLangFlag, "lang", false, "Unset saved default language")
		fs.BoolVar(&cfg.HasAPIKeyFlag, "api-key", false, "Unset saved API key")
		if err := fs.Parse(args[1:]); err != nil {
			return cfg, err
		}
		if fs.NArg() > 0 {
			return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
		}
		return cfg, nil
	default:
		return cfg, fmt.Errorf("unsupported default subcommand %q (use: set, get, unset)", cfg.DefaultSubcommand)
	}
}
