package skills

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
)

func TestDestinationForCodexAndClaude(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	tests := []struct {
		name  string
		agent Agent
		want  string
	}{
		{
			name:  "codex",
			agent: AgentCodex,
			want:  filepath.Join(home, ".agents", "skills", "webcap-agent"),
		},
		{
			name:  "claude",
			agent: AgentClaude,
			want:  filepath.Join(home, ".claude", "skills", "webcap-agent"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DestinationFor(tt.agent, home, "webcap-agent")
			if err != nil {
				t.Fatalf("DestinationFor returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected destination: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestInstallCopiesNestedFilesAndIsIdempotent(t *testing.T) {
	home := t.TempDir()
	source := fstest.MapFS{
		"bundle/SKILL.md":                    {Data: []byte("---\nname: webcap-agent\n---\n")},
		"bundle/references/empty":            {Mode: fs.ModeDir},
		"bundle/references/visual-review.md": {Data: []byte("# Visual review\n")},
		"bundle/scripts/run.sh":              {Data: []byte("#!/bin/sh\n"), Mode: 0o755},
	}
	req := InstallRequest{
		Agent:     AgentCodex,
		SkillName: "webcap-agent",
		Source:    source,
		SourceDir: "bundle",
		HomeDir:   home,
	}

	first, err := Install(context.Background(), req)
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if first.FilesWritten != 3 {
		t.Fatalf("unexpected files written: %d", first.FilesWritten)
	}
	assertFileContent(t, filepath.Join(first.Destination, "SKILL.md"), "---\nname: webcap-agent\n---\n")
	assertFileContent(t, filepath.Join(first.Destination, "references", "visual-review.md"), "# Visual review\n")
	assertDirectory(t, filepath.Join(first.Destination, "references", "empty"))
	assertExecutableBit(t, filepath.Join(first.Destination, "scripts", "run.sh"))

	second, err := Install(context.Background(), req)
	if err != nil {
		t.Fatalf("repeat Install returned error: %v", err)
	}
	if second.Destination != first.Destination {
		t.Fatalf("repeat install changed destination: %q != %q", second.Destination, first.Destination)
	}
	if second.FilesWritten != 0 {
		t.Fatalf("repeat install rewrote unchanged files: %d", second.FilesWritten)
	}
	assertFileContent(t, filepath.Join(second.Destination, "SKILL.md"), "---\nname: webcap-agent\n---\n")
}

func TestInstallRejectsConflictingExistingFilesByDefault(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "webcap-agent")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	target := filepath.Join(destination, "SKILL.md")
	if err := os.WriteFile(target, []byte("# User edit\n"), 0o644); err != nil {
		t.Fatalf("write existing target: %v", err)
	}

	_, err := Install(context.Background(), InstallRequest{
		Agent:       AgentCodex,
		SkillName:   "webcap-agent",
		Source:      validSource(),
		Destination: destination,
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var conflict ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ConflictError, got %T %v", err, err)
	}
	assertFileContent(t, target, "# User edit\n")
}

func TestInstallForceReplacesConflictingExistingFiles(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "webcap-agent")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	target := filepath.Join(destination, "SKILL.md")
	if err := os.WriteFile(target, []byte("# User edit\n"), 0o644); err != nil {
		t.Fatalf("write existing target: %v", err)
	}

	result, err := Install(context.Background(), InstallRequest{
		Agent:       AgentClaude,
		SkillName:   "webcap-agent",
		Source:      validSource(),
		Destination: destination,
		Force:       true,
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.FilesWritten != 1 {
		t.Fatalf("unexpected files written: %d", result.FilesWritten)
	}
	assertFileContent(t, target, "# Skill\n")
}

func TestInstallUsesExplicitDestination(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "custom", "webcap-agent")
	result, err := Install(context.Background(), InstallRequest{
		Agent:       AgentClaude,
		SkillName:   "webcap-agent",
		Source:      validSource(),
		Destination: destination,
	})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.Destination != destination {
		t.Fatalf("unexpected destination: %s", result.Destination)
	}
	assertFileContent(t, filepath.Join(destination, "SKILL.md"), "# Skill\n")
}

func TestInstallRejectsUnsupportedAgent(t *testing.T) {
	_, err := Install(context.Background(), InstallRequest{
		Agent:     Agent("cursor"),
		SkillName: "webcap-agent",
		Source:    validSource(),
		HomeDir:   t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported agent") {
		t.Fatalf("expected unsupported agent error, got %v", err)
	}
}

func TestInstallRejectsMissingSkillMarkdown(t *testing.T) {
	_, err := Install(context.Background(), InstallRequest{
		Agent:     AgentCodex,
		SkillName: "webcap-agent",
		Source: fstest.MapFS{
			"references/cli.md": {Data: []byte("# CLI\n")},
		},
		HomeDir: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "SKILL.md") {
		t.Fatalf("expected missing SKILL.md error, got %v", err)
	}
}

func TestInstallRejectsDirectorySkillMarkdown(t *testing.T) {
	_, err := Install(context.Background(), InstallRequest{
		Agent:     AgentCodex,
		SkillName: "webcap-agent",
		Source: fstest.MapFS{
			"SKILL.md": {Mode: fs.ModeDir},
		},
		HomeDir: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "directory SKILL.md") {
		t.Fatalf("expected directory SKILL.md error, got %v", err)
	}
}

func TestInstallRejectsNonRegularSkillMarkdown(t *testing.T) {
	_, err := Install(context.Background(), InstallRequest{
		Agent:     AgentCodex,
		SkillName: "webcap-agent",
		Source: fstest.MapFS{
			"SKILL.md": {Data: []byte("# Skill\n"), Mode: fs.ModeIrregular},
		},
		HomeDir: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "non-regular SKILL.md") {
		t.Fatalf("expected non-regular SKILL.md error, got %v", err)
	}
}

func TestInstallHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Install(ctx, InstallRequest{
		Agent:     AgentCodex,
		SkillName: "webcap-agent",
		Source:    validSource(),
		HomeDir:   t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected canceled context error")
	}
}

func validSource() fstest.MapFS {
	return fstest.MapFS{
		"SKILL.md": {Data: []byte("# Skill\n")},
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("unexpected content for %s: got %q want %q", path, got, want)
	}
}

func assertDirectory(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat directory %s: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory %s, got mode %s", path, info.Mode())
	}
}

func assertExecutableBit(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected executable bit on %s, got mode %s", path, info.Mode())
	}
}
