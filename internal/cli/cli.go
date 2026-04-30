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
	helpTitle             = "ttscli - Multi-Provider Text-to-Speech CLI"
	helpDescription       = "Convert text to speech using cloud TTS providers (Google Cloud, AWS, Azure, and more)."
	helpUsageSpeak        = `  ttscli speak --text "Hello world"`
	helpUsageSave         = `  ttscli save --text "Hello world" --out output.mp3`
	helpUsageVoices       = "  ttscli voices --lang en-GB"
	helpExampleSpeak      = `  ttscli speak --text "Hello world, this is a test."`
	helpExampleSave       = `  ttscli save --text "Save this to a file." --out output.mp3`
	helpExampleVoices     = "  ttscli voices --lang en-GB"
	helpExampleSetup      = "  ttscli setup"
	helpExampleDoctor     = "  ttscli doctor"
	helpExampleProfile    = "  ttscli profile create --provider gcp --name work --api-key YOUR_API_KEY"
	helpExampleSpeakAlias = `  ttscli speak -t "Quick test" -l en-GB -v en-GB-Neural2-B`
	helpExampleSaveAlias  = `  ttscli save -t "Save this" -o speech.mp3`
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
	DefaultSubcommand string
	Profile           string
	Provider          string
	ProfileName       string
	DefaultVoice      string
}

func ParseArgs(args []string, stderr io.Writer) (Config, error) {
	if len(args) == 0 {
		return Config{}, fmt.Errorf("no command provided. Run 'ttscli --help' for usage")
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
			return cfg, fmt.Errorf("setup does not accept arguments: %s", strings.Join(args[1:], " "))
		}
		return cfg, nil
	}
	if len(args) > 0 && args[0] == ModeDoctor {
		cfg := Config{Mode: ModeDoctor}
		if len(args) > 1 {
			return cfg, fmt.Errorf("doctor does not accept arguments: %s", strings.Join(args[1:], " "))
		}
		return cfg, nil
	}
	if len(args) > 0 && args[0] == ModeCompletion {
		return parseCompletionCommand(args[1:])
	}
	if len(args) > 0 && args[0] == ModeProfile {
		return parseProfileCommand(args[1:], stderr)
	}

	return Config{}, fmt.Errorf("unknown command %q. Run 'ttscli --help' for available commands", args[0])
}

func parseCompletionCommand(args []string) (Config, error) {
	cfg := Config{Mode: ModeCompletion}
	if len(args) == 0 {
		return cfg, fmt.Errorf("please specify a shell: bash, zsh, or fish")
	}
	if len(args) > 1 {
		return cfg, fmt.Errorf("too many arguments. Expected one shell: bash, zsh, or fish")
	}

	shell := strings.ToLower(strings.TrimSpace(args[0]))
	if _, ok := supportedCompletionShells[shell]; !ok {
		return cfg, fmt.Errorf("unsupported shell %q. Supported shells: bash, zsh, fish", args[0])
	}
	cfg.CompletionShell = shell
	return cfg, nil
}

func addLangFlag(fs *flag.FlagSet, target *string, usage string) {
	fs.StringVar(target, "lang", DefaultLanguage, usage)
	fs.StringVar(target, "l", DefaultLanguage, "Language code (shorthand)")
}

func addVoiceFlag(fs *flag.FlagSet, target *string, usage string) {
	fs.StringVar(target, "voice", DefaultVoice, usage)
	fs.StringVar(target, "v", DefaultVoice, "Voice name (shorthand)")
}

func addProfileFlag(fs *flag.FlagSet, target *string) {
	fs.StringVar(target, "profile", "", "Profile to use (e.g., gcp:default)")
	fs.StringVar(target, "p", "", "Profile to use (shorthand)")
}

func markExplicitSpeechFlags(fs *flag.FlagSet, cfg *Config) {
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "voice", "v":
			cfg.HasVoiceFlag = true
		case "lang", "l":
			cfg.HasLangFlag = true
		}
	})
}

