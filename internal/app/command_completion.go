package app

import (
	"fmt"
	"io"

	"github.com/ppikrorngarn/ttscli/internal/cli"
)

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

func bashCompletionScript() string {
	return `# bash completion for ttscli
_ttscli_completion() {
  local cur words cword
  words=("${COMP_WORDS[@]}")
  cword=$COMP_CWORD
  cur="${COMP_WORDS[COMP_CWORD]}"

  if [[ ${cword} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "speak save voices setup doctor completion profile --version --help" -- "${cur}") )
    return
  fi

  case "${words[1]}" in
    speak)
      COMPREPLY=( $(compgen -W "--text -t --lang -l --voice -v --profile -p --help" -- "${cur}") )
      ;;
    save)
      COMPREPLY=( $(compgen -W "--text -t --out -o --lang -l --voice -v --profile -p --help" -- "${cur}") )
      ;;
    voices)
      COMPREPLY=( $(compgen -W "--lang -l --profile -p --help" -- "${cur}") )
      ;;
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish" -- "${cur}") )
      ;;
    profile)
      if [[ ${cword} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "list create delete use get" -- "${cur}") )
      else
        case "${words[2]}" in
          create)
            COMPREPLY=( $(compgen -W "--provider -P --name -n --api-key -k --voice -v" -- "${cur}") )
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
  local -a speak_flags save_flags voices_flags profile_create_flags
  speak_flags=(
    '-t[Text to convert to speech]:text:'
    '--text[Text to convert to speech]:text:'
    '-l[Language code]:lang:'
    '--lang[Language code]:lang:'
    '-v[Voice name]:voice:'
    '--voice[Voice name]:voice:'
    '-p[Profile to use]:profile:'
    '--profile[Profile to use]:profile:'
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
    '-p[Profile to use]:profile:'
    '--profile[Profile to use]:profile:'
    '--help[Show help]'
  )
  voices_flags=(
    '-l[Language code]:lang:'
    '--lang[Language code]:lang:'
    '-p[Profile to use]:profile:'
    '--profile[Profile to use]:profile:'
    '--help[Show help]'
  )
  profile_create_flags=(
    '-P[Provider name]:provider:'
    '--provider[Provider name]:provider:'
    '-n[Profile name]:name:'
    '--name[Profile name]:name:'
    '-k[API key]:key:'
    '--api-key[API key]:key:'
    '-v[Default voice]:voice:'
    '--voice[Default voice]:voice:'
  )

  if (( CURRENT == 2 )); then
    _describe 'command' \
      'speak:Synthesize speech' \
      'save:Synthesize and save MP3' \
      'voices:List available voices' \
      'setup:Run first-time setup' \
      'doctor:Run diagnostics' \
      'completion:Generate shell completions' \
      'profile:Manage TTS provider profiles' \
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
    profile)
      if (( CURRENT == 3 )); then
        _values 'subcommand' list create delete use get
      else
        case "$words[3]" in
          create)
            _arguments -s $profile_create_flags
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
complete -c ttscli -f -n "__fish_use_subcommand" -a profile -d "Manage TTS provider profiles"
complete -c ttscli -f -n "__fish_use_subcommand" -l version -d "Print version and exit"

complete -c ttscli -f -n "__fish_seen_subcommand_from completion" -a bash
complete -c ttscli -f -n "__fish_seen_subcommand_from completion" -a zsh
complete -c ttscli -f -n "__fish_seen_subcommand_from completion" -a fish

complete -c ttscli -f -n "__fish_seen_subcommand_from profile; and not __fish_seen_subcommand_from list create delete use get" -a list -d "List all profiles"
complete -c ttscli -f -n "__fish_seen_subcommand_from profile; and not __fish_seen_subcommand_from list create delete use get" -a create -d "Create a new profile"
complete -c ttscli -f -n "__fish_seen_subcommand_from profile; and not __fish_seen_subcommand_from list create delete use get" -a delete -d "Delete a profile"
complete -c ttscli -f -n "__fish_seen_subcommand_from profile; and not __fish_seen_subcommand_from list create delete use get" -a use -d "Set active profile"
complete -c ttscli -f -n "__fish_seen_subcommand_from profile; and not __fish_seen_subcommand_from list create delete use get" -a get -d "Show profile details"

complete -c ttscli -f -n "__fish_seen_subcommand_from create" -l provider -s P -d "Provider name"
complete -c ttscli -f -n "__fish_seen_subcommand_from create" -l name -s n -d "Profile name"
complete -c ttscli -f -n "__fish_seen_subcommand_from create" -l api-key -s k -d "API key"
complete -c ttscli -f -n "__fish_seen_subcommand_from create" -l voice -s v -d "Default voice"

complete -c ttscli -f -n "__fish_seen_subcommand_from speak" -l text -s t -d "Text to convert to speech"
complete -c ttscli -f -n "__fish_seen_subcommand_from speak" -l lang -s l -d "Language code"
complete -c ttscli -f -n "__fish_seen_subcommand_from speak" -l voice -s v -d "Voice name"
complete -c ttscli -f -n "__fish_seen_subcommand_from speak" -l profile -s p -d "Profile to use"
complete -c ttscli -f -n "__fish_seen_subcommand_from save" -l text -s t -d "Text to convert to speech"
complete -c ttscli -f -n "__fish_seen_subcommand_from save" -l out -s o -d "Path to save MP3 output"
complete -c ttscli -f -n "__fish_seen_subcommand_from save" -l lang -s l -d "Language code"
complete -c ttscli -f -n "__fish_seen_subcommand_from save" -l voice -s v -d "Voice name"
complete -c ttscli -f -n "__fish_seen_subcommand_from save" -l profile -s p -d "Profile to use"
complete -c ttscli -f -n "__fish_seen_subcommand_from voices" -l lang -s l -d "Language code"
complete -c ttscli -f -n "__fish_seen_subcommand_from voices" -l profile -s p -d "Profile to use"
complete -c ttscli -f -n "__fish_seen_subcommand_from speak save voices" -l help -d "Show help"
`
}
