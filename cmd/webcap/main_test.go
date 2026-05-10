package main

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goliatone/webcap/pkg/version"
)

func TestRunHelpCLI(t *testing.T) {
	invocation, err := parseCLI([]string{"help"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	var runErr error
	output := captureStdout(t, func() {
		runErr = run(context.Background(), invocation)
	})
	if runErr != nil {
		t.Fatalf("run returned error: %v", runErr)
	}
	for _, expected := range []string{
		"Usage:",
		"webcap help",
		"webcap version",
		"webcap shot [flags] <url>",
		"webcap mcp serve [flags]",
		"webcap skill install --agent <codex|claude>",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected help output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunVersionCLI(t *testing.T) {
	oldTag, oldTime, oldUser, oldCommit := version.Tag, version.Time, version.User, version.Commit
	t.Cleanup(func() {
		version.Tag = oldTag
		version.Time = oldTime
		version.User = oldUser
		version.Commit = oldCommit
	})
	version.Tag = "0.1.0"
	version.Time = "2026-05-08T00:00:00Z"
	version.User = "builder"
	version.Commit = "abc123"

	invocation, err := parseCLI([]string{"version"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	var runErr error
	output := captureStdout(t, func() {
		runErr = run(context.Background(), invocation)
	})
	if runErr != nil {
		t.Fatalf("run returned error: %v", runErr)
	}
	for _, expected := range []string{
		"Version:",
		"0.1.0",
		"Build Commit Hash:",
		"abc123",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected version output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunSkillInstallUsesEmbeddedAssets(t *testing.T) {
	tests := []struct {
		agent       string
		destination func(string) string
	}{
		{
			agent: "codex",
			destination: func(home string) string {
				return filepath.Join(home, ".agents", "skills", "webcap-agent")
			},
		},
		{
			agent: "claude",
			destination: func(home string) string {
				return filepath.Join(home, ".claude", "skills", "webcap-agent")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			invocation, err := parseCLI([]string{"skill", "install", "--agent", tt.agent})
			if err != nil {
				t.Fatalf("parseCLI returned error: %v", err)
			}

			var runErr error
			output := captureStdout(t, func() {
				runErr = run(context.Background(), invocation)
			})
			if runErr != nil {
				t.Fatalf("run returned error: %v", runErr)
			}
			destination := tt.destination(home)
			for _, expected := range []string{
				`"agent": "` + tt.agent + `"`,
				`"skill_name": "webcap-agent"`,
				destination,
			} {
				if !strings.Contains(output, expected) {
					t.Fatalf("expected skill install output to contain %q, got:\n%s", expected, output)
				}
			}
			for _, path := range []string{
				filepath.Join(destination, "SKILL.md"),
				filepath.Join(destination, "references", "cli.md"),
				filepath.Join(destination, "references", "visual-review.md"),
			} {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("expected installed asset %s: %v", path, err)
				}
			}
		})
	}
}

func TestRunSemanticDiffUsesDefaultBuiltInProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("expected OpenAI auth header, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"summary\":\"CLI semantic ok\",\"verdict\":\"no_meaningful_change\",\"severity\":\"info\"}"}]}]}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	writeMainTestPNG(t, currentPath, color.NRGBA{R: 255, A: 255})
	writeMainTestPNG(t, referencePath, color.NRGBA{B: 255, A: 255})
	t.Setenv("OPENAI_API_KEY", "test-key")

	invocation, err := parseCLI([]string{
		"semantic-diff",
		"--provider", "openai",
		"--model", "gpt-test",
		"--openai-base-url", server.URL,
		"--metadata", filepath.Join(dir, "semantic.json"),
		currentPath,
		referencePath,
	})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	var runErr error
	output := captureStdout(t, func() {
		runErr = run(context.Background(), invocation)
	})
	if runErr != nil {
		t.Fatalf("run returned error: %v", runErr)
	}
	if !strings.Contains(output, "CLI semantic ok") {
		t.Fatalf("expected semantic summary in output, got:\n%s", output)
	}
}

func TestRunSemanticDiffUsesCodexCLIProvider(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake uses POSIX sh")
	}
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	writeMainTestPNG(t, currentPath, color.NRGBA{R: 255, A: 255})
	writeMainTestPNG(t, referencePath, color.NRGBA{B: 255, A: 255})
	fake := writeMainFakeCodex(t, dir, `#!/bin/sh
cat >/dev/null
printf '{"summary":"CLI codex ok","verdict":"no_meaningful_change","severity":"info"}\n'
`)

	invocation, err := parseCLI([]string{
		"semantic-diff",
		"--provider", "codex-cli",
		"--model", "gpt-test",
		"--codex-bin", fake,
		"--metadata", filepath.Join(dir, "semantic.json"),
		currentPath,
		referencePath,
	})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	var runErr error
	output := captureStdout(t, func() {
		runErr = run(context.Background(), invocation)
	})
	if runErr != nil {
		t.Fatalf("run returned error: %v", runErr)
	}
	if !strings.Contains(output, "CLI codex ok") {
		t.Fatalf("expected codex semantic summary in output, got:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	defer func() {
		os.Stdout = original
		_ = reader.Close()
	}()

	os.Stdout = writer
	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(output)
}

func writeMainTestPNG(t *testing.T, path string, c color.NRGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create image dir: %v", err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, c)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode image: %v", err)
	}
}

func writeMainFakeCodex(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "codex-fake")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	return path
}