func markExplicitVoiceListFlags(fs *flag.FlagSet, cfg *Config) {
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "lang" || f.Name == "l" {
			cfg.HasLangFlag = true
		}
	})
}

func parseSpeakCommand(args []string, stderr io.Writer) (Config, error) {
	cfg := Config{Mode: ModeSpeak}
	fs := flag.NewFlagSet(appName+" speak", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.StringVar(&cfg.Text, "text", "", "Text to convert to speech")
	fs.StringVar(&cfg.Text, "t", "", "Text to convert to speech (shorthand)")
	addLangFlag(fs, &cfg.Lang, "Language code (e.g., en-US, en-GB, fr-FR)")
	addVoiceFlag(fs, &cfg.Voice, "Voice name for synthesis")
	addProfileFlag(fs, &cfg.Profile)

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	markExplicitSpeechFlags(fs, &cfg)

	if fs.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected arguments: %s. Use --text or -t to provide text", strings.Join(fs.Args(), " "))
	}
	if cfg.Text == "" {
		return cfg, fmt.Errorf("text is required. Use --text or -t to provide text to synthesize")
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
	addLangFlag(fs, &cfg.Lang, "Language code (e.g., en-US, en-GB, fr-FR)")
	addVoiceFlag(fs, &cfg.Voice, "Voice name for synthesis")
	addProfileFlag(fs, &cfg.Profile)

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	markExplicitSpeechFlags(fs, &cfg)

	if fs.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected arguments: %s. Use --text/-t and --out/-o", strings.Join(fs.Args(), " "))
	}
	if cfg.Text == "" {
		return cfg, fmt.Errorf("text is required. Use --text or -t to provide text to synthesize")
	}
	if cfg.SavePath == "" {
		return cfg, fmt.Errorf("output path is required. Use --out or -o to specify the MP3 file path")
	}
	return cfg, nil
}

func parseVoicesCommand(args []string, stderr io.Writer) (Config, error) {
	cfg := Config{Mode: ModeVoices}
	fs := flag.NewFlagSet(appName+" voices", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addLangFlag(fs, &cfg.Lang, "Language code to filter voices (e.g., en-US, en-GB)")
	addProfileFlag(fs, &cfg.Profile)

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	markExplicitVoiceListFlags(fs, &cfg)

	if fs.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected arguments: %s. Use --lang or -l to filter by language", strings.Join(fs.Args(), " "))
	}
	cfg.ListVoices = true
	return cfg, nil
}

func printHelp(stderr io.Writer) {
	fmt.Fprintln(stderr, helpTitle)
	fmt.Fprintln(stderr, helpDescription)
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Commands:")
	fmt.Fprintln(stderr, "  speak       Synthesize text and play audio immediately")
	fmt.Fprintln(stderr, "  save        Synthesize text and save to MP3 file")
	fmt.Fprintln(stderr, "  voices      List available voices for a language")
	fmt.Fprintln(stderr, "  setup       Run interactive first-time setup")
	fmt.Fprintln(stderr, "  doctor      Run diagnostics to check configuration")
	fmt.Fprintln(stderr, "  completion  Generate shell completion scripts")
	fmt.Fprintln(stderr, "  profile     Manage TTS provider profiles")
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Usage:")
	fmt.Fprintln(stderr, "  ttscli <command> [flags]")
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Examples:")
	fmt.Fprintln(stderr, helpUsageSpeak)
	fmt.Fprintln(stderr, helpUsageSave)
	fmt.Fprintln(stderr, helpUsageVoices)
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "More Examples:")
	fmt.Fprintln(stderr, helpExampleSpeak)
	fmt.Fprintln(stderr, helpExampleSave)
	fmt.Fprintln(stderr, helpExampleVoices)
	fmt.Fprintln(stderr, helpExampleSpeakAlias)
	fmt.Fprintln(stderr, helpExampleSaveAlias)
	fmt.Fprintln(stderr, helpExampleSetup)
	fmt.Fprintln(stderr, helpExampleDoctor)
	fmt.Fprintln(stderr, helpExampleProfile)
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Flags:")
	fmt.Fprintln(stderr, "  -t, --text      Text to convert to speech (speak/save)")
	fmt.Fprintln(stderr, "  -o, --out       Output MP3 file path (save)")
	fmt.Fprintln(stderr, "  -l, --lang      Language code (default: en-US)")
	fmt.Fprintln(stderr, "  -v, --voice     Voice name (default: en-US-Neural2-F)")
	fmt.Fprintln(stderr, "  -p, --profile   Profile to use (e.g., gcp:default)")
	fmt.Fprintln(stderr, "  --version       Print version information")
	fmt.Fprintln(stderr, "  --help, -h      Show this help message")
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Profiles:")
	fmt.Fprintln(stderr, "  Profiles store provider credentials and default settings.")
	fmt.Fprintln(stderr, "  Format: provider:name (e.g., gcp:default, aws:work)")
	fmt.Fprintln(stderr, "  Run 'ttscli profile --help' for profile management.")
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Documentation: https://github.com/ppikrorngarn/ttscli")
}

