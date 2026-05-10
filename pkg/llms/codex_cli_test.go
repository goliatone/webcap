package llms

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCodexCLIProviderBuildsCommandAndPassesPrompt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake uses POSIX sh")
	}
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "record.txt")
	fake := writeFakeCodex(t, dir, `#!/bin/sh
printf 'args:%s\n' "$*" > "$WEB_CAP_RECORD"
stdin=$(cat)
printf 'stdin:%s\n' "$stdin" >> "$WEB_CAP_RECORD"
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "--output-last-message" ]; then out="$arg"; fi
  prev="$arg"
done
printf '{"summary":"from file","verdict":"no_meaningful_change","severity":"info"}\n' > "$out"
`)
	t.Setenv("WEB_CAP_RECORD", recordPath)

	provider := NewCodexCLIProvider(CodexCLIOptions{
		CommandPath:   fake,
		Profile:       "work",
		UseOSS:        true,
		LocalProvider: "ollama",
		Ephemeral:     true,
		IgnoreRules:   true,
		ExtraArgs:     []string{"--reasoning-effort", "low"},
	})
	resp, err := provider.CompareImages(context.Background(), Request{
		Model:          "gpt-test",
		Prompt:         "Compare these images",
		StructuredJSON: true,
		Images: []Image{
			{Role: ImageRoleCurrent, Path: "current.png"},
			{Role: ImageRoleReference, Path: "reference.png"},
			{Role: ImageRoleDiff, Path: "diff.png"},
		},
	})
	if err != nil {
		t.Fatalf("CompareImages returned error: %v", err)
	}
	if resp.Provider != ProviderCodexCLI || resp.Model != "gpt-test" || !strings.Contains(resp.RawText, "from file") {
		t.Fatalf("unexpected response: %#v", resp)
	}
	record := readFile(t, recordPath)
	for _, expected := range []string{
		"exec",
		"--profile work",
		"--oss",
		"--local-provider ollama",
		"--ephemeral",
		"--ignore-rules",
		"--image current.png",
		"--image reference.png",
		"--image diff.png",
		"--model gpt-test",
		"--output-schema",
		"--output-last-message",
		"--reasoning-effort low",
		"stdin:Compare these images",
	} {
		if !strings.Contains(record, expected) {
			t.Fatalf("expected record to contain %q, got:\n%s", expected, record)
		}
	}
}

func TestCodexCLIProviderFallsBackToStdout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake uses POSIX sh")
	}
	dir := t.TempDir()
	fake := writeFakeCodex(t, dir, `#!/bin/sh
cat >/dev/null
printf '{"summary":"from stdout","verdict":"no_meaningful_change","severity":"info"}\n'
`)
	provider := NewCodexCLIProvider(CodexCLIOptions{CommandPath: fake})
	resp, err := provider.CompareImages(context.Background(), Request{
		Prompt: "Compare",
		Images: []Image{{Path: "current.png"}, {Path: "reference.png"}},
	})
	if err != nil {
		t.Fatalf("CompareImages returned error: %v", err)
	}
	if !strings.Contains(resp.RawText, "from stdout") {
		t.Fatalf("expected stdout fallback, got %#v", resp)
	}
}

func TestCodexCLIProviderCapturesStderrWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake uses POSIX sh")
	}
	dir := t.TempDir()
	fake := writeFakeCodex(t, dir, `#!/bin/sh
echo "abcdef" >&2
printf '{"summary":"ok","verdict":"no_meaningful_change","severity":"info"}\n'
`)
	provider := NewCodexCLIProvider(CodexCLIOptions{CommandPath: fake, StderrLimit: 4})
	resp, err := provider.CompareImages(context.Background(), Request{Prompt: "Compare", Images: []Image{{Path: "current.png"}}})
	if err != nil {
		t.Fatalf("CompareImages returned error: %v", err)
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0].Message != "a..." {
		t.Fatalf("expected capped stderr warning, got %#v", resp.Warnings)
	}
}

func TestCodexCLIProviderRejectsMissingOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake uses POSIX sh")
	}
	dir := t.TempDir()
	fake := writeFakeCodex(t, dir, `#!/bin/sh
cat >/dev/null
`)
	provider := NewCodexCLIProvider(CodexCLIOptions{CommandPath: fake})
	_, err := provider.CompareImages(context.Background(), Request{Prompt: "Compare", Images: []Image{{Path: "current.png"}}})
	if err == nil || !strings.Contains(err.Error(), "produced no output") {
		t.Fatalf("expected missing output error, got %v", err)
	}
}

func TestCodexCLIProviderReturnsTimeoutAndNonzeroErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake uses POSIX sh")
	}
	dir := t.TempDir()
	slow := writeFakeCodex(t, dir, `#!/bin/sh
sleep 2
`)
	provider := NewCodexCLIProvider(CodexCLIOptions{CommandPath: slow})
	_, err := provider.CompareImages(context.Background(), Request{Prompt: "Compare", Timeout: 10 * time.Millisecond, Images: []Image{{Path: "current.png"}}})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}

	failing := writeFakeCodex(t, dir, `#!/bin/sh
echo "bad args" >&2
exit 7
`)
	provider = NewCodexCLIProvider(CodexCLIOptions{CommandPath: failing})
	_, err = provider.CompareImages(context.Background(), Request{Prompt: "Compare", Images: []Image{{Path: "current.png"}}})
	if err == nil || !strings.Contains(err.Error(), "bad args") {
		t.Fatalf("expected nonzero stderr error, got %v", err)
	}
}

func TestCodexCLIProviderRejectsMissingImagePath(t *testing.T) {
	provider := NewCodexCLIProvider(CodexCLIOptions{CommandPath: "unused"})
	if _, err := provider.CompareImages(context.Background(), Request{Images: []Image{{}}}); err == nil {
		t.Fatal("expected missing image path error")
	}
}

func writeFakeCodex(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "codex-fake")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(payload)
}
