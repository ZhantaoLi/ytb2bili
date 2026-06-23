package manager

import (
	"strings"
	"testing"
)

type panicTask struct{}

func (panicTask) GetName() string {
	return "panic-task"
}

func (panicTask) InsertTask() error {
	return nil
}

func (panicTask) UpdateStatus(string, string) error {
	return nil
}

func (panicTask) Execute(map[string]interface{}) bool {
	panic("boom")
}

func TestTaskChainRecordsPanicAsError(t *testing.T) {
	chain := NewTaskChain()
	chain.AddTask(panicTask{})

	result := chain.Run(true)

	rawError, ok := result["error"].(string)
	if !ok {
		t.Fatalf("result[error] missing or not string: %#v", result["error"])
	}
	if !strings.Contains(rawError, "boom") {
		t.Fatalf("result[error] = %q, want panic detail", rawError)
	}
}
