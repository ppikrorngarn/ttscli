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

2. **Or download a pre-built binary from GitHub Releases:**

   Each tagged release (matching `v*.*.*`) produces archives built by GoReleaser. The following combinations are published:

   | OS      | Architectures | Archive format |
   |---------|---------------|----------------|
   | Linux   | amd64, arm64  | `.tar.gz`      |
   | macOS   | amd64, arm64  | `.tar.gz`      |
   | Windows | amd64, arm64  | `.zip`         |

   A `checksums.txt` file is published alongside each release for verification. Extract the archive, place the `ttscli` binary somewhere on your `PATH`, and verify with `ttscli --version`.

3. **Or clone the repository and build the binary locally:**
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

4. **Run first-time setup (recommended):**
   ```bash
   ttscli setup
   ```
    This guided flow prompts for API key, default language, and default voice, validates them, and can run a sound check.
    Press Enter on language/voice prompts to use built-in defaults: `en-US` and `en-US-Neural2-F`.

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

The `Makefile` provides the following targets:

| Target           | Description                                                       |
|------------------|-------------------------------------------------------------------|
| `make build`     | Build the `ttscli` binary with version metadata injected via ldflags |
| `make clean`     | Remove the compiled binary and any `*.mp3` files                 |
| `make run`       | Build and run the CLI (pass flags via `ARGS="..."`)              |
| `make test`      | Run all tests with verbose output                                |
| `make test-race` | Run all tests with the race detector                             |
| `make tools`     | Install local developer tools (staticcheck)                      |
| `make lint`      | Run static analysis (staticcheck)                                |
| `make check`     | Run `go vet`, tests, race tests, and lint                        |
| `make help`      | Print all available targets with their descriptions              |

Quick examples:

```bash
make build
make run ARGS="--version"
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
1. `--profile` flag
2. Active profile from config
3. First available profile
4. Error if no profiles exist

**Current Provider Support**:
- ✅ **Google Cloud (GCP)**: Fully implemented
- 🚧 **AWS Polly**: Planned
- 🚧 **Azure Speech**: Planned
- 🚧 **IBM Watson**: Planned
- 🚧 **Alibaba Cloud**: Planned

### Configuration File

`ttscli` stores its profiles in a JSON file. The CLI resolves the config path in two steps:

1. **Next to the binary** — if a `config.json` sits in the same directory as the `ttscli` executable, it takes priority. Useful for portable or pinned setups.
2. **User config directory (fallback):**

| OS      | Path                                                 |
|---------|------------------------------------------------------|
| macOS   | `~/Library/Application Support/ttscli/config.json`   |
| Linux   | `~/.config/ttscli/config.json`                       |
| Windows | `%AppData%\ttscli\config.json`                       |

**Config file format:**

```json
{
  "activeProvider": "gcp",
  "activeProfile": "default",
  "profiles": {
    "gcp:default": {
      "provider": "gcp",
      "name": "default",
      "credentials": {
        "apiKey": "YOUR_API_KEY"
      },
      "defaults": {
        "lang": "en-US",
        "voice": "en-US-Neural2-F"
      }
    }
  }
}
```

Prefer managing profiles via the `ttscli profile` subcommands (see [Command Reference](#command-reference)) rather than editing this file by hand.


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

## Project Structure

Module path: `github.com/ppikrorngarn/ttscli`

The codebase is organized into a thin `cmd/` entry point and focused `internal/` packages:

```
cmd/ttscli/          Binary entry point. Injects version/commit/date via ldflags at build time.
internal/cli/        Argument parsing. Owns the Config struct and all mode/subcommand constants.
internal/app/        Command handlers (setup, doctor, profile, completion). Orchestrates the run
                     flow: parse args -> resolve profile -> create provider -> execute.
internal/config/     JSON config file I/O. Profile CRUD. Two-step path resolution (local next
                     to the binary, falling back to the user config directory).
internal/player/     Cross-platform audio playback. macOS: afplay. Linux: mpg123/paplay/ffplay.
                     Windows: PowerShell Media.SoundPlayer.
internal/tts/        Provider interface, GCP HTTP client, voice listing, PrintVoices helper.
docs/                Documentation.
```

Packages under `internal/` use small, package-level function variables (e.g. `readFile`, `writeFile`, `lookupEnv`) for dependency injection, so tests can stub the filesystem, environment, and HTTP calls without real I/O.

