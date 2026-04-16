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
	helpUsageSpeak        = `  ttscli speak --text "Hello world"`
	helpUsageSave         = `  ttscli save --text "Hello world" --out output.mp3`
	helpUsageVoices       = "  ttscli voices --lang en-GB"
	helpUsageSetup        = "  ttscli setup"
	helpUsageDoctor       = "  ttscli doctor"
	helpUsageCompletion   = "  ttscli completion <bash|zsh|fish>"
	helpUsageDefault      = "  ttscli default <set|get|unset> [flags]"
	helpExampleSpeak      = `  ttscli speak --text "Hello world"`
	helpExampleSave       = `  ttscli save --text "Hello world" --out output.mp3`
	helpExampleVoices     = "  ttscli voices --lang en-GB"
	helpExampleSetup      = "  ttscli setup"
	helpExampleDoctor     = "  ttscli doctor"
	helpExampleCompletion = "  ttscli completion zsh"
	helpExampleDefault    = "  ttscli default set --voice en-US-Chirp3-HD-Achernar --lang en-US"
	helpAliases           = "Short aliases: -t/--text, -o/--out, -l/--lang, -v/--voice, -k/--api-key"
	ModeSpeak             = "speak"
	ModeSave              = "save"
	ModeVoices            = "voices"
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
	Profile           string
}

func ParseArgs(args []string, stderr io.Writer) (Config, error) {
	if len(args) == 0 {
		return Config{}, fmt.Errorf("no command provided; run \"ttscli --help\" for usage")
	}

	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printHelp(stderr)
		return Config{}, flag.ErrHelp
	}

	if len(args) > 0 && args[0] == ModeSpeak {
		return parseSpeakCommand(args[1:], stderr)
	}
	if len(args) > 0 && args[0] == ModeSave {
		return parseSaveCommand(args[1:], stderr)
	}
	if len(args) > 0 && args[0] == ModeVoices {
		return parseVoicesCommand(args[1:], stderr)
	}
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

	return Config{}, fmt.Errorf("unsupported command %q (use: speak, save, voices, setup, doctor, completion, default)", args[0])
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

func parseSpeakCommand(args []string, stderr io.Writer) (Config, error) {
	cfg := Config{Mode: ModeSpeak}
	fs := flag.NewFlagSet(appName+" speak", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.StringVar(&cfg.Text, "text", "", "Text to convert to speech")
	fs.StringVar(&cfg.Text, "t", "", "Text to convert to speech (shorthand)")
	fs.StringVar(&cfg.Lang, "lang", DefaultLanguage, "Language code")
	fs.StringVar(&cfg.Lang, "l", DefaultLanguage, "Language code (shorthand)")
	fs.StringVar(&cfg.Voice, "voice", DefaultVoice, "Voice name")
	fs.StringVar(&cfg.Voice, "v", DefaultVoice, "Voice name (shorthand)")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "voice", "v":
			cfg.HasVoiceFlag = true
		case "lang", "l":
			cfg.HasLangFlag = true
		}
	})

	if fs.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if cfg.Text == "" {
		return cfg, fmt.Errorf("please provide text using --text or -t")
	}
	cfg.Play = true
	return cfg, nil
}

func parseSaveCommand(args []string, stderr io.Writer) (Config, error) {
	cfg := Config{Mode: ModeSave}
	fs := flag.NewFlagSet(appName+" save", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.StringVar(&cfg.Text, "text", "", "Text to convert to speech")
	fs.StringVar(&cfg.Text, "t", "", "Text to convert to speech (shorthand)")
	fs.StringVar(&cfg.SavePath, "out", "", "Path to save the output MP3 file (e.g., output.mp3)")
	fs.StringVar(&cfg.SavePath, "o", "", "Path to save the output MP3 file (shorthand)")
	fs.StringVar(&cfg.Lang, "lang", DefaultLanguage, "Language code")
	fs.StringVar(&cfg.Lang, "l", DefaultLanguage, "Language code (shorthand)")
	fs.StringVar(&cfg.Voice, "voice", DefaultVoice, "Voice name")
	fs.StringVar(&cfg.Voice, "v", DefaultVoice, "Voice name (shorthand)")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "voice", "v":
			cfg.HasVoiceFlag = true
		case "lang", "l":
			cfg.HasLangFlag = true
		}
	})

	if fs.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if cfg.Text == "" {
		return cfg, fmt.Errorf("please provide text using --text or -t")
	}
	if cfg.SavePath == "" {
		return cfg, fmt.Errorf("please provide output path using --out or -o")
	}
	return cfg, nil
}

