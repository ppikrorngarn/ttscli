package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/ppikrorngarn/ttscli/internal/cli"
)

type doctorCheck struct {
	name   string
	ok     bool
	detail string
	hint   string
}

func runDoctorCommand(stdout io.Writer) error {
	var checks []doctorCheck

	appCfg, err := loadConfig()
	if err != nil {
		checks = append(checks, doctorCheck{
			name:   "Config file",
			ok:     false,
			detail: err.Error(),
			hint:   "Check that the config file exists and has proper read permissions.",
		})
	} else {
		checks = append(checks, doctorCheck{
			name:   "Config file",
			ok:     true,
			detail: "Configuration file found and readable",
		})
	}

	if len(appCfg.Profiles) == 0 {
		checks = append(checks, doctorCheck{
			name:   "Profiles",
			ok:     false,
			detail: "No profiles configured",
			hint:   "Run \"ttscli setup\" for interactive setup or \"ttscli profile create\" to add a profile.",
		})
		checks = append(checks, doctorCheck{
			name:   "Active profile",
			ok:     false,
			detail: "No active profile set",
			hint:   "Use \"ttscli profile use <provider:name>\" to set an active profile.",
		})
		checks = append(checks, doctorCheck{
			name:   "API connectivity",
			ok:     false,
			detail: "Skipped (no profiles configured)",
			hint:   "Create a profile first to test API connectivity.",
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
				detail: "No active profile set",
				hint:   "Use \"ttscli profile use <provider:name>\" to select an active profile.",
			})
			checks = append(checks, doctorCheck{
				name:   "API connectivity",
				ok:     false,
				detail: "Skipped (no active profile)",
				hint:   "Set an active profile first to test API connectivity.",
			})
		} else {
			profile, profileErr := getProfile(appCfg, activeProfileKey)
			if profileErr != nil {
				checks = append(checks, doctorCheck{
					name:   "Active profile",
					ok:     false,
					detail: profileErr.Error(),
					hint:   "Use \"ttscli profile use <provider:name>\" to select a valid profile.",
				})
				checks = append(checks, doctorCheck{
					name:   "API connectivity",
					ok:     false,
					detail: "Skipped (invalid active profile)",
					hint:   "Fix or change the active profile first.",
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
						hint:   "Check the profile credentials and configuration.",
					})
					checks = append(checks, doctorCheck{
						name:   "API connectivity",
						ok:     false,
						detail: "Skipped (provider initialization failed)",
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
							hint:   "Verify your API key, permissions, and that the TTS service is enabled.",
						})
					} else if len(voices) == 0 {
						checks = append(checks, doctorCheck{
							name:   "API connectivity",
							ok:     false,
							detail: "Connected but no voices returned",
							hint:   "Check API permissions and account/project status in your cloud console.",
						})
					} else {
						checks = append(checks, doctorCheck{
							name:   "API connectivity",
							ok:     true,
							detail: fmt.Sprintf("Successfully listed %d voices for %s", len(voices), testLang),
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
		fmt.Fprintf(stdout, "\n✗ Doctor result: FAIL (%d check(s) failed)\n", failed)
		return fmt.Errorf("doctor failed with %d check(s)", failed)
	}

	fmt.Fprintln(stdout, "\n✓ Doctor result: OK - All checks passed")
	return nil
}

func checkPlaybackCapability(goos string, lookPath func(file string) (string, error)) doctorCheck {
	switch goos {
	case "darwin":
		if _, err := lookPath("afplay"); err != nil {
			return doctorCheck{
				name:   "Audio playback",
				ok:     false,
				detail: "afplay command not found",
				hint:   "macOS should include afplay by default. Check your system installation.",
			}
		}
		return doctorCheck{name: "Audio playback", ok: true, detail: "afplay found"}
	case "linux":
		if _, err := lookPath("mpg123"); err == nil {
			return doctorCheck{name: "Audio playback", ok: true, detail: "mpg123 found"}
		}
		if _, err := lookPath("paplay"); err == nil {
			return doctorCheck{name: "Audio playback", ok: true, detail: "paplay found"}
		}
		if _, err := lookPath("ffplay"); err == nil {
			return doctorCheck{name: "Audio playback", ok: true, detail: "ffplay found"}
		}
		return doctorCheck{
			name:   "Audio playback",
			ok:     false,
			detail: "No supported player found (requires mpg123, paplay, or ffplay)",
			hint:   "Install a player: sudo apt install mpg123 (Debian/Ubuntu) or sudo dnf install mpg123 (Fedora)",
		}
	case "windows":
		if _, err := lookPath("mpg123"); err == nil {
			return doctorCheck{name: "Audio playback", ok: true, detail: "mpg123 found"}
		}
		if _, err := lookPath("ffplay"); err == nil {
			return doctorCheck{name: "Audio playback", ok: true, detail: "ffplay found"}
		}
		return doctorCheck{
			name:   "Audio playback",
			ok:     false,
			detail: "No supported player found (requires mpg123 or ffplay)",
			hint:   "Install ffplay (ships with ffmpeg from https://ffmpeg.org/) or mpg123, and ensure the binary is on your PATH.",
		}
	default:
		return doctorCheck{
			name:   "Audio playback",
			ok:     false,
			detail: fmt.Sprintf("Unsupported platform: %s", goos),
			hint:   "Audio playback is supported on macOS, Linux, and Windows.",
		}
	}
}

func printDoctorChecks(stdout io.Writer, checks []doctorCheck) int {
	fmt.Fprintln(stdout, "Running diagnostics...")
	fmt.Fprintln(stdout)
	failed := 0
	for _, check := range checks {
		if !check.ok {
			failed++
		}
		status := "✓"
		if !check.ok {
			status = "✗"
		}
		fmt.Fprintf(stdout, "  [%s] %s: %s\n", status, check.name, check.detail)
		if !check.ok && strings.TrimSpace(check.hint) != "" {
			fmt.Fprintf(stdout, "       → %s\n", check.hint)
		}
	}
	return failed
}
