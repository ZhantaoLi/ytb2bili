package handler

import "testing"

func TestSafeUploadTempPathRejectsPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	for _, filename := range []string{"../escape.png", `..\escape.png`, "/tmp/escape.png", `C:\tmp\escape.png`} {
		if _, err := safeUploadTempPath(tmpDir, filename); err == nil {
			t.Fatalf("safeUploadTempPath(%q) error = nil, want traversal rejection", filename)
		}
	}
}

func TestSafeUploadTempPathAllowsPlainFilename(t *testing.T) {
	tmpDir := t.TempDir()

	path, err := safeUploadTempPath(tmpDir, "cover.png")
	if err != nil {
		t.Fatalf("safeUploadTempPath() error = %v", err)
	}
	if path == tmpDir {
		t.Fatal("safeUploadTempPath() returned directory path, want file path")
	}
}
