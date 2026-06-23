package main

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRunServerAsyncReportsStartupError(t *testing.T) {
	wantErr := errors.New("bind failed")

	errCh := runServerAsync(func() error {
		return wantErr
	}, zap.NewNop().Sugar())

	select {
	case gotErr := <-errCh:
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("runServerAsync error = %v, want %v", gotErr, wantErr)
		}
	case <-time.After(time.Second):
		t.Fatal("runServerAsync did not report server startup error")
	}
}
