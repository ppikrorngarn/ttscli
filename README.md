# Multi-Provider Text-to-Speech CLI (`ttscli`)

A lightweight, fast Command Line Interface (CLI) written in Go to convert text into speech using multiple cloud TTS providers. Currently supports Google Cloud, with planned support for AWS, Azure, IBM Cloud, and Alibaba Cloud.

This tool allows you to easily synthesize speech, save it to an MP3 file, or play it directly from your terminal on macOS, Linux, and Windows.

## Prerequisites

1. **Go:** Install Go `1.25+` (matches `go.mod`).
2. **Cloud Provider API Key:** You need an API key for your chosen TTS provider.
   - **Google Cloud:** Go to Google Cloud Console > APIs & Services > Credentials. Create an API Key and restrict it to the "Cloud Text-to-Speech API".
   - **Other providers:** Support coming soon.
3. **Audio Player (Linux Only):** If you are running on Linux and want to use the `speak` command (which plays audio), you need an audio player installed. The CLI looks for `mpg123`, `paplay`, or `ffplay`.
   - Ubuntu/Debian: `sudo apt install mpg123`
4. **Staticcheck (for local quality checks):** Optional but recommended for contributors.
   - Install: `go install honnef.co/go/tools/cmd/staticcheck@v0.7.0`

## Setup

1. **Install the CLI:**
   ```bash
   go install github.com/ppikrorngarn/ttscli/cmd/ttscli@latest
   ```
   This installs the `ttscli` binary into your Go bin directory.
   If `GOBIN` is set, Go installs there. Otherwise it uses:
   ```bash
   $(go env GOPATH)/bin
   ```

   You can inspect `GOBIN` with:
   ```bash
   $(go env GOBIN)
   ```

   Make sure that directory is in your `PATH`.

   Use the same Go bin directory in your `PATH`.
   If `GOBIN` is empty, that usually means `$(go env GOPATH)/bin`.

   Temporary shell setup for the current session:
   ```bash
   export PATH="<go-bin-dir>:$PATH"
   ```

   To make that permanent on macOS/Linux, add the same line to your shell profile such as `~/.zshrc`, `~/.bashrc`, or `~/.profile`.

   Temporary PowerShell example for the current session:
   ```powershell
   $env:Path += ";<go-bin-dir>"
   ```

   To make that permanent on Windows, add the directory to your user `Path` environment variable in System Settings, or persist it in your PowerShell profile.

   Then verify the install:
   ```bash
   ttscli --version
   ```

2. **Or clone the repository and build the binary locally:**
   You can use the provided Makefile to build the project easily:
   ```bash
   make build
   ```
   *(Or run `go build -o ttscli ./cmd/ttscli` directly).*

   If you want to install that locally built binary into your personal bin directory:
   ```bash
   mkdir -p ~/.local/bin
   install -m 0755 ttscli ~/.local/bin/ttscli
   export PATH="$HOME/.local/bin:$PATH"
   ```

   The `export PATH=...` line only affects the current shell session.
   To make it permanent on macOS/Linux, add it to your shell profile such as `~/.zshrc`, `~/.bashrc`, or `~/.profile`.

   Windows PowerShell example:
   ```powershell
   New-Item -ItemType Directory -Force "$HOME\bin" | Out-Null
   Copy-Item .\ttscli.exe "$HOME\bin\ttscli.exe"
   $env:Path += ";" + "$HOME\bin"
   ```

   The `$env:Path += ...` line only affects the current PowerShell session.
   To make it permanent on Windows, add `%USERPROFILE%\bin` to your user `Path` environment variable in System Settings, or persist it in your PowerShell profile.

   Then verify it:
   ```bash
   ttscli --version
   ```

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

   To make that persistent on macOS/Linux, add it to your shell profile.
   On Windows, set it as a user environment variable instead of relying on a session-only shell command.

### Contributor Setup

For local quality checks:

```bash
make tools
export PATH="<go-bin-dir>:$PATH"
make check
```

Use the same Go bin directory that `go install` writes to.
If `GOBIN` is empty, that usually means `$(go env GOPATH)/bin`.

The `export PATH=...` line above only affects the current shell session.
If you use these tools often, add the same line to your shell profile.

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
If you manually copied the binary into a personal bin directory such as `~/.local/bin`, run `ttscli ...` after adding that directory to your `PATH`.
On Windows, the equivalent is typically `ttscli.exe` from a directory such as `%USERPROFILE%\bin` after adding it to `Path`.

### Profile System

`ttscli` uses a profile-based configuration system that allows you to manage multiple TTS provider configurations. Each profile contains:

- **Provider**: The TTS service provider (e.g., `gcp`, `aws`, `azure`)
- **Name**: A unique name for the profile (e.g., `default`, `work`, `personal`)
- **Credentials**: Provider-specific authentication (e.g., API keys)
- **Defaults**: Default language and voice settings for that profile

**Profile Key Format**: `{provider}:{name}` (e.g., `gcp:default`, `aws:work`)

