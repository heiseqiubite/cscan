package worker

import (
	"context"
	"testing"

	"cscan/scanner"
	"cscan/scheduler"
)

type noopLogger struct{}

func (l *noopLogger) Info(format string, args ...interface{})  {}
func (l *noopLogger) Error(format string, args ...interface{}) {}
func (l *noopLogger) Warn(format string, args ...interface{})  {}
func (l *noopLogger) Debug(format string, args ...interface{}) {}

func TestFingerprintExecutorGeneratedAssetsPersistToContext(t *testing.T) {
	executor := &FingerprintExecutor{worker: &Worker{logger: &noopLogger{}}}
	ctx := &TaskContext{
		Ctx:    context.Background(),
		Task:   &scheduler.TaskInfo{TaskId: "t1"},
		Target: "example.com",
		Config: &scheduler.TaskConfig{
			Fingerprint: &scheduler.FingerprintConfig{Enable: true},
		},
	}

	if !executor.CanExecute(ctx) {
		t.Fatal("expected executor to be runnable")
	}

	assets := scanner.GenerateAssetsFromTargets(ctx.Target)
	if len(assets) == 0 {
		t.Fatal("expected generated assets")
	}

	ctx.Assets = assets
	if len(ctx.Assets) == 0 {
		t.Fatal("expected generated assets to persist on context")
	}
}
