package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/player"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

const apiKeyEnvVar = "TTSCLI_GOOGLE_API_KEY"
const setupSoundCheckText = "Setup complete. This is a sound check from ttscli."

type ttsService interface {
	ListVoices(ctx context.Context, langCode string) ([]tts.Voice, error)
	Synthesize(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error)
}

type doctorCheck struct {
	name   string
	ok     bool
	detail string
	hint   string
}

var (
	parseArgs               = cli.ParseArgs
	lookupEnv               = os.Getenv
	currentGOOS             = func() string { return runtime.GOOS }
	lookPathCmd             = exec.LookPath
	newTTSClient            = func(apiKey string) ttsService { return tts.NewClient(apiKey, nil) }
	loadDefaults            = config.LoadDefaults
	saveDefaults            = config.SaveDefaults
	clearDefaults           = config.ClearDefaults
	printVoices             = tts.PrintVoices
	writeFile               = os.WriteFile
	playAudio               = player.PlayAudio
	setupInput    io.Reader = os.Stdin
	newAppCtx               = func() (context.Context, context.CancelFunc) {
		return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	}
)

func Run(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}

	switch cfg.Mode {
	case cli.ModeDefault:
		return runDefaultCommand(cfg, stdout)
	case cli.ModeSetup:
		return runSetupCommand(stdout, stderr)
	case cli.ModeDoctor:
		return runDoctorCommand(stdout)
	case cli.ModeCompletion:
		return runCompletionCommand(cfg, stdout)
	case "":
		// For backward compatibility in tests that may stub ParseArgs manually.
		cfg.Mode = cli.ModeRun
	case cli.ModeRun:
		// Continue with synth/list flow below.
	default:
		return fmt.Errorf("unsupported mode: %s", cfg.Mode)
	}

	defaults, err := loadDefaults()
	if err != nil {
		return fmt.Errorf("load defaults: %w", err)
	}
	apiKey := resolveAPIKey(defaults.APIKey)
	if apiKey == "" {
		return fmt.Errorf("%s environment variable is not set and no saved API key found", apiKeyEnvVar)
	}

	client := newTTSClient(apiKey)
	cfg = resolveRunDefaults(cfg, defaults)
	ctx, stop := newAppCtx()
	defer stop()

	if cfg.ListVoices {
		voices, err := client.ListVoices(ctx, cfg.Lang)
		if err != nil {
			return fmt.Errorf("failed to list voices: %w", err)
		}
		printVoices(stdout, cfg.Lang, voices)
		return nil
	}

	fmt.Fprintln(stdout, "Synthesizing speech...")
	audioBytes, err := client.Synthesize(ctx, cfg.Text, cfg.Lang, cfg.Voice, tts.AudioEncodingMP3)
	if err != nil {
		return fmt.Errorf("failed to synthesize: %w", err)
	}

	if cfg.SavePath != "" {
		if err := writeFile(cfg.SavePath, audioBytes, 0o644); err != nil {
			return fmt.Errorf("failed to save file: %w", err)
		}
		fmt.Fprintf(stdout, "Saved audio to: %s\n", cfg.SavePath)
	}

	if cfg.Play {
		fmt.Fprintln(stdout, "Playing audio...")
		if err := playAudio(audioBytes, stdout, stderr); err != nil {
			return fmt.Errorf("failed to play audio: %w", err)
		}
	}

	return nil
}