**Profile Resolution**:
1. `--profile` flag or `TTSCLI_PROFILE` environment variable
2. Active profile from config
3. First available profile
4. Error if no profiles exist

**Current Provider Support**:
- ✅ **Google Cloud (GCP)**: Fully implemented
- 🚧 **AWS Polly**: Planned
- 🚧 **Azure Speech**: Planned
- 🚧 **IBM Watson**: Planned
- 🚧 **Alibaba Cloud**: Planned

### Backward Compatibility

For users upgrading from previous versions, `ttscli` maintains backward compatibility with the legacy `default` command system. The `setup` command creates both a new profile and saves legacy defaults, ensuring existing workflows continue to work.

### Command Reference

- `ttscli speak`: Synthesize text to speech and play it immediately.
- `ttscli save`: Synthesize text to speech and save MP3 output.
- `ttscli voices`: List available voices (optionally filter with `--lang`).
- `ttscli setup`: Interactive first-run setup (creates a GCP profile).
- `ttscli doctor`: Health checks (config, profiles, API connectivity, playback). Returns non-zero when checks fail.
- `ttscli completion <bash|zsh|fish>`: Prints shell completion script to stdout.
- `ttscli profile list`: List all configured profiles.
- `ttscli profile create`: Create a new profile with provider credentials.
- `ttscli profile delete <provider:name>`: Delete a profile.
- `ttscli profile use <provider:name>`: Set the active profile.
- `ttscli profile get <provider:name>`: Show profile details.
- `ttscli default set|get|unset`: Manage saved defaults (legacy, for backward compatibility).
- `ttscli --version`: Print build metadata.
- `ttscli --help`: Show top-level help.

### `speak` Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--text` | string | `""` | Required for `speak`. Alias: `-t`. |
| `--lang` | string | `en-US` | Language code for speak (or profile default). Alias: `-l`. |
| `--voice` | string | `en-US-Neural2-F` | Voice name for synth (or profile default). Alias: `-v`. |
| `--profile` | string | `""` | Profile to use (e.g., `gcp:default`). Alias: `-p`. |

### `save` Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--text` | string | `""` | Required for `save`. Alias: `-t`. |
| `--out` | string | `""` | Required output MP3 path. Alias: `-o`. |
| `--lang` | string | `en-US` | Language code for save (or profile default). Alias: `-l`. |
| `--voice` | string | `en-US-Neural2-F` | Voice name for save (or profile default). Alias: `-v`. |
| `--profile` | string | `""` | Profile to use (e.g., `gcp:default`). Alias: `-p`. |

### `voices` Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--lang` | string | `en-US` | Optional language filter for voice listing. Alias: `-l`. |
| `--profile` | string | `""` | Profile to use (e.g., `gcp:default`). Alias: `-p`. |

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

**3. Manage profiles:**
```bash
# List all profiles
ttscli profile list

# Create a new profile
ttscli profile create --provider gcp --name work --api-key YOUR_API_KEY

# Set active profile
ttscli profile use gcp:work

# View profile details
ttscli profile get gcp:work

# Delete a profile
ttscli profile delete gcp:work
```

**4. Generate shell completions:**
```bash
ttscli completion zsh
```

**5. Play audio immediately (without saving):**
```bash
# Using active profile
ttscli speak --text "Hello world, this is a test."

# Using specific profile
ttscli speak --text "Hello world, this is a test." --profile gcp:work
```

**6. Save audio to a file:**
```bash
# Using active profile
ttscli save --text "Save this to a file." --out output.mp3

# Using specific profile
ttscli save --text "Save this to a file." --out output.mp3 --profile gcp:work
```

**7. List voices:**
```bash
# Using active profile
ttscli voices --lang en-GB

# Using specific profile
ttscli voices --lang en-GB --profile gcp:work
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
  if `config.json` exists next to the `ttscli` binary, it is used first; otherwise use the user config path.
  Examples:
  macOS: `~/Library/Application Support/ttscli/config.json`
  Linux: `~/.config/ttscli/config.json`
  Windows: `%AppData%\ttscli\config.json`
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
For command-specific flags, use:
```bash
ttscli speak --help
ttscli save --help
ttscli default set --help
```
Short aliases are available for the most common flags, such as `-t`, `-o`, `-l`, `-v`, and `-k`.
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
# bash (Linux, system-wide example; may require sudo and varies by distro)
ttscli completion bash > /etc/bash_completion.d/ttscli
# start a new shell, or source your distro's bash-completion setup if available
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

- `ttscli: command not found`:
  make sure the binary directory is on your `PATH`.
  Common locations are `$(go env GOPATH)/bin`, `$(go env GOBIN)`, `~/.local/bin`, or `%USERPROFILE%\bin`.
- missing API key errors:
  save a key with `ttscli default set --api-key ...`, run `ttscli setup`, or set `TTSCLI_GOOGLE_API_KEY` in your environment.
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
