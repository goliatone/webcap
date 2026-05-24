package llms

import (
	"fmt"
	"strings"
)

type ExecutionError struct {
	Provider  string
	Command   string
	Args      []string
	ExitCode  int
	Stdout    string
	Stderr    string
	TimedOut  bool
	Cancelled bool
	Err       error
}

type PayloadBudgetError struct {
	Provider    string
	LimitName   string
	LimitValue  int64
	ActualValue int64
}

func (e *PayloadBudgetError) Error() string {
	if e == nil {
		return ""
	}
	provider := strings.TrimSpace(e.Provider)
	if provider == "" {
		provider = "provider"
	}
	return fmt.Sprintf("%s request exceeds %s budget: %d > %d", provider, e.LimitName, e.ActualValue, e.LimitValue)
}

func (e *ExecutionError) Error() string {
	if e == nil {
		return ""
	}
	var parts []string
	provider := strings.TrimSpace(e.Provider)
	if provider == "" {
		provider = "provider"
	}
	command := strings.TrimSpace(e.Command)
	if command == "" {
		command = "command"
	}
	if e.TimedOut {
		parts = append(parts, "timed out")
	}
	if e.Cancelled {
		parts = append(parts, "cancelled")
	}
	if e.ExitCode != 0 {
		parts = append(parts, fmt.Sprintf("exit code %d", e.ExitCode))
	}
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}
	if stderr := strings.TrimSpace(e.Stderr); stderr != "" {
		parts = append(parts, stderr)
	}
	if len(parts) == 0 {
		parts = append(parts, "execution failed")
	}
	return provider + " " + command + " failed: " + strings.Join(parts, ": ")
}

func (e *ExecutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
