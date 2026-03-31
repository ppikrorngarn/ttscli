package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
)

type synthesizeRequest struct {
	Input struct {
		Text string `json:"text"`
	} `json:"input"`
	Voice struct {
		LanguageCode string `json:"languageCode"`
		Name         string `json:"name,omitempty"`
		SsmlGender   string `json:"ssmlGender,omitempty"`
	} `json:"voice"`
	AudioConfig struct {
		AudioEncoding string  `json:"audioEncoding"`
		SpeakingRate  float64 `json:"speakingRate,omitempty"`
		Pitch         float64 `json:"pitch,omitempty"`
	} `json:"audioConfig"`
}

type synthesizeResponse struct {
	AudioContent string `json:"audioContent"`
}

type listVoicesResponse struct {
	Voices []struct {
		LanguageCodes []string `json:"languageCodes"`
		Name          string   `json:"name"`
		SsmlGender    string   `json:"ssmlGender"`
	} `json:"voices"`
}

func main() {
	// Parse CLI flags
	textPtr := flag.String("text", "", "Text to convert to speech")
	savePtr := flag.String("save", "", "Path to save the output MP3 file (e.g., output.mp3)")
	playPtr := flag.Bool("play", false, "Play the audio immediately")
	langPtr := flag.String("lang", "en-US", "Language code")
	voicePtr := flag.String("voice", "en-US-Neural2-F", "Voice name")
	listPtr := flag.Bool("list-voices", false, "List available voices (filtered by --lang)")

	// Customize the default help message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "GCP Text-to-Speech CLI\n\nUsage:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ttscli --text \"Hello world\" --play\n")
		fmt.Fprintf(os.Stderr, "  ttscli --list-voices --lang en-GB\n")
	}

	flag.Parse()

	// Load .env file if it exists, ignoring errors if it doesn't
	_ = godotenv.Load()

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		exitErr(errors.New("GOOGLE_API_KEY environment variable is not set"))
	}

	if *listPtr {
		if err := fetchAndPrintVoices(apiKey, *langPtr); err != nil {
			exitErr(fmt.Errorf("failed to list voices: %w", err))
		}
		return
	}

	if *textPtr == "" {
		exitErr(errors.New("please provide text using the --text flag"))
	}

	if *savePtr == "" && !*playPtr {
		exitErr(errors.New("please specify either --save <path> or --play (or both)"))
	}

	fmt.Println("Synthesizing speech...")
	audioBytes, err := synthesizeWithAPIKey(apiKey, *textPtr, *langPtr, *voicePtr, "MP3")
	if err != nil {
		exitErr(fmt.Errorf("failed to synthesize: %w", err))
	}

	// Save if requested
	if *savePtr != "" {
		if err := os.WriteFile(*savePtr, audioBytes, 0o644); err != nil {
			exitErr(fmt.Errorf("failed to save file: %w", err))
		}
		fmt.Printf("Saved audio to: %s\n", *savePtr)
	}

	// Play if requested
	if *playPtr {
		fmt.Println("Playing audio...")
		if err := playAudio(audioBytes); err != nil {
			exitErr(fmt.Errorf("failed to play audio: %w", err))
		}
	}
}

func synthesizeWithAPIKey(apiKey, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
	var reqBody synthesizeRequest
	reqBody.Input.Text = text
	reqBody.Voice.LanguageCode = languageCode
	reqBody.Voice.Name = voiceName
	reqBody.AudioConfig.AudioEncoding = audioEncoding

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := "https://texttospeech.googleapis.com/v1/text:synthesize?key=" + apiKey
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(body))
	}

	var parsed synthesizeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if parsed.AudioContent == "" {
		return nil, errors.New("empty audioContent in response")
	}

	audioBytes, err := base64.StdEncoding.DecodeString(parsed.AudioContent)
	if err != nil {
		return nil, fmt.Errorf("decode audioContent: %w", err)
	}
	return audioBytes, nil
}

// playAudio writes bytes to a temp file and plays it using OS-specific commands
func playAudio(audioBytes []byte) error {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "ttscli_temp.mp3")

	if err := os.WriteFile(tmpFile, audioBytes, 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("afplay", tmpFile)
	case "linux":
		// check if mpg123 is installed, fallback to paplay or ffplay
		if _, err := exec.LookPath("mpg123"); err == nil {
			cmd = exec.Command("mpg123", "-q", tmpFile)
		} else if _, err := exec.LookPath("paplay"); err == nil {
			cmd = exec.Command("paplay", tmpFile)
		} else if _, err := exec.LookPath("ffplay"); err == nil {
			cmd = exec.Command("ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", tmpFile)
		} else {
			return errors.New("no supported audio player found on Linux (try installing mpg123)")
		}
	case "windows":
		// use powershell to play the file using System.Media.SoundPlayer
		// Note: SoundPlayer might not support mp3 out of the box depending on Windows version,
		// but typically we can use mplayer or a built-in media player trick.
		// For simplicity, we can use an inline powershell script using Media.MediaPlayer
		psScript := fmt.Sprintf(`(New-Object Media.SoundPlayer "%s").PlaySync()`, tmpFile)
		cmd = exec.Command("powershell", "-c", psScript)
	default:
		return fmt.Errorf("unsupported platform for audio playback: %s", runtime.GOOS)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

func fetchAndPrintVoices(apiKey, langCode string) error {
	urlStr := "https://texttospeech.googleapis.com/v1/voices?key=" + apiKey
	if langCode != "" {
		urlStr += "&languageCode=" + langCode
	}

	resp, err := http.Get(urlStr)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, string(body))
	}

	var parsed listVoicesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	fmt.Printf("Available voices for language: %s\n", langCode)
	fmt.Printf("%-35s | %-10s | %s\n", "VOICE NAME", "GENDER", "LANGUAGES")
	fmt.Println("----------------------------------------------------------------------")
	for _, v := range parsed.Voices {
		fmt.Printf("%-35s | %-10s | %v\n", v.Name, v.SsmlGender, v.LanguageCodes)
	}

	return nil
}
