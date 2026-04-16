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