func parseVoicesCommand(args []string, stderr io.Writer) (Config, error) {
	cfg := Config{Mode: ModeVoices}
	fs := flag.NewFlagSet(appName+" voices", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.Lang, "lang", DefaultLanguage, "Language code")
	fs.StringVar(&cfg.Lang, "l", DefaultLanguage, "Language code (shorthand)")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "lang" || f.Name == "l" {
			cfg.HasLangFlag = true
		}
	})

	if fs.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	cfg.ListVoices = true
	return cfg, nil
}

func printHelp(stderr io.Writer) {
	fmt.Fprintln(stderr, helpTitle)
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Usage:")
	fmt.Fprintln(stderr, helpUsageSpeak)
	fmt.Fprintln(stderr, helpUsageSave)
	fmt.Fprintln(stderr, helpUsageVoices)
	fmt.Fprintln(stderr, helpUsageSetup)
	fmt.Fprintln(stderr, helpUsageDoctor)
	fmt.Fprintln(stderr, helpUsageCompletion)
	fmt.Fprintln(stderr, helpUsageDefault)
	fmt.Fprintln(stderr, "  ttscli --version")
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, helpAliases)
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Examples:")
	fmt.Fprintln(stderr, helpExampleSpeak)
	fmt.Fprintln(stderr, helpExampleSave)
	fmt.Fprintln(stderr, helpExampleVoices)
	fmt.Fprintln(stderr, helpExampleSetup)
	fmt.Fprintln(stderr, helpExampleDoctor)
	fmt.Fprintln(stderr, helpExampleCompletion)
	fmt.Fprintln(stderr, helpExampleDefault)
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
		fs.StringVar(&cfg.Voice, "v", "", "Default voice name (shorthand)")
		fs.StringVar(&cfg.Lang, "lang", "", "Default language code")
		fs.StringVar(&cfg.Lang, "l", "", "Default language code (shorthand)")
		fs.StringVar(&cfg.APIKey, "api-key", "", "Default Google Cloud Text-to-Speech API key")
		fs.StringVar(&cfg.APIKey, "k", "", "Default Google Cloud Text-to-Speech API key (shorthand)")
		if err := fs.Parse(args[1:]); err != nil {
			return cfg, err
		}
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "voice", "v":
				cfg.HasVoiceFlag = true
			case "lang", "l":
				cfg.HasLangFlag = true
			case "api-key", "k":
				cfg.HasAPIKeyFlag = true
			}
		})
		if fs.NArg() > 0 {
			return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
		}
		if !cfg.HasVoiceFlag && !cfg.HasLangFlag && !cfg.HasAPIKeyFlag {
			return cfg, fmt.Errorf("please provide --voice/-v, --lang/-l, and/or --api-key/-k")
		}
		return cfg, nil
	case DefaultUnset:
		fs := flag.NewFlagSet(appName+" default unset", flag.ContinueOnError)
		fs.SetOutput(stderr)
		fs.BoolVar(&cfg.HasVoiceFlag, "voice", false, "Unset saved default voice")
		fs.BoolVar(&cfg.HasVoiceFlag, "v", false, "Unset saved default voice (shorthand)")
		fs.BoolVar(&cfg.HasLangFlag, "lang", false, "Unset saved default language")
		fs.BoolVar(&cfg.HasLangFlag, "l", false, "Unset saved default language (shorthand)")
		fs.BoolVar(&cfg.HasAPIKeyFlag, "api-key", false, "Unset saved API key")
		fs.BoolVar(&cfg.HasAPIKeyFlag, "k", false, "Unset saved API key (shorthand)")
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
