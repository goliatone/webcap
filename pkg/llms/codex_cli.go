package llms

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultCodexCommand     = "codex"
	defaultCodexStderrLimit = 4096
)

type CodexCLIProvider struct {
	options CodexCLIOptions
}

func NewCodexCLIProvider(options CodexCLIOptions) *CodexCLIProvider {
	return &CodexCLIProvider{options: normalizeCodexCLIOptions(options)}
}

func (p *CodexCLIProvider) Name() string { return ProviderCodexCLI }

func (p *CodexCLIProvider) CompareImages(ctx context.Context, req Request) (Response, error) {
	if p == nil {
		return Response{}, errors.New("codex CLI provider is not configured")
	}
	options := normalizeCodexCLIOptions(p.options)
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	} else if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	tempDir, err := os.MkdirTemp("", "webcap-codex-*")
	if err != nil {
		return Response{}, fmt.Errorf("create codex temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	args, resultPath, err := p.args(req, tempDir)
	if err != nil {
		return Response{}, err
	}
	// #nosec G204 -- this provider intentionally executes a user-configured Codex CLI with structured arguments.
	cmd := exec.CommandContext(ctx, options.CommandPath, args...)
	cmd.Dir = options.WorkingDir
	cmd.Stdin = strings.NewReader(req.Prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startedAt := time.Now().UTC()
	err = cmd.Run()
	completedAt := time.Now().UTC()
	timing := Timing{StartedAt: startedAt, CompletedAt: completedAt, Duration: completedAt.Sub(startedAt)}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Response{}, codexExecutionError(options, args, stdout.String(), stderr.String(), ctxErr, ctxErr, -1)
		}
		return Response{}, codexExecutionError(options, args, stdout.String(), stderr.String(), err, nil, -1)
	}

	raw, err := readCodexOutput(resultPath, stdout.String())
	if err != nil {
		return Response{}, codexExecutionError(options, args, stdout.String(), stderr.String(), err, nil, 0)
	}
	warnings := codexWarnings(stderr.String(), options.StderrLimit)
	return Response{
		Provider: p.Name(),
		Model:    FirstNonEmpty(req.Model, options.Model),
		RawText:  raw,
		Warnings: warnings,
		Metadata: map[string]string{
			"command": filepath.Base(options.CommandPath),
		},
		Exit:   Exit{Code: 0},
		Timing: timing,
	}, nil
}

func codexExecutionError(options CodexCLIOptions, args []string, stdout, stderr string, cause error, ctxErr error, defaultExitCode int) *ExecutionError {
	exitCode := defaultExitCode
	var exitErr *exec.ExitError
	if errors.As(cause, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	timedOut := errors.Is(ctxErr, context.DeadlineExceeded)
	cancelled := errors.Is(ctxErr, context.Canceled)
	return &ExecutionError{
		Provider:  ProviderCodexCLI,
		Command:   options.CommandPath,
		Args:      append([]string(nil), args...),
		ExitCode:  exitCode,
		Stdout:    limitString(strings.TrimSpace(stdout), options.StderrLimit),
		Stderr:    limitString(strings.TrimSpace(stderr), options.StderrLimit),
		TimedOut:  timedOut,
		Cancelled: cancelled,
		Err:       cause,
	}
}

func normalizeCodexCLIOptions(options CodexCLIOptions) CodexCLIOptions {
	options.CommandPath = strings.TrimSpace(options.CommandPath)
	if options.CommandPath == "" {
		options.CommandPath = defaultCodexCommand
	}
	options.WorkingDir = strings.TrimSpace(options.WorkingDir)
	options.Profile = strings.TrimSpace(options.Profile)
	options.Model = strings.TrimSpace(options.Model)
	options.LocalProvider = strings.TrimSpace(options.LocalProvider)
	options.OutputSchemaPath = strings.TrimSpace(options.OutputSchemaPath)
	if options.StderrLimit <= 0 {
		options.StderrLimit = defaultCodexStderrLimit
	}
	clean := make([]string, 0, len(options.ExtraArgs))
	for _, arg := range options.ExtraArgs {
		arg = strings.TrimSpace(arg)
		if arg != "" {
			clean = append(clean, arg)
		}
	}
	options.ExtraArgs = clean
	return options
}

func (p *CodexCLIProvider) args(req Request, tempDir string) ([]string, string, error) {
	options := normalizeCodexCLIOptions(p.options)
	args := []string{"exec"}
	if options.Profile != "" {
		args = append(args, "--profile", options.Profile)
	}
	if options.UseOSS {
		args = append(args, "--oss")
	}
	if options.LocalProvider != "" {
		args = append(args, "--local-provider", options.LocalProvider)
	}
	if options.Ephemeral {
		args = append(args, "--ephemeral")
	}
	if options.IgnoreRules {
		args = append(args, "--ignore-rules")
	}
	for _, image := range req.Images {
		path := strings.TrimSpace(image.Path)
		if path == "" {
			return nil, "", fmt.Errorf("codex CLI image path is required")
		}
		args = append(args, "--image", path)
	}
	if model := FirstNonEmpty(req.Model, options.Model); model != "" {
		args = append(args, "--model", model)
	}
	if req.StructuredJSON {
		schemaPath := options.OutputSchemaPath
		if schemaPath == "" {
			var err error
			schemaPath, err = writeCodexSchema(tempDir)
			if err != nil {
				return nil, "", err
			}
		}
		args = append(args, "--output-schema", schemaPath)
	}
	resultPath := filepath.Join(tempDir, "codex-result.txt")
	args = append(args, "--output-last-message", resultPath)
	args = append(args, options.ExtraArgs...)
	args = append(args, "-")
	return args, resultPath, nil
}

func writeCodexSchema(tempDir string) (string, error) {
	path := filepath.Join(tempDir, "semantic-output.schema.json")
	schema := []byte(`{"type":"object","additionalProperties":true}` + "\n")
	if err := os.WriteFile(path, schema, 0o600); err != nil {
		return "", fmt.Errorf("write codex output schema: %w", err)
	}
	return path, nil
}

func readCodexOutput(resultPath, stdout string) (string, error) {
	if strings.TrimSpace(resultPath) != "" {
		if payload, err := readFileInRoot(resultPath); err == nil {
			if text := strings.TrimSpace(string(payload)); text != "" {
				return text, nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read codex output file: %w", err)
		}
	}
	if text := strings.TrimSpace(stdout); text != "" {
		return text, nil
	}
	return "", fmt.Errorf("codex CLI produced no output")
}

func readFileInRoot(path string) ([]byte, error) {
	root, name, err := openPathRoot(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = root.Close()
	}()
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()
	return io.ReadAll(file)
}

func openPathRoot(path string) (*os.Root, string, error) {
	path = filepath.Clean(path)
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, "", err
	}
	return root, filepath.Base(path), nil
}

func codexWarnings(stderr string, limit int) []Warning {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return nil
	}
	return []Warning{{
		Code:    "codex_cli_stderr",
		Message: limitString(stderr, limit),
	}}
}

func limitString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}
