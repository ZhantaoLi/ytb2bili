package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/base"
	"github.com/ZhantaoLi/ytb2bili/internal/chain_task/manager"
)

func TestBcutHandlerExecuteReusesExistingSubtitleBeforeAudio(t *testing.T) {
	tmpDir := t.TempDir()
	enSRT := filepath.Join(tmpDir, "en.srt")
	if err := os.WriteFile(enSRT, []byte("1\n00:00:00,000 --> 00:00:02,000\nGLM 5.2 versus Claude Opus 4.8\n\n"), 0644); err != nil {
		t.Fatalf("write existing subtitle: %v", err)
	}

	task := &BcutHandler{
		BaseTask: base.BaseTask{
			Name: "B站必剪转录",
			StateManager: &manager.StateManager{
				VideoID:     "YObDxucFg4s",
				CurrentDir:  tmpDir,
				OriginalSRT: enSRT,
				OriginalMP3: filepath.Join(tmpDir, "missing.mp3"),
				OriginalWAV: filepath.Join(tmpDir, "missing.wav"),
			},
		},
	}

	context := map[string]interface{}{}
	if ok := task.Execute(context); !ok {
		t.Fatalf("Execute() = false, want true when a source subtitle already exists; context=%v", context)
	}
	if got := context["subtitle_path"]; got != enSRT {
		t.Fatalf("subtitle_path = %v, want %s", got, enSRT)
	}
	if got := context["subtitle_source"]; got != "existing" {
		t.Fatalf("subtitle_source = %v, want existing", got)
	}
}

func TestBuildYouTubeSubtitleCommandIncludesAutoSubtitleFlags(t *testing.T) {
	command := buildYouTubeSubtitleCommand(youtubeSubtitleCommandOptions{
		YtDlpPath:  "yt-dlp",
		VideoURL:   "https://www.youtube.com/watch?v=YObDxucFg4s",
		OutputBase: filepath.Join("tmp", "youtube_captions"),
		Languages:  []string{"en-orig", "en"},
		Format:     "srt",
		ProxyURL:   "http://127.0.0.1:10809",
		AuthAttempt: ytDlpAuthAttempt{
			args: []string{"--cookies", "cookies.txt"},
		},
	})

	joined := strings.Join(command, " ")
	for _, want := range []string{
		"--write-auto-subs",
		"--write-subs",
		"--ignore-no-formats-error",
		"--sub-langs en-orig,en",
		"--sub-format srt",
		"--convert-subs srt",
		"--proxy http://127.0.0.1:10809",
		"--cookies cookies.txt",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("command %q missing %q", joined, want)
		}
	}
}

func TestFindPreferredYouTubeSubtitleFilePrefersOriginalEnglish(t *testing.T) {
	tmpDir := t.TempDir()
	outputBase := filepath.Join(tmpDir, "youtube_captions")
	enPath := outputBase + ".en.srt"
	origPath := outputBase + ".en-orig.srt"
	if err := os.WriteFile(enPath, []byte("translated english"), 0644); err != nil {
		t.Fatalf("write en subtitle: %v", err)
	}
	if err := os.WriteFile(origPath, []byte("original english"), 0644); err != nil {
		t.Fatalf("write en-orig subtitle: %v", err)
	}

	got := findPreferredYouTubeSubtitleFile(outputBase, []string{"en-orig", "en"}, "srt")
	if got != origPath {
		t.Fatalf("preferred subtitle = %q, want %q", got, origPath)
	}
}

func TestBcutQueryResultURLUsesCurrentModelID(t *testing.T) {
	got := bcutQueryResultURL("task-1")
	want := APIQueryResult + "?model_id=8&task_id=task-1"
	if got != want {
		t.Fatalf("bcut query URL = %q, want %q", got, want)
	}
}
