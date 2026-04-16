package app

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

type doctorCheck struct {
	name   string
	ok     bool
	detail string
	hint   string
}

func runSetupCommand(stdout, stderr io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	reader := bufio.NewReader(setupInput)

	fmt.Fprintln(stdout, "Welcome to ttscli setup.")
	fmt.Fprintln(stdout, "This will create a new GCP profile for text-to-speech.")
	fmt.Fprintln(stdout)

	profileName := "default"
	profileKey := "gcp:" + profileName

	if len(appCfg.Profiles) > 0 {
		inputProfileName, err := promptLine(reader, stdout, "Profile name [default]: ")
		if err != nil {
			return fmt.Errorf("read profile name: %w", err)
		}
		profileName = strings.TrimSpace(inputProfileName)
		if profileName == "" {
			profileName = "default"
		}

		profileKey = "gcp:" + profileName
		if _, exists := appCfg.Profiles[profileKey]; exists {
			fmt.Fprintf(stdout, "Profile %s already exists. Use 'ttscli profile use %s' to activate it.\n", profileKey, profileKey)
			return nil
		}
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Press Enter on language/voice to use built-in defaults: %s / %s.\n", cli.DefaultLanguage, cli.DefaultVoice)

	inputAPIKey, err := promptLine(reader, stdout, "Google Cloud API key: ")
	if err != nil {
		return fmt.Errorf("read api key: %w", err)
	}
	apiKey := strings.TrimSpace(inputAPIKey)
	if apiKey == "" {
		return fmt.Errorf("api key is required")
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

	fmt.Fprintln(stdout, "Validating API key and voice...")

	profile := config.Profile{
		Provider: "gcp",
		Name:     profileName,
		Credentials: map[string]interface{}{
			"apiKey": apiKey,
		},
		Defaults: map[string]string{
			"lang":  lang,
			"voice": voice,
		},
	}

	provider, err := newProvider(profile)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	ctx, stop := newAppCtx()
	defer stop()

	voices, err := provider.ListVoices(ctx, lang)
	if err != nil {
		return fmt.Errorf("validate provider: %w", err)
	}
	if !voiceExists(voice, voices) {
		return fmt.Errorf("voice %q is not available for language %q", voice, lang)
	}

	appCfg.Profiles[profileKey] = profile
	if appCfg.ActiveProvider == "" && appCfg.ActiveProfile == "" {
		appCfg.ActiveProvider = "gcp"
		appCfg.ActiveProfile = profileName
	}

	if err := saveConfig(appCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	runCheck, err := promptYesNo(reader, stdout, "Run sound check now? [Y/n]: ", true)
	if err != nil {
		return fmt.Errorf("read sound check choice: %w", err)
	}
	if runCheck {
		fmt.Fprintln(stdout, "Running sound check...")
		req := tts.SynthRequest{
			Text:          setupSoundCheckText,
			LanguageCode:  lang,
			VoiceName:     voice,
			AudioEncoding: tts.AudioEncodingMP3,
		}
		audioBytes, err := provider.SynthesizeRequest(ctx, req)
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
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Setup complete.")
	fmt.Fprintf(stdout, "Created profile: %s\n", profileKey)
	fmt.Fprintf(stdout, "Profile settings: voice=%s lang=%s apiKey=%s\n", voice, lang, maskAPIKey(apiKey))
	if appCfg.ActiveProvider == "gcp" && appCfg.ActiveProfile == profileName {
		fmt.Fprintf(stdout, "Profile set as active.\n")
	}
	fmt.Fprintf(stdout, "Config file: %s\n", cfgPath)
	return nil
}

func runDoctorCommand(stdout io.Writer) error {
	var checks []doctorCheck

	appCfg, err := loadConfig()
	if err != nil {
		checks = append(checks, doctorCheck{
			name:   "Config file",
			ok:     false,
			detail: err.Error(),
			hint:   "Check config file permissions and location.",
		})
	} else {
		checks = append(checks, doctorCheck{
			name:   "Config file",
			ok:     true,
			detail: "config file is readable",
		})
	}

	if len(appCfg.Profiles) == 0 {
		checks = append(checks, doctorCheck{
			name:   "Profiles",
			ok:     false,
			detail: "no profiles configured",
			hint:   "Run \"ttscli setup\" to create a profile.",
		})
		checks = append(checks, doctorCheck{
			name:   "Active profile",
			ok:     false,
			detail: "no active profile set",
			hint:   "Run \"ttscli profile use <provider:name>\" to set an active profile.",
		})
		checks = append(checks, doctorCheck{
			name:   "API connectivity",
			ok:     false,
			detail: "skipped: no profiles configured",
			hint:   "Create a profile first.",
		})
	} else {
		checks = append(checks, doctorCheck{
			name:   "Profiles",
			ok:     true,
			detail: fmt.Sprintf("%d profile(s) configured", len(appCfg.Profiles)),
		})

		activeProfileKey := appCfg.ActiveProvider + ":" + appCfg.ActiveProfile
		if appCfg.ActiveProvider == "" || appCfg.ActiveProfile == "" {
			checks = append(checks, doctorCheck{
				name:   "Active profile",
				ok:     false,
				detail: "no active profile set",
				hint:   "Run \"ttscli profile use <provider:name>\" to set an active profile.",
			})
			checks = append(checks, doctorCheck{
				name:   "API connectivity",
				ok:     false,
				detail: "skipped: no active profile",
				hint:   "Set an active profile first.",
			})
		} else {
			profile, profileErr := getProfile(appCfg, activeProfileKey)
			if profileErr != nil {
				checks = append(checks, doctorCheck{
					name:   "Active profile",
					ok:     false,
					detail: profileErr.Error(),
					hint:   "Run \"ttscli profile use <provider:name>\" to set a valid active profile.",
				})
				checks = append(checks, doctorCheck{
					name:   "API connectivity",
					ok:     false,
					detail: "skipped: invalid active profile",
					hint:   "Fix or change the active profile.",
				})
			} else {
				checks = append(checks, doctorCheck{
					name:   "Active profile",
					ok:     true,
					detail: fmt.Sprintf("%s (provider: %s)", activeProfileKey, profile.Provider),
				})

				provider, providerErr := newProvider(profile)
				if providerErr != nil {
					checks = append(checks, doctorCheck{
						name:   "Provider initialization",
						ok:     false,
						detail: providerErr.Error(),
						hint:   "Check profile credentials and configuration.",
					})
					checks = append(checks, doctorCheck{
						name:   "API connectivity",
						ok:     false,
						detail: "skipped: provider initialization failed",
						hint:   "Fix provider initialization errors first.",
					})
				} else {
					checks = append(checks, doctorCheck{
						name:   "Provider initialization",
						ok:     true,
						detail: fmt.Sprintf("%s provider initialized", provider.Name()),
					})

					ctx, stop := newAppCtx()
					testLang := cli.DefaultLanguage
					if lang, ok := profile.Defaults["lang"]; ok && lang != "" {
						testLang = lang
					}
					voices, err := provider.ListVoices(ctx, testLang)
					stop()
					if err != nil {
						checks = append(checks, doctorCheck{
							name:   "API connectivity",
							ok:     false,
							detail: err.Error(),
							hint:   "Verify API key, permissions, and service enablement.",
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
							detail: fmt.Sprintf("successfully listed %d voices for %s", len(voices), testLang),
						})
					}
				}
			}
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

func runProfileCommand(cfg cli.Config, stdout io.Writer) error {
	switch cfg.DefaultSubcommand {
	case cli.ProfileList:
		return runProfileList(stdout)
	case cli.ProfileCreate:
		return runProfileCreate(cfg, stdout)
	case cli.ProfileDelete:
		return runProfileDelete(cfg, stdout)
	case cli.ProfileUse:
		return runProfileUse(cfg, stdout)
	case cli.ProfileGet:
		return runProfileGet(cfg, stdout)
	default:
		return fmt.Errorf("unsupported profile subcommand: %s", cfg.DefaultSubcommand)
	}
}

func runProfileList(stdout io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if len(appCfg.Profiles) == 0 {
		fmt.Fprintln(stdout, "No profiles found. Create one with: ttscli profile create")
		return nil
	}

	fmt.Fprintln(stdout, "Available profiles:")
	fmt.Fprintln(stdout, "PROFILE KEY   | PROVIDER | NAME    | DEFAULT VOICE | DEFAULT LANG")
	fmt.Fprintln(stdout, "-------------------------------------------------------------------")
	for key, profile := range appCfg.Profiles {
		isActive := ""
		if profile.Provider == appCfg.ActiveProvider && profile.Name == appCfg.ActiveProfile {
			isActive = " (active)"
		}
		defaultVoice := profile.Defaults["voice"]
		if defaultVoice == "" {
			defaultVoice = "(not set)"
		}
		defaultLang := profile.Defaults["lang"]
		if defaultLang == "" {
			defaultLang = "(not set)"
		}
		fmt.Fprintf(stdout, "%-13s | %-8s | %-7s | %-13s | %s%s\n",
			key, profile.Provider, profile.Name, defaultVoice, defaultLang, isActive)
	}
	return nil
}

func runProfileCreate(cfg cli.Config, stdout io.Writer) error {
	if cfg.Lang == "" {
		return fmt.Errorf("--provider is required")
	}
	if cfg.Voice == "" {
		return fmt.Errorf("--name is required")
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("--api-key is required")
	}

	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	profileKey := cfg.Lang + ":" + cfg.Voice
	if _, exists := appCfg.Profiles[profileKey]; exists {
		return fmt.Errorf("profile %q already exists", profileKey)
	}

	profile := config.Profile{
		Provider: cfg.Lang,
		Name:     cfg.Voice,
		Credentials: map[string]interface{}{
			"apiKey": cfg.APIKey,
		},
		Defaults: make(map[string]string),
	}

	if cfg.SavePath != "" {
		profile.Defaults["voice"] = cfg.SavePath
	}

	provider, err := newProvider(profile)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	ctx, stop := newAppCtx()
	defer stop()

	voices, err := provider.ListVoices(ctx, "en-US")
	if err != nil {
		return fmt.Errorf("validate provider: %w", err)
	}
	if len(voices) == 0 {
		return fmt.Errorf("provider returned no voices, validation failed")
	}

	appCfg.Profiles[profileKey] = profile
	if appCfg.ActiveProvider == "" && appCfg.ActiveProfile == "" {
		appCfg.ActiveProvider = profile.Provider
		appCfg.ActiveProfile = profile.Name
	}

	if err := saveConfig(appCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(stdout, "Profile created: %s\n", profileKey)
	if profile.Provider == appCfg.ActiveProvider && profile.Name == appCfg.ActiveProfile {
		fmt.Fprintf(stdout, "Profile set as active.\n")
	}
	return nil
}

func runProfileDelete(cfg cli.Config, stdout io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if _, exists := appCfg.Profiles[cfg.Profile]; !exists {
		return fmt.Errorf("profile %q not found", cfg.Profile)
	}

	delete(appCfg.Profiles, cfg.Profile)

	if appCfg.ActiveProvider+":"+appCfg.ActiveProfile == cfg.Profile {
		appCfg.ActiveProvider = ""
		appCfg.ActiveProfile = ""
		for key, profile := range appCfg.Profiles {
			appCfg.ActiveProvider = profile.Provider
			appCfg.ActiveProfile = profile.Name
			fmt.Fprintf(stdout, "Switched active profile to: %s\n", key)
			break
		}
		if appCfg.ActiveProvider == "" {
			fmt.Fprintln(stdout, "No active profile set. Use 'ttscli profile use' to set one.")
		}
	}

	if err := saveConfig(appCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(stdout, "Profile deleted: %s\n", cfg.Profile)
	return nil
}

func runProfileUse(cfg cli.Config, stdout io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	parts := strings.Split(cfg.Profile, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid profile key format, expected provider:name (e.g., gcp:default)")
	}

	provider := parts[0]
	name := parts[1]
	profileKey := cfg.Profile

	if _, exists := appCfg.Profiles[profileKey]; !exists {
		return fmt.Errorf("profile %q not found", profileKey)
	}

	appCfg.ActiveProvider = provider
	appCfg.ActiveProfile = name

	if err := saveConfig(appCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(stdout, "Active profile set to: %s\n", profileKey)
	return nil
}

func runProfileGet(cfg cli.Config, stdout io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	profile, err := getProfile(appCfg, cfg.Profile)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Profile: %s\n", cfg.Profile)
	fmt.Fprintf(stdout, "Provider: %s\n", profile.Provider)
	fmt.Fprintf(stdout, "Name: %s\n", profile.Name)
	fmt.Fprintf(stdout, "API Key: %s\n", maskAPIKey(resolveProfileAPIKey(profile)))
	if voice, ok := profile.Defaults["voice"]; ok {
		fmt.Fprintf(stdout, "Default Voice: %s\n", voice)
	} else {
		fmt.Fprintln(stdout, "Default Voice: (not set)")
	}
	if lang, ok := profile.Defaults["lang"]; ok {
		fmt.Fprintf(stdout, "Default Language: %s\n", lang)
	} else {
		fmt.Fprintln(stdout, "Default Language: (not set)")
	}
	return nil
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
    COMPREPLY=( $(compgen -W "speak save voices setup doctor completion default --version --help" -- "${cur}") )
    return
  fi

  case "${words[1]}" in
    speak)
      COMPREPLY=( $(compgen -W "--text -t --lang -l --voice -v --help" -- "${cur}") )
      ;;
    save)
      COMPREPLY=( $(compgen -W "--text -t --out -o --lang -l --voice -v --help" -- "${cur}") )
      ;;
    voices)
      COMPREPLY=( $(compgen -W "--lang -l --help" -- "${cur}") )
      ;;
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish" -- "${cur}") )
      ;;
    default)
      if [[ ${cword} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "set get unset" -- "${cur}") )
      else
        case "${words[2]}" in
          set)
            COMPREPLY=( $(compgen -W "--voice -v --lang -l --api-key -k" -- "${cur}") )
            ;;
          unset)
            COMPREPLY=( $(compgen -W "--voice -v --lang -l --api-key -k" -- "${cur}") )
            ;;
        esac
      fi
      ;;
  esac
}

complete -F _ttscli_completion ttscli
`
}

func zshCompletionScript() string {
	return `#compdef ttscli

_ttscli() {
  local -a speak_flags
  local -a save_flags
  local -a voices_flags
  speak_flags=(
    '-t[Text to convert to speech]:text:'
    '--text[Text to convert to speech]:text:'
    '-l[Language code]:lang:'
    '--lang[Language code]:lang:'
    '-v[Voice name]:voice:'
    '--voice[Voice name]:voice:'
    '--help[Show help]'
  )
  save_flags=(
    '-t[Text to convert to speech]:text:'
    '--text[Text to convert to speech]:text:'
    '-o[Path to save MP3 output]:file:_files'
    '--out[Path to save MP3 output]:file:_files'
    '-l[Language code]:lang:'
    '--lang[Language code]:lang:'
    '-v[Voice name]:voice:'
    '--voice[Voice name]:voice:'
    '--help[Show help]'
  )
  voices_flags=(
    '-l[Language code]:lang:'
    '--lang[Language code]:lang:'
    '--help[Show help]'
  )

  if (( CURRENT == 2 )); then
    _describe 'command' \
      'speak:Synthesize speech' \
      'save:Synthesize and save MP3' \
      'voices:List available voices' \
      'setup:Run first-time setup' \
      'doctor:Run diagnostics' \
      'completion:Generate shell completions' \
      'default:Manage saved defaults' \
      '--version:Print version' \
      '--help:Show help'
    return
  fi

  case "$words[2]" in
    speak)
      _arguments -s $speak_flags
      ;;
    save)
      _arguments -s $save_flags
      ;;
    voices)
      _arguments -s $voices_flags
      ;;
    completion)
      _values 'shell' bash zsh fish
      ;;
    default)
      if (( CURRENT == 3 )); then
        _values 'subcommand' set get unset
      else
        case "$words[3]" in
          set|unset)
            _values 'flags' --voice -v --lang -l --api-key -k
            ;;
        esac
      fi
      ;;
  esac
}