func runDefaultCommand(cfg cli.Config, stdout io.Writer) error {
	switch cfg.DefaultSubcommand {
	case cli.DefaultGet:
		defaults, err := loadDefaults()
		if err != nil {
			return fmt.Errorf("load defaults: %w", err)
		}

		voice := defaults.Voice
		lang := defaults.Lang
		if strings.TrimSpace(voice) == "" {
			voice = "(not set)"
		}
		if strings.TrimSpace(lang) == "" {
			lang = "(not set)"
		}
		fmt.Fprintf(stdout, "Default voice: %s\n", voice)
		fmt.Fprintf(stdout, "Default language: %s\n", lang)
		fmt.Fprintf(stdout, "Default API key: %s\n", maskAPIKey(defaults.APIKey))
		return nil
	case cli.DefaultUnset:
		return runDefaultUnset(cfg, stdout)
	case cli.DefaultSet:
		return runDefaultSet(cfg, stdout)
	default:
		return fmt.Errorf("unsupported default subcommand: %s", cfg.DefaultSubcommand)
	}
}

func runSetupCommand(stdout, stderr io.Writer) error {
	defaults, err := loadDefaults()
	if err != nil {
		return fmt.Errorf("load defaults: %w", err)
	}

	reader := bufio.NewReader(setupInput)
	envAPIKey := strings.TrimSpace(lookupEnv(apiKeyEnvVar))
	currentAPIKey := strings.TrimSpace(defaults.APIKey)

	fmt.Fprintln(stdout, "Welcome to ttscli setup.")
	fmt.Fprintf(stdout, "Press Enter on language/voice to use built-in defaults: %s / %s.\n", cli.DefaultLanguage, cli.DefaultVoice)
	if envAPIKey != "" {
		fmt.Fprintf(stdout, "Detected %s in environment; press Enter for API key to use it.\n", apiKeyEnvVar)
	}

	apiKeyPrompt := "Google Cloud API key: "
	if currentAPIKey != "" {
		apiKeyPrompt = "Google Cloud API key (press Enter to keep saved key): "
	}
	inputAPIKey, err := promptLine(reader, stdout, apiKeyPrompt)
	if err != nil {
		return fmt.Errorf("read api key: %w", err)
	}
	apiKey := strings.TrimSpace(inputAPIKey)
	if apiKey == "" {
		if currentAPIKey != "" {
			apiKey = currentAPIKey
		} else if envAPIKey != "" {
			apiKey = envAPIKey
		}
	}
	if apiKey == "" {
		return fmt.Errorf("api key is required: set one in setup or via %s", apiKeyEnvVar)
	}

	inputLang, err := promptLine(reader, stdout, fmt.Sprintf("Default language [%s] (press Enter to use default): ", cli.DefaultLanguage))
	if err != nil {
		return fmt.Errorf("read language: %w", err)
	}
	lang := strings.TrimSpace(inputLang)
	if lang == "" {
		lang = cli.DefaultLanguage
	}

	inputVoice, err := promptLine(reader, stdout, fmt.Sprintf("Default voice [%s] (press Enter to use default): ", cli.DefaultVoice))
	if err != nil {
		return fmt.Errorf("read voice: %w", err)
	}
	voice := strings.TrimSpace(inputVoice)
	if voice == "" {
		voice = cli.DefaultVoice
	}

	ctx, stop := newAppCtx()
	defer stop()

	client := newTTSClient(apiKey)
	voices, err := client.ListVoices(ctx, lang)
	if err != nil {
		return fmt.Errorf("validate defaults via list voices: %w", err)
	}
	if !voiceExists(voice, voices) {
		return fmt.Errorf("voice %q is not available for language %q", voice, lang)
	}

	merged := defaults
	merged.APIKey = apiKey
	merged.Lang = lang
	merged.Voice = voice
	if err := saveDefaults(merged); err != nil {
		return fmt.Errorf("save defaults: %w", err)
	}

	runCheck, err := promptYesNo(reader, stdout, "Run sound check now? [Y/n]: ", true)
	if err != nil {
		return fmt.Errorf("read sound check choice: %w", err)
	}
	if runCheck {
		fmt.Fprintln(stdout, "Running sound check...")
		audioBytes, err := client.Synthesize(ctx, setupSoundCheckText, lang, voice, tts.AudioEncodingMP3)
		if err != nil {
			return fmt.Errorf("failed to synthesize sound check: %w", err)
		}
		fmt.Fprintln(stdout, "Playing audio...")
		if err := playAudio(audioBytes, stdout, stderr); err != nil {
			return fmt.Errorf("failed to play sound check: %w", err)
		}
	}

	cfgPath, err := config.Path()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	fmt.Fprintln(stdout, "Setup complete.")
	fmt.Fprintf(stdout, "Saved defaults: voice=%s lang=%s apiKey=%s\n", merged.Voice, merged.Lang, maskAPIKey(merged.APIKey))
	fmt.Fprintf(stdout, "Config file: %s\n", cfgPath)
	return nil
}

