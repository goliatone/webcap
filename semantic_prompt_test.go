package webcap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSemanticDiffPromptGeneralFocusedAndPixelContext(t *testing.T) {
	prompt, metadata, err := buildSemanticDiffPrompt(SemanticDiffRequest{
		Mode:  SemanticDiffModeFocused,
		Focus: []string{"checkout button", "navigation labels"},
		PixelContext: SemanticPixelContext{
			PixelDiffImagePath: "diff.png",
			ChangedPixels:      12,
			TotalPixels:        100,
			ChangedPercent:     12,
			Threshold:          0.1,
		},
	})
	if err != nil {
		t.Fatalf("buildSemanticDiffPrompt returned error: %v", err)
	}
	for _, expected := range []string{"Mode: focused", "- checkout button", "- navigation labels", "Pixel diff context", `"summary"`} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to contain %q:\n%s", expected, prompt)
		}
	}
	if metadata.PromptSource != "focus" || !metadata.StructuredOutput || !metadata.PixelContextIncluded {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
}

func TestBuildSemanticDiffPromptUsesPromptFileAndExplicitPrompt(t *testing.T) {
	dir := t.TempDir()
	promptPath := filepath.Join(dir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("Prompt file instruction."), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	prompt, metadata, err := buildSemanticDiffPrompt(SemanticDiffRequest{
		Mode:       SemanticDiffModeCustom,
		PromptPath: promptPath,
		Prompt:     "Explicit instruction.",
	})
	if err != nil {
		t.Fatalf("buildSemanticDiffPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "Prompt file instruction.") || !strings.Contains(prompt, "Explicit instruction.") {
		t.Fatalf("expected prompt to include file and explicit prompt:\n%s", prompt)
	}
	if metadata.PromptSource != "prompt" {
		t.Fatalf("expected explicit prompt source to win, got %s", metadata.PromptSource)
	}
}
