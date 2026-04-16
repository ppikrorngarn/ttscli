package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

func TestCheckPlaybackCapabilityDarwinFound(t *testing.T) {
	check := checkPlaybackCapability("darwin", func(f string) (string, error) {
		return "/usr/bin/" + f, nil
	})
	if !check.ok || check.detail != "found afplay" {
		t.Errorf("expected ok with afplay, got %+v", check)
	}
}

func TestCheckPlaybackCapabilityDarwinNotFound(t *testing.T) {
	check := checkPlaybackCapability("darwin", func(f string) (string, error) {
		return "", errors.New("not found")
	})
	if check.ok {
		t.Error("expected not ok")
	}
	if !strings.Contains(check.detail, "afplay") {
		t.Errorf("expected detail to mention afplay, got %q", check.detail)
	}
}

func TestCheckPlaybackCapabilityLinuxPrefersMpg123(t *testing.T) {
	check := checkPlaybackCapability("linux", func(f string) (string, error) {
		if f == "mpg123" {
			return "/usr/bin/mpg123", nil
		}
		return "", errors.New("missing")
	})
	if !check.ok || check.detail != "found mpg123" {
		t.Errorf("expected ok with mpg123, got %+v", check)
	}
}

func TestCheckPlaybackCapabilityLinuxFallbackPaplay(t *testing.T) {
	check := checkPlaybackCapability("linux", func(f string) (string, error) {
		if f == "paplay" {
			return "/usr/bin/paplay", nil
		}
		return "", errors.New("missing")
	})
	if !check.ok || check.detail != "found paplay" {
		t.Errorf("expected ok with paplay, got %+v", check)
	}
}

func TestCheckPlaybackCapabilityLinuxFallbackFfplay(t *testing.T) {
	check := checkPlaybackCapability("linux", func(f string) (string, error) {
		if f == "ffplay" {
			return "/usr/bin/ffplay", nil
		}
		return "", errors.New("missing")
	})
	if !check.ok || check.detail != "found ffplay" {
		t.Errorf("expected ok with ffplay, got %+v", check)
	}
}

func TestCheckPlaybackCapabilityLinuxNoPlayers(t *testing.T) {
	check := checkPlaybackCapability("linux", func(f string) (string, error) {
		return "", errors.New("missing")
	})
	if check.ok {
		t.Error("expected not ok when no players found")
	}
}

func TestCheckPlaybackCapabilityWindowsFound(t *testing.T) {
	check := checkPlaybackCapability("windows", func(f string) (string, error) {
		if f == "powershell" {
			return "powershell", nil
		}
		return "", errors.New("missing")
	})
	if !check.ok || check.detail != "found powershell" {
		t.Errorf("expected ok with powershell, got %+v", check)
	}
}

func TestCheckPlaybackCapabilityWindowsNotFound(t *testing.T) {
	check := checkPlaybackCapability("windows", func(f string) (string, error) {
		return "", errors.New("missing")
	})
	if check.ok {
		t.Error("expected not ok when powershell not found")
	}
}

func TestCheckPlaybackCapabilityUnsupportedPlatform(t *testing.T) {
	check := checkPlaybackCapability("plan9", func(f string) (string, error) {
		return "", nil
	})
	if check.ok {
		t.Error("expected not ok for unsupported platform")
	}
	if !strings.Contains(check.detail, "plan9") {
		t.Errorf("expected platform name in detail, got %q", check.detail)
	}
}

func TestPrintDoctorChecksAllPass(t *testing.T) {
	var stdout bytes.Buffer
	checks := []doctorCheck{
		{name: "A", ok: true, detail: "fine"},
		{name: "B", ok: true, detail: "also fine"},
	}
	failed := printDoctorChecks(&stdout, checks)
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}
	if strings.Contains(stdout.String(), "[FAIL]") {
		t.Error("expected no FAIL in output")
	}
}

func TestPrintDoctorChecksSomeFail(t *testing.T) {
	var stdout bytes.Buffer
	checks := []doctorCheck{
		{name: "A", ok: true, detail: "fine"},
		{name: "B", ok: false, detail: "broken", hint: "fix it"},
	}
	failed := printDoctorChecks(&stdout, checks)
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
	out := stdout.String()
	if !strings.Contains(out, "[FAIL]") {
		t.Error("expected FAIL in output")
	}
	if !strings.Contains(out, "fix it") {
		t.Error("expected hint in output")
	}
}

func TestPrintDoctorChecksHintOnlyOnFail(t *testing.T) {
	var stdout bytes.Buffer
	checks := []doctorCheck{
		{name: "A", ok: true, detail: "fine", hint: "should not appear"},
	}
	printDoctorChecks(&stdout, checks)
	if strings.Contains(stdout.String(), "should not appear") {
		t.Error("expected hint not to appear for passing check")
	}
}

func TestRunDoctorCommandOK(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	currentGOOS = func() string { return "darwin" }
	lookPathCmd = func(f string) (string, error) { return "/usr/bin/" + f, nil }

	var stdout bytes.Buffer
	if err := runDoctorCommand(&stdout); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stdout.String(), "Doctor result: OK") {
		t.Errorf("expected OK result, got: %q", stdout.String())
	}
}

func TestRunDoctorCommandNoProfiles(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	loadConfig = func() (config.Config, error) {
		return config.Config{Profiles: map[string]config.Profile{}}, nil
	}
	currentGOOS = func() string { return "darwin" }
	lookPathCmd = func(f string) (string, error) { return "/usr/bin/" + f, nil }

	var stdout bytes.Buffer
	err := runDoctorCommand(&stdout)
	if err == nil {
		t.Fatal("expected error when no profiles configured")
	}
	out := stdout.String()
	if !strings.Contains(out, "Doctor result: FAIL") {
		t.Errorf("expected FAIL result, got: %q", out)
	}
	if !strings.Contains(out, "no profiles configured") {
		t.Errorf("expected no profiles message, got: %q", out)
	}
}

func TestRunDoctorCommandAPIConnectivityFail(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	currentGOOS = func() string { return "darwin" }
	lookPathCmd = func(f string) (string, error) { return "/usr/bin/" + f, nil }
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, lang string) ([]tts.Voice, error) {
				return nil, errors.New("API error")
			},
			synthesizeFn: func(ctx context.Context, text, lang, voice, enc string) ([]byte, error) {
				return nil, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	err := runDoctorCommand(&stdout)
	if err == nil {
		t.Fatal("expected error when API fails")
	}
	if !strings.Contains(stdout.String(), "[FAIL] API connectivity") {
		t.Errorf("expected API connectivity failure, got: %q", stdout.String())
	}
}

func TestRunDoctorCommandNoActiveProfile(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	loadConfig = func() (config.Config, error) {
		return config.Config{
			Profiles: map[string]config.Profile{
				"gcp:default": {Provider: "gcp", Name: "default"},
			},
		}, nil
	}
	currentGOOS = func() string { return "darwin" }
	lookPathCmd = func(f string) (string, error) { return "/usr/bin/" + f, nil }

	var stdout bytes.Buffer
	err := runDoctorCommand(&stdout)
	if err == nil {
		t.Fatal("expected error when no active profile set")
	}
	if !strings.Contains(stdout.String(), "no active profile set") {
		t.Errorf("expected no active profile message, got: %q", stdout.String())
	}
}