func runDoctorCommand(stdout io.Writer) error {
	var checks []doctorCheck

	defaults, defaultsErr := loadDefaults()
	if defaultsErr != nil {
		checks = append(checks, doctorCheck{
			name:   "Saved config",
			ok:     false,
			detail: defaultsErr.Error(),
			hint:   "Run \"ttscli default unset\" to clear invalid config, then run \"ttscli setup\".",
		})
	} else {
		checks = append(checks, doctorCheck{
			name:   "Saved config",
			ok:     true,
			detail: "config file is readable",
		})
	}

	envKey := strings.TrimSpace(lookupEnv(apiKeyEnvVar))
	savedKey := ""
	if defaultsErr == nil {
		savedKey = strings.TrimSpace(defaults.APIKey)
	}

	apiKey := ""
	apiKeySource := ""
	switch {
	case envKey != "":
		apiKey = envKey
		apiKeySource = apiKeyEnvVar
	case savedKey != "":
		apiKey = savedKey
		apiKeySource = "saved config"
	}

	if apiKey == "" {
		checks = append(checks, doctorCheck{
			name:   "API key availability",
			ok:     false,
			detail: "no API key found in environment or saved defaults",
			hint:   fmt.Sprintf("Run \"ttscli default set --api-key <key>\" or export %s.", apiKeyEnvVar),
		})
		checks = append(checks, doctorCheck{
			name:   "API connectivity",
			ok:     false,
			detail: "skipped: missing API key",
			hint:   fmt.Sprintf("Configure API key first via \"ttscli setup\" or %s.", apiKeyEnvVar),
		})
	} else {
		checks = append(checks, doctorCheck{
			name:   "API key availability",
			ok:     true,
			detail: fmt.Sprintf("API key found via %s", apiKeySource),
		})

		ctx, stop := newAppCtx()
		client := newTTSClient(apiKey)
		voices, err := client.ListVoices(ctx, cli.DefaultLanguage)
		stop()
		if err != nil {
			checks = append(checks, doctorCheck{
				name:   "API connectivity",
				ok:     false,
				detail: err.Error(),
				hint:   "Verify API key restrictions and Cloud Text-to-Speech API enablement.",
			})
		} else if len(voices) == 0 {
			checks = append(checks, doctorCheck{
				name:   "API connectivity",
				ok:     false,
				detail: "connected but returned no voices",
				hint:   "Check API permissions and account/project status.",
			})
		} else {
			checks = append(checks, doctorCheck{
				name:   "API connectivity",
				ok:     true,
				detail: fmt.Sprintf("successfully listed %d voices for %s", len(voices), cli.DefaultLanguage),
			})
		}
	}

	playbackCheck := checkPlaybackCapability(currentGOOS(), lookPathCmd)
	checks = append(checks, playbackCheck)

	failed := printDoctorChecks(stdout, checks)
	if failed > 0 {
		fmt.Fprintf(stdout, "Doctor result: FAIL (%d failed)\n", failed)
		return fmt.Errorf("doctor failed with %d check(s)", failed)
	}

	fmt.Fprintln(stdout, "Doctor result: OK")
	return nil
}

