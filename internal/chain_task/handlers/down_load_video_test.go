package handlers

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatDownloadErrorIncludesYtDlpStderr(t *testing.T) {
	stderrLines := []string{
		"[youtube] w_O6bZo1rVU: Downloading webpage",
		"ERROR: [youtube] w_O6bZo1rVU: Sign in to confirm you're not a bot. Use --cookies-from-browser or --cookies for the authentication.",
	}

	got := formatDownloadError(errors.New("exit status 1"), stderrLines)

	if !strings.Contains(got, "Sign in to confirm") {
		t.Fatalf("formatted error should include yt-dlp stderr, got %q", got)
	}
	if !strings.Contains(got, "需要 YouTube cookies") {
		t.Fatalf("formatted error should include actionable cookies hint, got %q", got)
	}
}

func TestAppendYtDlpRuntimeArgsUsesNodeWhenAvailable(t *testing.T) {
	args := []string{"yt-dlp", "--cookies", "cookies.txt"}
	lookPath := func(name string) (string, error) {
		if name != "node" {
			return "", fmt.Errorf("unexpected runtime lookup: %s", name)
		}
		return `D:\ProgramData\nodejs\node.exe`, nil
	}

	got := appendYtDlpRuntimeArgs(args, lookPath)
	joined := strings.Join(got, " ")

	if !strings.Contains(joined, "--js-runtimes node:D:\\ProgramData\\nodejs\\node.exe") {
		t.Fatalf("yt-dlp args should include node js runtime, got %q", joined)
	}
}

func TestBuildYtDlpAuthAttemptsFallsBackToChromeCookies(t *testing.T) {
	attempts := buildYtDlpAuthAttemptsFromCookies(`D:\tmp\cookies.txt`)

	if len(attempts) != 2 {
		t.Fatalf("expected cookies file plus Chrome fallback, got %d attempts", len(attempts))
	}
	if strings.Join(attempts[0].args, " ") != `--cookies D:\tmp\cookies.txt` {
		t.Fatalf("first attempt should use cookies file, got %#v", attempts[0].args)
	}
	if strings.Join(attempts[1].args, " ") != "--cookies-from-browser chrome" {
		t.Fatalf("second attempt should use Chrome cookies, got %#v", attempts[1].args)
	}
}

func TestBuildYtDlpAuthAttemptsUsesChromeWhenNoCookiesFile(t *testing.T) {
	attempts := buildYtDlpAuthAttemptsFromCookies("")

	if len(attempts) != 1 {
		t.Fatalf("expected only Chrome cookies attempt, got %d attempts", len(attempts))
	}
	if strings.Join(attempts[0].args, " ") != "--cookies-from-browser chrome" {
		t.Fatalf("attempt should use Chrome cookies, got %#v", attempts[0].args)
	}
}

func TestIsYouTubeAuthChallenge(t *testing.T) {
	lines := []string{
		"ERROR: [youtube] gOepkI6OfD8: Sign in to confirm you're not a bot.",
	}

	if !isYouTubeAuthChallenge(lines) {
		t.Fatal("expected YouTube bot/login challenge to be detected")
	}
}

func TestDefaultYtDlpFormatCapsVideoHeight(t *testing.T) {
	if !strings.Contains(defaultYtDlpFormat, "height<=1080") {
		t.Fatalf("default format should avoid unexpectedly large 4K downloads, got %q", defaultYtDlpFormat)
	}
}

func TestNormalizeExecutablePathReturnsAbsolutePath(t *testing.T) {
	got := normalizeExecutablePath(filepath.Join("data", "tools", "yt-dlp", "yt-dlp.exe"))

	if !filepath.IsAbs(got) {
		t.Fatalf("expected absolute executable path, got %q", got)
	}
}
