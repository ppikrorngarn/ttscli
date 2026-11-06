package app

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/ppikrorngarn/ttscli/internal/tts"
)

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

func promptPassword(reader *bufio.Reader, stdout io.Writer, prompt string) (string, error) {
	fmt.Fprint(stdout, prompt)
	password, err := readPassword()
	if err != nil {
		return "", err
	}
	fmt.Fprintln(stdout) // Add newline after password input
	return strings.TrimSpace(string(password)), nil
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
			fmt.Fprintln(stdout, "Please enter 'y' for yes or 'n' for no.")
		}
	}
}

// printVoicesImpl renders the voices table plus usage hints for the voices command.
func printVoicesImpl(w io.Writer, langCode string, voices []tts.Voice) {
	fmt.Fprintf(w, "Available voices for language: %s\n", langCode)
	fmt.Fprintf(w, "Found %d voice(s)\n", len(voices))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "VOICE NAME                            GENDER       LANGUAGES")
	fmt.Fprintln(w, "───────────────────────────────────── ──────────── ─────────────────────")
	for _, v := range voices {
		gender := v.SsmlGender
		if gender == "" {
			gender = "Unknown"
		}
		languages := strings.Join(v.LanguageCodes, ", ")
		if len(languages) > 35 {
			languages = languages[:32] + "..."
		}
		fmt.Fprintf(w, "%-37s %-12s %s\n", v.Name, gender, languages)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  ttscli speak --text \"Hello\" --lang %s --voice <VOICE_NAME>\n", langCode)
	fmt.Fprintf(w, "  ttscli say --text \"Hello\" --lang %s --voice <VOICE_NAME>\n", langCode)
	fmt.Fprintf(w, "  ttscli save --text \"Hello\" --out speech.mp3 --lang %s --voice <VOICE_NAME>\n", langCode)
}