func runCompletionCommand(cfg cli.Config, stdout io.Writer) error {
	var script string
	switch cfg.CompletionShell {
	case "bash":
		script = bashCompletionScript()
	case "zsh":
		script = zshCompletionScript()
	case "fish":
		script = fishCompletionScript()
	default:
		return fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish)", cfg.CompletionShell)
	}
	fmt.Fprint(stdout, script)
	return nil
}

func runDefaultUnset(cfg cli.Config, stdout io.Writer) error {
	if !cfg.HasVoiceFlag && !cfg.HasLangFlag && !cfg.HasAPIKeyFlag {
		if err := clearDefaults(); err != nil {
			return fmt.Errorf("clear defaults: %w", err)
		}
		fmt.Fprintln(stdout, "Cleared saved defaults.")
		return nil
	}

	defaults, err := loadDefaults()
	if err != nil {
		return fmt.Errorf("load defaults: %w", err)
	}

	if cfg.HasVoiceFlag {
		defaults.Voice = ""
	}
	if cfg.HasLangFlag {
		defaults.Lang = ""
	}
	if cfg.HasAPIKeyFlag {
		defaults.APIKey = ""
	}

	if defaultsEmpty(defaults) {
		if err := clearDefaults(); err != nil {
			return fmt.Errorf("clear defaults: %w", err)
		}
	} else {
		if err := saveDefaults(defaults); err != nil {
			return fmt.Errorf("save defaults: %w", err)
		}
	}

	fmt.Fprintln(stdout, "Updated saved defaults.")
	return nil
}

func runDefaultSet(cfg cli.Config, stdout io.Writer) error {
	existing, err := loadDefaults()
	if err != nil {
		return fmt.Errorf("load defaults: %w", err)
	}

	merged := existing
	if cfg.HasVoiceFlag {
		merged.Voice = cfg.Voice
	}
	if cfg.HasLangFlag {
		merged.Lang = cfg.Lang
	}
	if cfg.HasAPIKeyFlag {
		merged.APIKey = cfg.APIKey
	}
	if strings.TrimSpace(merged.Lang) == "" {
		merged.Lang = cli.DefaultLanguage
	}
	if strings.TrimSpace(merged.Voice) == "" {
		merged.Voice = cli.DefaultVoice
	}

	validationKey := resolveAPIKey(merged.APIKey)
	if cfg.HasAPIKeyFlag {
		validationKey = strings.TrimSpace(merged.APIKey)
	}
	if validationKey == "" {
		return fmt.Errorf("%s environment variable is not set and no saved API key found", apiKeyEnvVar)
	}

	ctx, stop := newAppCtx()
	defer stop()

	client := newTTSClient(validationKey)
	voices, err := client.ListVoices(ctx, merged.Lang)
	if err != nil {
		return fmt.Errorf("validate defaults via list voices: %w", err)
	}
	if !voiceExists(merged.Voice, voices) {
		return fmt.Errorf("voice %q is not available for language %q", merged.Voice, merged.Lang)
	}

	if err := saveDefaults(merged); err != nil {
		return fmt.Errorf("save defaults: %w", err)
	}
	fmt.Fprintf(stdout, "Saved defaults: voice=%s lang=%s apiKey=%s\n", merged.Voice, merged.Lang, maskAPIKey(merged.APIKey))
	return nil
}

func resolveRunDefaults(cfg cli.Config, defaults config.Defaults) cli.Config {
	if !cfg.HasVoiceFlag && strings.TrimSpace(defaults.Voice) != "" {
		cfg.Voice = defaults.Voice
	}
	if !cfg.HasLangFlag && strings.TrimSpace(defaults.Lang) != "" {
		cfg.Lang = defaults.Lang
	}
	return cfg
}

func resolveAPIKey(savedAPIKey string) string {
	if envKey := strings.TrimSpace(lookupEnv(apiKeyEnvVar)); envKey != "" {
		return envKey
	}
	return strings.TrimSpace(savedAPIKey)
}

