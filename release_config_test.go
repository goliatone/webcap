package webcap

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGoReleaserInjectsVersionMetadata(t *testing.T) {
	data, err := os.ReadFile(".goreleaser.yml")
	if err != nil {
		t.Fatalf("read .goreleaser.yml: %v", err)
	}

	var config struct {
		Builds []struct {
			Main    string   `yaml:"main"`
			Binary  string   `yaml:"binary"`
			LDFlags []string `yaml:"ldflags"`
		} `yaml:"builds"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse .goreleaser.yml: %v", err)
	}

	var webcapBuild *struct {
		Main    string   `yaml:"main"`
		Binary  string   `yaml:"binary"`
		LDFlags []string `yaml:"ldflags"`
	}
	for i := range config.Builds {
		if config.Builds[i].Binary == "webcap" && config.Builds[i].Main == "./cmd/webcap" {
			webcapBuild = &config.Builds[i]
			break
		}
	}
	if webcapBuild == nil {
		t.Fatal("expected .goreleaser.yml to define the webcap ./cmd/webcap build")
	}

	ldflags := strings.Join(webcapBuild.LDFlags, " ")
	for _, expected := range []string{
		"-X github.com/goliatone/webcap/pkg/version.Tag={{ .Version }}",
		"-X github.com/goliatone/webcap/pkg/version.Time={{ .Date }}",
		"-X github.com/goliatone/webcap/pkg/version.User=goreleaser",
		"-X github.com/goliatone/webcap/pkg/version.Commit={{ .FullCommit }}",
	} {
		if !strings.Contains(ldflags, expected) {
			t.Fatalf("expected GoReleaser ldflags to contain %q, got %q", expected, ldflags)
		}
	}
}

func TestGoReleaserPublishesHomebrewCask(t *testing.T) {
	data, err := os.ReadFile(".goreleaser.yml")
	if err != nil {
		t.Fatalf("read .goreleaser.yml: %v", err)
	}

	var config struct {
		Brews         []any `yaml:"brews"`
		HomebrewCask []struct {
			Name        string   `yaml:"name"`
			Binaries    []string `yaml:"binaries"`
			Directory   string   `yaml:"directory"`
			Description string   `yaml:"description"`
			Homepage    string   `yaml:"homepage"`
		} `yaml:"homebrew_casks"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse .goreleaser.yml: %v", err)
	}
	if len(config.Brews) != 0 {
		t.Fatal("expected GoReleaser config to use homebrew_casks instead of deprecated brews")
	}

	for _, cask := range config.HomebrewCask {
		if cask.Name != "webcap" {
			continue
		}
		if cask.Directory != "Casks" {
			t.Fatalf("expected webcap cask directory to be Casks, got %q", cask.Directory)
		}
		if !containsString(cask.Binaries, "webcap") {
			t.Fatalf("expected webcap cask to install webcap binary, got %v", cask.Binaries)
		}
		if cask.Description == "" || cask.Homepage == "" {
			t.Fatalf("expected webcap cask metadata to include description and homepage, got %+v", cask)
		}
		return
	}

	t.Fatal("expected .goreleaser.yml to define a webcap homebrew cask")
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
