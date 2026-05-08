package main

import (
	"context"
	"io"
	"os"
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
