# GCP Text-to-Speech CLI (`ttscli`)

A lightweight, fast Command Line Interface (CLI) written in Go to convert text into speech using the Google Cloud Text-to-Speech API. 

This tool allows you to easily synthesize speech, save it to an MP3 file, or play it directly from your terminal on macOS, Linux, and Windows.

## Prerequisites

1. **Go:** Install Go `1.25+` (matches `go.mod`).
2. **Google Cloud API Key:** You need an API key for the Google Cloud Text-to-Speech API.
   - Go to Google Cloud Console > APIs & Services > Credentials.
   - Create an API Key and restrict it to the "Cloud Text-to-Speech API".
3. **Audio Player (Linux Only):** If you are running on Linux and want to use the `speak` command (which plays audio), you need an audio player installed. The CLI looks for `mpg123`, `paplay`, or `ffplay`.
   - Ubuntu/Debian: `sudo apt install mpg123`
4. **Staticcheck (for local quality checks):** Optional but recommended for contributors.
   - Install: `go install honnef.co/go/tools/cmd/staticcheck@v0.7.0`

## Setup

1. **Install the CLI:**
   ```bash
   go install github.com/ppikrorngarn/ttscli/cmd/ttscli@latest
   ```
   Make sure `$(go env GOPATH)/bin` is in your `PATH`.

2. **Or clone the repository and build the binary locally:**
   You can use the provided Makefile to build the project easily:
   ```bash
   make build
   ```
   *(Or run `go build -o ttscli ./cmd/ttscli` directly).*

3. **Run first-time setup (recommended):**
   ```bash
   ttscli setup
   ```
   This guided flow prompts for API key, default language, and default voice, validates them, and can run a sound check.
   Press Enter on language/voice prompts to use built-in defaults: `en-US` and `en-US-Neural2-F`.

4. **Manual API key setup (alternative):**
   Save it once with:
   ```bash
   ttscli default set --api-key "your_api_key_here"
   ```
   Or export it in your terminal for the current shell/session:
   ```bash
   export TTSCLI_GOOGLE_API_KEY="your_api_key_here"
   ```

### Contributor Setup

For local quality checks:

```bash
make tools
export PATH="$(go env GOPATH)/bin:$PATH"
make check
```

For contribution and licensing details, see:
- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [LICENSE](./LICENSE)

### Development Checks

Run these during development:

```bash
make test
make test-race
make lint
make check
```

## Usage

If you installed via `go install`, run `ttscli ...` from your shell `PATH`.
If you built locally with `make build`, run `./ttscli ...` from the repo root.

### Command Reference

- `ttscli speak`: Synthesize text to speech and play it immediately.
- `ttscli save`: Synthesize text to speech and save MP3 output.
- `ttscli voices`: List available voices (optionally filter with `--lang`).
- `ttscli setup`: Interactive first-run setup (API key + defaults + optional sound check).
- `ttscli doctor`: Health checks (config, API key, API connectivity, playback). Returns non-zero when checks fail.
- `ttscli completion <bash|zsh|fish>`: Prints shell completion script to stdout.
- `ttscli default set|get|unset`: Manage saved defaults (`voice`, `lang`, `apiKey`).
- `ttscli --version`: Print build metadata.
- `ttscli --help`: Show top-level help.

### `speak` Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--text` | string | `""` | Required for `speak`. Alias: `-t`. |
| `--lang` | string | `en-US` | Language code for speak (or saved default, if configured). Alias: `-l`. |
| `--voice` | string | `en-US-Neural2-F` | Voice name for synth (or saved default, if configured). Alias: `-v`. |

### `save` Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--text` | string | `""` | Required for `save`. Alias: `-t`. |
| `--out` | string | `""` | Required output MP3 path. Alias: `-o`. |
| `--lang` | string | `en-US` | Language code for save (or saved default, if configured). Alias: `-l`. |
| `--voice` | string | `en-US-Neural2-F` | Voice name for save (or saved default, if configured). Alias: `-v`. |

### `voices` Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--lang` | string | `en-US` | Optional language filter for voice listing. Alias: `-l`. |

### `default set` / `default unset` Flags

| Flag | Applies To | Notes |
| --- | --- | --- |
| `--voice` | `set`, `unset` | Voice default value or selector. Alias: `-v`. |
| `--lang` | `set`, `unset` | Language default value or selector. Alias: `-l`. |
| `--api-key` | `set`, `unset` | API key default value or selector. Alias: `-k`. |

### Basic Commands

**0. Show CLI version/build metadata:**
```bash
ttscli --version
```

**1. Run setup wizard (first run):**
```bash
ttscli setup
```

**2. Run diagnostics:**
```bash
ttscli doctor
```

**3. Generate shell completions:**
```bash
ttscli completion zsh
```

**4. Play audio immediately (without saving):**
```bash
ttscli speak --text "Hello world, this is a test."
```

**5. Save audio to a file:**
```bash
ttscli save --text "Save this to a file." --out output.mp3
```

### Alias Quick Examples

```bash
ttscli speak -t "Hello world, this is a test."
ttscli save -t "Save this to a file." -o output.mp3
ttscli voices -l en-GB
ttscli default set -v en-US-Chirp3-HD-Achernar -l en-US
ttscli default unset -v -k
```

### Voice and Language Selection

By default, the CLI uses a female US English voice (`en-US-Neural2-F`). You can customize this using the `--lang` and `--voice` flags.

**Example: British Male Voice:**
```bash
ttscli speak --text "Hello there, how are you doing today?" --lang "en-GB" --voice "en-GB-Neural2-B"
```