func parseProfileCommand(args []string, stderr io.Writer) (Config, error) {
	cfg := Config{Mode: ModeProfile}
	if len(args) == 0 {
		return cfg, fmt.Errorf("profile subcommand required. Use: list, create, delete, use, or get. Run 'ttscli profile --help' for details")
	}

	subcommand := args[0]
	cfg.DefaultSubcommand = subcommand

	switch subcommand {
	case ProfileList:
		if len(args) > 1 {
			return cfg, fmt.Errorf("list does not accept arguments. Usage: ttscli profile list")
		}
		return cfg, nil
	case ProfileCreate:
		fs := flag.NewFlagSet(appName+" profile create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		fs.StringVar(&cfg.APIKey, "api-key", "", "API key for the provider")
		fs.StringVar(&cfg.APIKey, "k", "", "API key (shorthand)")
		fs.StringVar(&cfg.Provider, "provider", "", "Provider name (gcp, aws, azure, ibm, alibaba)")
		fs.StringVar(&cfg.Provider, "P", "", "Provider name (shorthand)")
		fs.StringVar(&cfg.ProfileName, "name", "", "Profile name")
		fs.StringVar(&cfg.ProfileName, "n", "", "Profile name (shorthand)")
		fs.StringVar(&cfg.DefaultVoice, "voice", "", "Default voice for this profile")
		fs.StringVar(&cfg.DefaultVoice, "v", "", "Default voice (shorthand)")
		if err := fs.Parse(args[1:]); err != nil {
			return cfg, err
		}
		if fs.NArg() > 0 {
			return cfg, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
		}
		return cfg, nil
	case ProfileDelete:
		if len(args) < 2 {
			return cfg, fmt.Errorf("profile key required. Usage: ttscli profile delete gcp:default")
		}
		if len(args) > 2 {
			return cfg, fmt.Errorf("too many arguments. Usage: ttscli profile delete gcp:default")
		}
		cfg.Profile = args[1]
		return cfg, nil
	case ProfileUse:
		if len(args) < 2 {
			return cfg, fmt.Errorf("profile key required. Usage: ttscli profile use gcp:default")
		}
		if len(args) > 2 {
			return cfg, fmt.Errorf("too many arguments. Usage: ttscli profile use gcp:default")
		}
		cfg.Profile = args[1]
		return cfg, nil
	case ProfileGet:
		if len(args) < 2 {
			return cfg, fmt.Errorf("profile key required. Usage: ttscli profile get gcp:default")
		}
		if len(args) > 2 {
			return cfg, fmt.Errorf("too many arguments. Usage: ttscli profile get gcp:default")
		}
		cfg.Profile = args[1]
		return cfg, nil
	default:
		return cfg, fmt.Errorf("unknown profile subcommand %q. Use: list, create, delete, use, or get", subcommand)
	}
}
