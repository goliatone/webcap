package presentation

import (
	"fmt"
	"io"

	"github.com/goliatone/webcap/pkg/agents/skills"
)

func writeSkillInstall(w io.Writer, result skills.InstallResult) error {
	return writeLines(w,
		"Skill install complete",
		fmt.Sprintf("Agent: %s", result.Agent),
		fmt.Sprintf("Skill: %s", result.SkillName),
		fmt.Sprintf("Destination: %s", result.Destination),
		fmt.Sprintf("Files written: %d", result.FilesWritten),
	)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
