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
	helpTitle             = "Multi-Provider Text-to-Speech CLI"
	helpUsageSpeak        = `  ttscli speak --text "Hello world"`
	helpUsageSave         = `  ttscli save --text "Hello world" --out output.mp3`
	helpUsageVoices       = "  ttscli voices --lang en-GB"
	helpUsageSetup        = "  ttscli setup"
	helpUsageDoctor       = "  ttscli doctor"
	helpUsageCompletion   = "  ttscli completion <bash|zsh|fish>"
	helpUsageProfile      = "  ttscli profile <list|create|delete|use|get> [flags]"
	helpExampleSpeak      = `  ttscli speak --text "Hello world"`
	helpExampleSave       = `  ttscli save --text "Hello world" --out output.mp3"`
	helpExampleVoices     = "  ttscli voices --lang en-GB"
	helpExampleSetup      = "  ttscli setup"
	helpExampleDoctor     = "  ttscli doctor"
	helpExampleCompletion = "  ttscli completion zsh"
	helpExampleProfile    = "  ttscli profile create --provider gcp --name default --api-key YOUR_KEY"
	helpAliases           = "Short aliases: -t/--text, -o/--out, -l/--lang, -v/--voice, -p/--profile"
	ModeSpeak             = "speak"
	ModeSave              = "save"
	ModeVoices            = "voices"
	ModeSetup             = "setup"
	ModeDoctor            = "doctor"
	ModeCompletion        = "completion"
	ModeProfile           = "profile"
	ProfileList           = "list"
	ProfileCreate         = "create"
	ProfileDelete         = "delete"
	ProfileUse            = "use"
	ProfileGet            = "get"
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
	if len(args) > 0 && args[0] == ModeProfile {
		return parseProfileCommand(args[1:], stderr)
	}

	return Config{}, fmt.Errorf("unsupported command %q (use: speak, save, voices, setup, doctor, completion, profile)", args[0])
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
	fs.StringVar(&cfg.Profile, "profile", "", "Profile to use (e.g., gcp:default)")
	fs.StringVar(&cfg.Profile, "p", "", "Profile to use (shorthand)")

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
	fs.StringVar(&cfg.Profile, "profile", "", "Profile to use (e.g., gcp:default)")
	fs.StringVar(&cfg.Profile, "p", "", "Profile to use (shorthand)")

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
	fs.StringVar(&cfg.Profile, "profile", "", "Profile to use (e.g., gcp:default)")
	fs.StringVar(&cfg.Profile, "p", "", "Profile to use (shorthand)")

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
	fmt.Fprintln(stderr, helpUsageProfile)
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
	fmt.Fprintln(stderr, helpExampleProfile)
}

func parseProfileCommand(args []string, stderr io.Writer) (Config, error) {
	cfg := Config{Mode: ModeProfile}
	if len(args) == 0 {
		return cfg, fmt.Errorf("please provide a profile subcommand: list, create, delete, use, or get")
	}

	subcommand := args[0]
	cfg.DefaultSubcommand = subcommand

	switch subcommand {
	case ProfileList:
		if len(args) > 1 {
			return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(args[1:], " "))
		}
		return cfg, nil
	case ProfileCreate:
		fs := flag.NewFlagSet(appName+" profile create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		fs.StringVar(&cfg.APIKey, "api-key", "", "API key for the provider")
		fs.StringVar(&cfg.APIKey, "k", "", "API key for the provider (shorthand)")
		fs.StringVar(&cfg.Lang, "provider", "", "Provider name (gcp, aws, azure, ibm, alibaba)")
		fs.StringVar(&cfg.Lang, "P", "", "Provider name (shorthand)")
		fs.StringVar(&cfg.Voice, "name", "", "Profile name")
		fs.StringVar(&cfg.Voice, "n", "", "Profile name (shorthand)")
		fs.StringVar(&cfg.SavePath, "voice", "", "Default voice for this profile")
		fs.StringVar(&cfg.SavePath, "v", "", "Default voice for this profile (shorthand)")
		if err := fs.Parse(args[1:]); err != nil {
			return cfg, err
		}
		if fs.NArg() > 0 {
			return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
		}
		return cfg, nil
	case ProfileDelete:
		if len(args) < 2 {
			return cfg, fmt.Errorf("please provide profile key to delete (e.g., gcp:default)")
		}
		if len(args) > 2 {
			return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(args[2:], " "))
		}
		cfg.Profile = args[1]
		return cfg, nil
	case ProfileUse:
		if len(args) < 2 {
			return cfg, fmt.Errorf("please provide profile key to use (e.g., gcp:default)")
		}
		if len(args) > 2 {
			return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(args[2:], " "))
		}
		cfg.Profile = args[1]
		return cfg, nil
	case ProfileGet:
		if len(args) < 2 {
			return cfg, fmt.Errorf("please provide profile key to get (e.g., gcp:default)")
		}
		if len(args) > 2 {
			return cfg, fmt.Errorf("unexpected positional arguments: %s", strings.Join(args[2:], " "))
		}
		cfg.Profile = args[1]
		return cfg, nil
	default:
		return cfg, fmt.Errorf("unsupported profile subcommand %q (use: list, create, delete, use, get)", subcommand)
	}
}