func defaultsEmpty(d config.Defaults) bool {
	return strings.TrimSpace(d.Voice) == "" &&
		strings.TrimSpace(d.Lang) == "" &&
		strings.TrimSpace(d.APIKey) == ""
}

func maskAPIKey(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "(not set)"
	}
	if len(trimmed) <= 4 {
		return "****"
	}
	return "****" + trimmed[len(trimmed)-4:]
}

func voiceExists(voiceName string, voices []tts.Voice) bool {
	for _, v := range voices {
		if strings.EqualFold(v.Name, voiceName) {
			return true
		}
	}
	return false
}

func promptLine(reader *bufio.Reader, stdout io.Writer, prompt string) (string, error) {
	fmt.Fprint(stdout, prompt)
	raw, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(raw), nil
}

func promptYesNo(reader *bufio.Reader, stdout io.Writer, prompt string, defaultYes bool) (bool, error) {
	for {
		input, err := promptLine(reader, stdout, prompt)
		if err != nil {
			return false, err
		}
		if input == "" {
			return defaultYes, nil
		}
		switch strings.ToLower(input) {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(stdout, "Please answer y or n.")
		}
	}
}

func checkPlaybackCapability(goos string, lookPath func(file string) (string, error)) doctorCheck {
	switch goos {
	case "darwin":
		if _, err := lookPath("afplay"); err != nil {
			return doctorCheck{
				name:   "Audio playback",
				ok:     false,
				detail: "required player command \"afplay\" not found",
				hint:   "Install command-line audio playback support for macOS.",
			}
		}
		return doctorCheck{name: "Audio playback", ok: true, detail: "found afplay"}
	case "linux":
		if _, err := lookPath("mpg123"); err == nil {
			return doctorCheck{name: "Audio playback", ok: true, detail: "found mpg123"}
		}
		if _, err := lookPath("paplay"); err == nil {
			return doctorCheck{name: "Audio playback", ok: true, detail: "found paplay"}
		}
		if _, err := lookPath("ffplay"); err == nil {
			return doctorCheck{name: "Audio playback", ok: true, detail: "found ffplay"}
		}
		return doctorCheck{
			name:   "Audio playback",
			ok:     false,
			detail: "no supported player found (mpg123, paplay, ffplay)",
			hint:   "Install mpg123 (for example: sudo apt install mpg123).",
		}
	case "windows":
		if _, err := lookPath("powershell"); err != nil {
			if _, errExe := lookPath("powershell.exe"); errExe != nil {
				return doctorCheck{
					name:   "Audio playback",
					ok:     false,
					detail: "required player command \"powershell\" not found",
					hint:   "Ensure PowerShell is available in PATH.",
				}
			}
		}
		return doctorCheck{name: "Audio playback", ok: true, detail: "found powershell"}
	default:
		return doctorCheck{
			name:   "Audio playback",
			ok:     false,
			detail: fmt.Sprintf("unsupported platform: %s", goos),
			hint:   "Audio playback is supported on macOS, Linux, and Windows.",
		}
	}
}

func printDoctorChecks(stdout io.Writer, checks []doctorCheck) int {
	fmt.Fprintln(stdout, "Running doctor checks...")
	failed := 0
	for _, check := range checks {
		status := "PASS"
		if !check.ok {
			status = "FAIL"
			failed++
		}
		fmt.Fprintf(stdout, "[%s] %s: %s\n", status, check.name, check.detail)
		if !check.ok && strings.TrimSpace(check.hint) != "" {
			fmt.Fprintf(stdout, "  fix: %s\n", check.hint)
		}
	}
	return failed
}