_ttscli "$@"
`
}

func fishCompletionScript() string {
	return `# fish completion for ttscli
complete -c ttscli -f -n "__fish_use_subcommand" -a speak -d "Synthesize speech"
complete -c ttscli -f -n "__fish_use_subcommand" -a save -d "Synthesize and save MP3"
complete -c ttscli -f -n "__fish_use_subcommand" -a voices -d "List available voices"
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
complete -c ttscli -f -n "__fish_seen_subcommand_from set unset" -l voice -s v
complete -c ttscli -f -n "__fish_seen_subcommand_from set unset" -l lang -s l
complete -c ttscli -f -n "__fish_seen_subcommand_from set unset" -l api-key -s k

complete -c ttscli -f -n "__fish_seen_subcommand_from speak" -l text -s t -d "Text to convert to speech"
complete -c ttscli -f -n "__fish_seen_subcommand_from speak" -l lang -s l -d "Language code"
complete -c ttscli -f -n "__fish_seen_subcommand_from speak" -l voice -s v -d "Voice name"
complete -c ttscli -f -n "__fish_seen_subcommand_from save" -l text -s t -d "Text to convert to speech"
complete -c ttscli -f -n "__fish_seen_subcommand_from save" -l out -s o -d "Path to save MP3 output"
complete -c ttscli -f -n "__fish_seen_subcommand_from save" -l lang -s l -d "Language code"
complete -c ttscli -f -n "__fish_seen_subcommand_from save" -l voice -s v -d "Voice name"
complete -c ttscli -f -n "__fish_seen_subcommand_from voices" -l lang -s l -d "Language code"
complete -c ttscli -f -n "__fish_seen_subcommand_from speak save voices" -l help -d "Show help"
`
}
