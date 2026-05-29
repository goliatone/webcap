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