func bashCompletionScript() string {
	return `# bash completion for ttscli
_ttscli_completion() {
  local cur prev words cword
  words=("${COMP_WORDS[@]}")
  cword=$COMP_CWORD
  cur="${COMP_WORDS[COMP_CWORD]}"
  if [[ ${COMP_CWORD} -gt 0 ]]; then
    prev="${COMP_WORDS[COMP_CWORD-1]}"
  else
    prev=""
  fi

  if [[ ${cword} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "setup doctor completion default --version --help" -- "${cur}") )
    return
  fi

  case "${words[1]}" in
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish" -- "${cur}") )
      ;;
    default)
      if [[ ${cword} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "set get unset" -- "${cur}") )
      else
        case "${words[2]}" in
          set)
            COMPREPLY=( $(compgen -W "--voice --lang --api-key" -- "${cur}") )
            ;;
          unset)
            COMPREPLY=( $(compgen -W "--voice --lang --api-key" -- "${cur}") )
            ;;
        esac
      fi
      ;;
    *)
      COMPREPLY=( $(compgen -W "--text --save --play --lang --voice --list-voices --version --help" -- "${cur}") )
      ;;
  esac
}

complete -F _ttscli_completion ttscli
`
}

func zshCompletionScript() string {
	return `#compdef ttscli

_ttscli() {
  local -a run_flags
  run_flags=(
    '--text[Text to convert to speech]:text:'
    '--save[Path to save MP3 output]:file:_files'
    '--play[Play audio immediately]'
    '--lang[Language code]:lang:'
    '--voice[Voice name]:voice:'
    '--list-voices[List available voices]'
    '--version[Print version and exit]'
    '--help[Show help]'
  )

  if (( CURRENT == 2 )); then
    _describe 'command' \
      'setup:Run first-time setup' \
      'doctor:Run diagnostics' \
      'completion:Generate shell completions' \
      'default:Manage saved defaults' \
      '--version:Print version' \
      '--help:Show help'
    return
  fi

  case "$words[2]" in
    completion)
      _values 'shell' bash zsh fish
      ;;
    default)
      if (( CURRENT == 3 )); then
        _values 'subcommand' set get unset
      else
        case "$words[3]" in
          set|unset)
            _values 'flags' --voice --lang --api-key
            ;;
        esac
      fi
      ;;
    *)
      _arguments -s $run_flags
      ;;
  esac
}

_ttscli "$@"
`
}

func fishCompletionScript() string {
	return `# fish completion for ttscli
complete -c ttscli -f -n "__fish_use_subcommand" -a setup -d "Run first-time setup"
complete -c ttscli -f -n "__fish_use_subcommand" -a doctor -d "Run diagnostics"
complete -c ttscli -f -n "__fish_use_subcommand" -a completion -d "Generate shell completions"
complete -c ttscli -f -n "__fish_use_subcommand" -a default -d "Manage saved defaults"
complete -c ttscli -f -n "__fish_use_subcommand" -l version -d "Print version and exit"

complete -c ttscli -f -n "__fish_seen_subcommand_from completion" -a bash
complete -c ttscli -f -n "__fish_seen_subcommand_from completion" -a zsh
complete -c ttscli -f -n "__fish_seen_subcommand_from completion" -a fish

complete -c ttscli -f -n "__fish_seen_subcommand_from default; and not __fish_seen_subcommand_from set get unset" -a set
complete -c ttscli -f -n "__fish_seen_subcommand_from default; and not __fish_seen_subcommand_from set get unset" -a get
complete -c ttscli -f -n "__fish_seen_subcommand_from default; and not __fish_seen_subcommand_from set get unset" -a unset
complete -c ttscli -f -n "__fish_seen_subcommand_from set unset" -l voice
complete -c ttscli -f -n "__fish_seen_subcommand_from set unset" -l lang
complete -c ttscli -f -n "__fish_seen_subcommand_from set unset" -l api-key

complete -c ttscli -l text -d "Text to convert to speech"
complete -c ttscli -l save -d "Path to save MP3 output"
complete -c ttscli -l play -d "Play audio immediately"
complete -c ttscli -l lang -d "Language code"
complete -c ttscli -l voice -d "Voice name"
complete -c ttscli -l list-voices -d "List available voices"
complete -c ttscli -l help -d "Show help"
`
}
