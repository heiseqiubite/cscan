package scanner

import (
	"context"
	"os"
	"testing"
)

func TestDefaultGogoOptions(t *testing.T) {
	opts := defaultGogoOptions()

	if opts.Ports != "80,443,8080" {
		t.Fatalf("expected default ports, got %q", opts.Ports)
	}
	if opts.Threads != 500 {
		t.Fatalf("expected default threads 500, got %d", opts.Threads)
	}
	if opts.Timeout != 3 {
		t.Fatalf("expected default timeout 3, got %d", opts.Timeout)
	}
	if opts.Delay != 2 || opts.HttpsDelay != 2 {
		t.Fatalf("expected default delays 2/2, got %d/%d", opts.Delay, opts.HttpsDelay)
	}
	if opts.Mod != "default" {
		t.Fatalf("expected default mod, got %q", opts.Mod)
	}
	if opts.Exploit != "none" {
		t.Fatalf("expected default exploit none, got %q", opts.Exploit)
	}
}

func TestNormalizeGogoOptionsNilUsesDefaults(t *testing.T) {
	opts, err := normalizeGogoOptions(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Ports != "80,443,8080" {
		t.Fatalf("expected default ports, got %q", opts.Ports)
	}
	if opts.Threads != 500 {
		t.Fatalf("expected default threads 500, got %d", opts.Threads)
	}
	if opts.Mod != "default" {
		t.Fatalf("expected default mod, got %q", opts.Mod)
	}
}

func TestNormalizeGogoOptionsRejectsInvalidMod(t *testing.T) {
	_, err := normalizeGogoOptions(&GogoOptions{Mod: "bad-mod"})
	if err == nil {
		t.Fatal("expected invalid mod error")
	}
}

func TestNormalizeGogoOptionsRejectsNegativeThreads(t *testing.T) {
	_, err := normalizeGogoOptions(&GogoOptions{Threads: -1})
	if err == nil {
		t.Fatal("expected invalid threads error")
	}
}

func TestExtractGogoOptionsPrefersNestedConfig(t *testing.T) {
	input := map[string]interface{}{
		"ports": "1-100",
		"gogo": map[string]interface{}{
			"ports":   "80,443",
			"threads": 200,
			"mod":     "ss",
		},
	}

	opts, err := extractGogoOptions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Ports != "80,443" {
		t.Fatalf("expected nested ports to win, got %q", opts.Ports)
	}
	if opts.Threads != 200 {
		t.Fatalf("expected nested threads 200, got %d", opts.Threads)
	}
	if opts.Mod != "ss" {
		t.Fatalf("expected nested mod ss, got %q", opts.Mod)
	}
}

func TestExtractGogoOptionsFallsBackToTopLevel(t *testing.T) {
	input := map[string]interface{}{
		"ports":   "8080",
		"threads": 50,
		"mod":     "sc",
	}

	opts, err := extractGogoOptions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Ports != "8080" || opts.Threads != 50 || opts.Mod != "sc" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestGogoScannerBootstrapRequiresCyberhubOrCache(t *testing.T) {
	scanner := NewGogoScanner()
	err := scanner.Bootstrap(context.Background(), &BootstrapConfig{CacheDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected bootstrap error without cyberhub config or cache")
	}
}

func TestGogoScannerBootstrapUsesLocalCache(t *testing.T) {
	dir := t.TempDir()
	cache := gogoCachePaths() // no args, returns relative paths

	// Save and change to temp dir so scanner finds local cache files
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	defer os.Chdir(oldCwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	fingersContent := []byte(`- name: test
  protocol: http
  rule: []
`)
	pocsContent := []byte("- info:\n    name: test-poc\n  http:\n    - path:\n        - /\n")
	if err := os.WriteFile(cache.fingersFile, fingersContent, 0o644); err != nil {
		t.Fatalf("write fingers cache failed: %v", err)
	}
	if err := os.WriteFile(cache.pocsFile, pocsContent, 0o644); err != nil {
		t.Fatalf("write pocs cache failed: %v", err)
	}

	scanner := NewGogoScanner()
	err = scanner.Bootstrap(context.Background(), &BootstrapConfig{CacheDir: dir})
	if err != nil {
		t.Fatalf("bootstrap with local cache should not error: %v", err)
	}
	if !scanner.IsInited() {
		t.Fatal("expected scanner initialized from local cache")
	}
}

func TestGogoScannerScanRequiresInit(t *testing.T) {
	scanner := NewGogoScanner()
	_, err := scanner.Scan(context.Background(), &ScanConfig{Target: "127.0.0.1"})
	if err == nil {
		t.Fatal("expected init error")
	}
}
