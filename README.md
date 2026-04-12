# GCP Text-to-Speech CLI (`ttscli`)

A lightweight, fast Command Line Interface (CLI) written in Go to convert text into speech using the Google Cloud Text-to-Speech API. 

This tool allows you to easily synthesize speech, save it to an MP3 file, or play it directly from your terminal on macOS, Linux, and Windows.

## Prerequisites

1. **Go:** Make sure you have Go installed on your machine.
2. **Google Cloud API Key:** You need an API key for the Google Cloud Text-to-Speech API.
   - Go to Google Cloud Console > APIs & Services > Credentials.
   - Create an API Key and restrict it to the "Cloud Text-to-Speech API".
3. **Audio Player (Linux Only):** If you are running on Linux and want to use the `--play` flag, you need an audio player installed. The CLI looks for `mpg123`, `paplay`, or `ffplay`.
   - Ubuntu/Debian: `sudo apt install mpg123`
4. **Staticcheck (for local quality checks):** Optional but recommended for contributors.
   - Install: `go install honnef.co/go/tools/cmd/staticcheck@latest`

## Setup

1. **Install the CLI:**
   ```bash
   go install github.com/ppikrorngarn/ttscli/cmd/ttscli@latest
   ```

2. **Or clone the repository and build the binary locally:**
   You can use the provided Makefile to build the project easily:
   ```bash
   make build
   ```
   *(Or run `go build -o ttscli ./cmd/ttscli` directly).*

3. **Configure your API Key:**
   Create a `.env` file in the root of the project and add your API key:
   ```env
   TTSCLI_GOOGLE_API_KEY="your_api_key_here"
   ```
   *(Alternatively, you can export it in your terminal: `export TTSCLI_GOOGLE_API_KEY="your_api_key_here"`)*

## Usage

You can run the CLI by executing the `./ttscli` binary. 

### Flags Reference

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--text` | string | `""` | Required for synth mode (not required with `--list-voices` or `--version`). |
| `--save` | string | `""` | Output MP3 path. |
| `--play` | bool | `false` | Play synthesized audio immediately. |
| `--lang` | string | `en-US` | Language code for synth/list. |
| `--voice` | string | `en-US-Neural2-F` | Voice name for synth. |
| `--list-voices` | bool | `false` | Lists voices (optionally filtered by `--lang`). |
| `--version` | bool | `false` | Prints build metadata and exits. |

### Basic Commands

**0. Show CLI version/build metadata:**
```bash
./ttscli --version
```

**1. Play audio immediately (without saving):**
```bash
./ttscli --text "Hello world, this is a test." --play
```

**2. Save audio to a file:**
```bash
./ttscli --text "Save this to a file." --save output.mp3
```

**3. Save and play:**
```bash
./ttscli --text "Save and play." --save output.mp3 --play
```

### Voice and Language Selection

By default, the CLI uses a female US English voice (`en-US-Neural2-F`). You can customize this using the `--lang` and `--voice` flags.

**Example: British Male Voice:**
```bash
./ttscli --text "Hello there, how are you doing today?" --lang "en-GB" --voice "en-GB-Neural2-B" --play
```

**Example: French Voice:**
```bash
./ttscli --text "Bonjour le monde" --lang "fr-FR" --voice "fr-FR-Neural2-A" --play
```

### Listing Available Voices

You can list all available voices for a specific language directly from the API.

**List US English voices (default):**
```bash
./ttscli --list-voices
```

**List voices for another language:**
```bash
./ttscli --list-voices --lang en-GB
```

### Behavior Notes

- Synthesize mode requires `--text` and at least one output mode: `--save` or `--play`.
- `--list-voices` bypasses synth validation and does not require `--text`.
- Press `Ctrl+C` to cancel in-flight API work gracefully.
- On Linux, playback command priority is: `mpg123`, then `paplay`, then `ffplay`.

## Help

For a full list of flags, use the `--help` command:
```bash
./ttscli --help
```

## Troubleshooting

- `TTSCLI_GOOGLE_API_KEY environment variable is not set`:
  set `TTSCLI_GOOGLE_API_KEY` in your shell or `.env` file.
- `no supported audio player found on Linux`:
  install one of `mpg123`, `paplay`, or `ffplay`.
- `failed to synthesize: status=... body=...`:
  verify API key validity, API enablement, and key restrictions for Cloud Text-to-Speech API.

## Project Structure

- `cmd/ttscli`: CLI entrypoint (`main`, `--version` handling).
- `internal/app`: top-level application flow and dependency wiring.
- `internal/cli`: flag parsing and argument validation.
- `internal/tts`: Google TTS client and response parsing.
- `internal/player`: local audio playback across OS platforms.

## Makefile Commands

A `Makefile` is included to simplify common development tasks.

- `make build` - Compiles the Go binary into `ttscli` with version metadata (`VERSION`, `COMMIT`, `DATE`).
- `make clean` - Removes the compiled binary and any generated `.mp3` files in the directory.
- `make run ARGS="..."` - Builds and runs the CLI, passing arguments to it (e.g., `make run ARGS="--list-voices"`).
- `make test` - Runs Go tests.
- `make test-race` - Runs tests with the race detector.
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
./ttscli --version
# ttscli version=v0.1.0 commit=abc1234 date=2026-04-12T12:00:00Z
```

Without explicit build metadata, local binaries show defaults (`version=dev`, `commit=none`, `date=unknown`).
