package llms

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testProvider struct{ name string }

func (p testProvider) Name() string { return p.name }

func (p testProvider) CompareImages(context.Context, Request) (Response, error) {
	return Response{}, nil
}

func TestNormalizeProviderNameAndClones(t *testing.T) {
	if got := NormalizeProviderName(" OpenAI "); got != ProviderOpenAI {
		t.Fatalf("unexpected normalized name: %q", got)
	}
	models := CloneStringMap(map[string]string{" OPENAI ": " gpt-test ", "": "ignored"})
	if len(models) != 1 || models[ProviderOpenAI] != "gpt-test" {
		t.Fatalf("unexpected model map: %#v", models)
	}
	metadata := CloneMetadata(map[string]string{" raw-key ": " value ", "": "ignored"})
	if len(metadata) != 1 || metadata["raw-key"] != "value" {
		t.Fatalf("unexpected metadata map: %#v", metadata)
	}
	providers := CloneProviders(map[string]Provider{" ANTHROPIC ": testProvider{name: ProviderOpenAI}, "": testProvider{name: ProviderCodexCLI}})
	if providers[ProviderAnthropic] == nil || providers[ProviderCodexCLI] == nil {
		t.Fatalf("unexpected provider map: %#v", providers)
	}
}

func TestResolveCredentialUsesEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", " test-key ")
	key, err := ResolveCredential(context.Background(), nil, ProviderOpenAI)
	if err != nil {
		t.Fatalf("ResolveCredential returned error: %v", err)
	}
	if key != "test-key" {
		t.Fatalf("unexpected key: %q", key)
	}
}

func TestPackageDoesNotImportRootWebcap(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		payload, err := os.ReadFile(filepath.Join(".", entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		rootImport := `"github.com/goliatone/` + `webcap"`
		if strings.Contains(string(payload), rootImport) {
			t.Fatalf("%s imports root webcap package", entry.Name())
		}
	}
}
