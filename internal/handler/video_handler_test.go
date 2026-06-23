package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestRunningStatusIncludesQueuedAndSchedulerStatuses(t *testing.T) {
	activeStatuses := []string{"001", "002", "200", "201", "300", "301"}
	for _, status := range activeStatuses {
		if !isRunningStatus(status) {
			t.Fatalf("status %s is not protected from deletion", status)
		}
	}

	if isRunningStatus("999") {
		t.Fatal("failed status 999 should remain deletable")
	}
}

func TestGetVideoDirectoryResolvesDateScopedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	videoID := "video-1"
	want := filepath.Join(tmpDir, "2026-06-09", videoID)
	if err := os.MkdirAll(want, 0755); err != nil {
		t.Fatalf("create video directory: %v", err)
	}

	handler := &VideoHandler{
		BaseHandler: BaseHandler{
			App: &core.AppServer{
				Config: &types.AppConfig{FileUpDir: tmpDir},
				Logger: zap.NewNop().Sugar(),
			},
		},
	}

	got := handler.getVideoDirectory(videoID)
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("getVideoDirectory() = %q, want %q", got, want)
	}
}

func TestGetVideoDirectoryChoosesLatestDateScopedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	videoID := "rerun-video"
	oldDir := filepath.Join(tmpDir, "2026-06-09", videoID)
	newDir := filepath.Join(tmpDir, "2026-06-23", videoID)
	for _, dir := range []string{oldDir, newDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create video directory %s: %v", dir, err)
		}
	}

	handler := &VideoHandler{
		BaseHandler: BaseHandler{
			App: &core.AppServer{
				Config: &types.AppConfig{FileUpDir: tmpDir},
				Logger: zap.NewNop().Sugar(),
			},
		},
	}

	got := handler.getVideoDirectory(videoID)
	if filepath.Clean(got) != filepath.Clean(newDir) {
		t.Fatalf("getVideoDirectory() = %q, want latest directory %q", got, newDir)
	}
}

func TestGetVideoCoverImageFindsDownloadedYoutubeThumbnail(t *testing.T) {
	tmpDir := t.TempDir()
	videoID := "video-1"
	videoDir := filepath.Join(tmpDir, "2026-06-22", videoID)
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		t.Fatalf("create video directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(videoDir, "maxresdefault.jpg"), []byte("jpg"), 0644); err != nil {
		t.Fatalf("write thumbnail: %v", err)
	}

	handler := newTestVideoHandler(t, tmpDir)

	got := handler.getVideoCoverImage(videoID)
	want := "/api/v1/videos/video-1/files/maxresdefault.jpg"
	if got != want {
		t.Fatalf("getVideoCoverImage() = %q, want %q", got, want)
	}
}

func TestListVideoFilesIncludesDownloadPathAndTypes(t *testing.T) {
	tmpDir := t.TempDir()
	videoID := "video-1"
	videoDir := filepath.Join(tmpDir, "2026-06-22", videoID)
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		t.Fatalf("create video directory: %v", err)
	}
	files := map[string][]byte{
		"video-1.mp4":       []byte("mp4"),
		"video-1.srt":       []byte("srt"),
		"maxresdefault.jpg": []byte("jpg"),
		"meta.json":         []byte("{}"),
		"audio.mp3":         []byte("mp3"),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(videoDir, name), content, 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	handler := newTestVideoHandler(t, tmpDir)

	got := handler.listVideoFiles(videoDir)
	if len(got) != len(files) {
		t.Fatalf("listVideoFiles() returned %d files, want %d", len(got), len(files))
	}

	byName := make(map[string]map[string]interface{}, len(got))
	for _, file := range got {
		byName[file["name"].(string)] = file
	}

	assertFile := func(name, wantType string) {
		t.Helper()
		file, ok := byName[name]
		if !ok {
			t.Fatalf("missing file %s in %#v", name, got)
		}
		if file["type"] != wantType {
			t.Fatalf("%s type = %q, want %q", name, file["type"], wantType)
		}
		wantPath := "/api/v1/videos/video-1/files/" + name
		if file["path"] != wantPath {
			t.Fatalf("%s path = %q, want %q", name, file["path"], wantPath)
		}
		if file["created_at"] == "" {
			t.Fatalf("%s missing created_at", name)
		}
	}

	assertFile("video-1.mp4", "video")
	assertFile("video-1.srt", "subtitle")
	assertFile("maxresdefault.jpg", "image")
	assertFile("meta.json", "metadata")
	assertFile("audio.mp3", "audio")
}

func TestServeVideoFileRejectsPathTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	handler := newTestVideoHandler(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/videos/video-1/files/secret.txt", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Params = gin.Params{
		{Key: "id", Value: "video-1"},
		{Key: "filename", Value: `..\secret.txt`},
	}

	handler.serveVideoFile(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("serveVideoFile traversal status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func newTestVideoHandler(t *testing.T, fileUpDir string) *VideoHandler {
	t.Helper()

	return &VideoHandler{
		BaseHandler: BaseHandler{
			App: &core.AppServer{
				Config: &types.AppConfig{FileUpDir: fileUpDir},
				Logger: zap.NewNop().Sugar(),
			},
		},
	}
}
