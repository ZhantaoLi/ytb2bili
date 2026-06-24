package handlers

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	if strings.Contains(joined, "--js-runtimes") {
		t.Fatalf("yt-dlp args should not use removed plural js runtime option, got %q", joined)
	}
	if !strings.Contains(joined, "--js-runtime node:D:\\ProgramData\\nodejs\\node.exe") {
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

func TestShouldReplaceDownloadFailureKeepsYouTubeAuthChallengeOverDPAPI(t *testing.T) {
	current := []string{
		"ERROR: [youtube] SRx8YiEwvoM: Sign in to confirm you're not a bot. Use --cookies-from-browser or --cookies for the authentication.",
	}
	candidate := []string{
		"ERROR: Failed to decrypt with DPAPI. See https://github.com/yt-dlp/yt-dlp/issues/10927 for more info",
	}

	if shouldReplaceDownloadFailure(current, candidate) {
		t.Fatal("DPAPI fallback error should not replace the more actionable YouTube cookies challenge")
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

func TestRunCommandWithTimeoutStopsLongRunningProcess(t *testing.T) {
	if os.Getenv("YTB2BILI_TIMEOUT_HELPER") == "1" {
		time.Sleep(time.Second)
		return
	}

	command := []string{os.Args[0], "-test.run=TestRunCommandWithTimeoutStopsLongRunningProcess"}
	_, err := runCommandWithTimeout(
		command,
		"",
		20*time.Millisecond,
		[]string{"YTB2BILI_TIMEOUT_HELPER=1"},
		func(reader io.Reader) {
			_, _ = io.Copy(io.Discard, reader)
		},
		func(reader io.Reader) []string {
			_, _ = io.Copy(io.Discard, reader)
			return nil
		},
	)

	if err == nil {
		t.Fatal("runCommandWithTimeout() error = nil, want timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("runCommandWithTimeout() error = %q, want timed out", err)
	}
}

func TestParseYtDlpMetadataIgnoresLeadingWarnings(t *testing.T) {
	raw := []byte("WARNING: Your yt-dlp version is older than 90 days!\nWARNING: To suppress this warning, add --no-update.\n{\"title\":\"GLM 5.2 vs Claude Was Crazy\",\"description\":\"sample desc\",\"uploader\":\"creator\",\"duration\":943}\n")

	metadata, err := parseYtDlpMetadata(raw)
	if err != nil {
		t.Fatalf("parseYtDlpMetadata() error = %v", err)
	}
	if metadata.Title != "GLM 5.2 vs Claude Was Crazy" {
		t.Fatalf("title = %q, want parsed title", metadata.Title)
	}
	if metadata.Description != "sample desc" {
		t.Fatalf("description = %q, want parsed description", metadata.Description)
	}
	if metadata.Duration != 943 {
		t.Fatalf("duration = %d, want 943", metadata.Duration)
	}
}
