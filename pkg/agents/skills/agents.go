package skills

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Agent identifies a supported agent runtime.
type Agent string

const (
	AgentCodex  Agent = "codex"
	AgentClaude Agent = "claude"
)

func normalizeAgent(agent Agent) Agent {
	return Agent(strings.ToLower(strings.TrimSpace(string(agent))))
}

// ParseAgent normalizes and validates a user-provided agent name.
func ParseAgent(raw string) (Agent, error) {
	agent := normalizeAgent(Agent(raw))
	if err := validateAgent(agent); err != nil {
		return "", err
	}
	return agent, nil
}

func validateAgent(agent Agent) error {
	switch normalizeAgent(agent) {
	case AgentCodex, AgentClaude:
		return nil
	default:
		return fmt.Errorf("unsupported agent %q, expected codex or claude", agent)
	}
}

func agentSkillRoot(agent Agent, homeDir string) (string, error) {
	switch normalizeAgent(agent) {
	case AgentCodex:
		return filepath.Join(homeDir, ".agents", "skills"), nil
	case AgentClaude:
		return filepath.Join(homeDir, ".claude", "skills"), nil
	default:
		return "", fmt.Errorf("unsupported agent %q, expected codex or claude", agent)
	}
}

// DestinationFor returns the default install destination for an agent and skill.
func DestinationFor(agent Agent, homeDir, skillName string) (string, error) {
	homeDir = strings.TrimSpace(homeDir)
	if homeDir == "" {
		return "", fmt.Errorf("home directory is required")
	}
	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		return "", fmt.Errorf("skill name is required")
	}
	root, err := agentSkillRoot(agent, homeDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, skillName), nil
}
