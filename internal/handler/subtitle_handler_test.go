package handler

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"go.uber.org/zap"
)

func TestSaveCookiesToFileUsesOwnerOnlyPermissions(t *testing.T) {
	dataPath := t.TempDir()
	handler := NewSubtitleHandler(&core.AppServer{
		Config: &types.AppConfig{DataPath: dataPath},
		Logger: zap.NewNop().Sugar(),
	})

	if err := handler.saveCookiesToFile("SID=secret; YSC=token"); err != nil {
		t.Fatalf("saveCookiesToFile() error = %v", err)
	}

	cookiesDir := filepath.Join(dataPath, "cookies")
	dirInfo, err := os.Stat(cookiesDir)
	if err != nil {
		t.Fatalf("stat cookies dir: %v", err)
	}
	if runtime.GOOS == "windows" {
		assertNoBroadWindowsACL(t, cookiesDir)
	} else if dirInfo.Mode().Perm()&0077 != 0 {
		t.Fatalf("cookies dir permissions = %v, want no group/other permissions", dirInfo.Mode().Perm())
	}

	matches, err := filepath.Glob(filepath.Join(cookiesDir, "cookies_*.txt"))
	if err != nil {
		t.Fatalf("glob cookies files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("cookies files = %d, want 1", len(matches))
	}

	fileInfo, err := os.Stat(matches[0])
	if err != nil {
		t.Fatalf("stat cookies file: %v", err)
	}
	if runtime.GOOS == "windows" {
		assertNoBroadWindowsACL(t, matches[0])
	} else if fileInfo.Mode().Perm()&0077 != 0 {
		t.Fatalf("cookies file permissions = %v, want no group/other permissions", fileInfo.Mode().Perm())
	}
}

func assertNoBroadWindowsACL(t *testing.T, path string) {
	t.Helper()

	output, err := exec.Command("icacls", path).CombinedOutput()
	if err != nil {
		t.Fatalf("icacls %s: %v\n%s", path, err, string(output))
	}

	raw := string(output)
	for _, broadPrincipal := range []string{
		"BUILTIN\\Users",
		"NT AUTHORITY\\Authenticated Users",
		"CodexSandboxUsers",
	} {
		if strings.Contains(raw, broadPrincipal) {
			t.Fatalf("cookies path %s still grants broad principal %q:\n%s", path, broadPrincipal, raw)
		}
	}
}