**Example: French Voice:**
```bash
ttscli speak --text "Bonjour le monde" --lang "fr-FR" --voice "fr-FR-Neural2-A"
```

### Listing Available Voices

You can list all available voices for a specific language directly from the API.

**List US English voices (default):**
```bash
ttscli voices
```

**List voices for another language:**
```bash
ttscli voices --lang en-GB
```

### Persistent Defaults (NVM-like)

Set user-level defaults:

```bash
ttscli default set --voice en-US-Chirp3-HD-Achernar --lang en-US
```

Set a saved API key:

```bash
ttscli default set --api-key "your_api_key_here"
```

Set only one field (partial update):

```bash
ttscli default set --voice en-US-Neural2-F
```

Show current defaults:

```bash
ttscli default get
```

Clear saved defaults:

```bash
ttscli default unset
```

### Behavior Notes

- `speak` requires `--text` and always plays audio.
- `save` requires `--text` and `--out`.
- `voices` does not require `--text`.
- Press `Ctrl+C` to cancel in-flight API work gracefully.
- On Linux, playback command priority is: `mpg123`, then `paplay`, then `ffplay`.
- Config lookup priority:
  if `config.json` exists next to the `ttscli` binary, it is used first; otherwise use the user config path (for example on macOS: `~/Library/Application Support/ttscli/config.json`).
- Priority for synth/list language and voice:
  explicit flags (`--voice`, `--lang`) > saved defaults (`ttscli default ...`) > built-in defaults.
- Priority for API key:
  `TTSCLI_GOOGLE_API_KEY` (env) > saved API key (`ttscli default set --api-key ...`).
- `default set` validates against Google TTS before saving.
- `doctor` runs health checks for config readability, API key availability, API connectivity, and playback capability.

## Help

For a full list of commands and flags, use:
```bash
ttscli --help
```
If you run `ttscli` with no arguments, it will print a hint to run `--help`.

## Shell Completion

Generate completion scripts:

```bash
ttscli completion bash
ttscli completion zsh
ttscli completion fish
```

Install examples:

```bash
# bash (Linux)
ttscli completion bash > /etc/bash_completion.d/ttscli
source /etc/bash_completion
```

```bash
# zsh (macOS/Linux)
mkdir -p ~/.zsh/completions
ttscli completion zsh > ~/.zsh/completions/_ttscli
# ensure ~/.zsh/completions is in fpath, then:
autoload -Uz compinit && compinit
```

```bash
# fish
mkdir -p ~/.config/fish/completions
ttscli completion fish > ~/.config/fish/completions/ttscli.fish
# open a new shell (or run: exec fish)
```

## Troubleshooting

- `TTSCLI_GOOGLE_API_KEY environment variable is not set`:
  set `TTSCLI_GOOGLE_API_KEY` in your shell, or save it with `ttscli default set --api-key ...`.
- `no supported audio player found on Linux`:
  install one of `mpg123`, `paplay`, or `ffplay`.
- `failed to synthesize: status=... body=...`:
  verify API key validity, API enablement, and key restrictions for Cloud Text-to-Speech API.
- `dial tcp: lookup texttospeech.googleapis.com: no such host`:
  this is a DNS/network issue in your environment; verify internet/DNS and retry.
- `voice "..." is not available for language "..."` when running `default set`:
  use `ttscli voices --lang <lang>` to find valid voice names.

## Project Structure

- `cmd/ttscli`: CLI entrypoint (`main`, `--version` handling).
- `internal/app`: top-level application flow and dependency wiring (split across `run.go`, `command.go`, `defaults.go`, and `util.go`).
- `internal/cli`: flag parsing and argument validation.
- `internal/config`: user-level persisted defaults (`voice`, `lang`, `apiKey`).
- `internal/tts`: Google TTS client and response parsing.
- `internal/player`: local audio playback across OS platforms.

## Makefile Commands

A `Makefile` is included to simplify common development tasks.

- `make build` - Compiles the Go binary into `ttscli` with version metadata (`VERSION`, `COMMIT`, `DATE`).
- `make clean` - Removes the compiled binary and any generated `.mp3` files in the directory.
- `make run ARGS="..."` - Builds and runs the CLI, passing arguments to it (e.g., `make run ARGS="voices --lang en-GB"`).
- `make test` - Runs Go tests.
- `make test-race` - Runs tests with the race detector.
- `make tools` - Installs pinned local developer tools (currently `staticcheck`).
- `make lint` - Runs `staticcheck`.
- `make check` - Runs `go vet`, tests, race tests, and `staticcheck`.
- `make help` - Displays a list of all available make commands.

### Build Metadata Example

To build a release-like binary with explicit metadata:

```bash
make build VERSION=v0.1.0 COMMIT=$(git rev-parse --short HEAD) DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
```

Sample version output:

```bash
ttscli --version
# ttscli version=v0.1.0 commit=abc1234 date=2026-04-12T12:00:00Z
```

Without explicit build metadata, local binaries show defaults (`version=dev`, `commit=none`, `date=unknown`).

## Releases

- Pushing a tag like `v0.1.0` triggers automated multi-platform releases via GitHub Actions + GoReleaser.
- Artifacts are built for Linux, macOS, and Windows.
- Release binaries include embedded version metadata visible from `--version`.

## Project Policies

- License: [LICENSE](./LICENSE)
- Contributing guide: [CONTRIBUTING.md](./CONTRIBUTING.md)
