package version

import (
	"bytes"
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	Tag = "0.0.0"
	Time = "2026-05-08T00:00:00Z"
	User = "builder"

	expected := "0.0.0-2026-05-08T00:00:00Z:builder"
	if actual := GetVersion(); actual != expected {
		t.Fatalf("unexpected version string: %q", actual)
	}
}

func TestPrint(t *testing.T) {
	Tag = "0.1.0"
	Time = "2026-05-08T00:00:00Z"
	User = "builder"
	Commit = "9ae92b384895797a5b291349eb64434d74a96b81"

	var stdout bytes.Buffer
	if err := Print(&stdout); err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"webcap:",
		"Version:",
		"0.1.0",
		"Build Commit Hash:",
		"9ae92b384895797a5b291349eb64434d74a96b81",
		"Build Time:",
		"Build User:",
		"https://github.com/goliatone/webcap",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}
