package skills

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type InstallRequest struct {
	Agent       Agent
	SkillName   string
	Source      fs.FS
	SourceDir   string
	HomeDir     string
	Destination string
}

type InstallResult struct {
	Agent        Agent  `json:"agent"`
	SkillName    string `json:"skill_name"`
	Destination  string `json:"destination"`
	FilesWritten int    `json:"files_written"`
}

// Install copies a skill directory from Source into the selected agent's skill directory.
func Install(ctx context.Context, req InstallRequest) (InstallResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req.Agent = normalizeAgent(req.Agent)
	if err := validateAgent(req.Agent); err != nil {
		return InstallResult{}, err
	}
	req.SkillName = strings.TrimSpace(req.SkillName)
	if req.SkillName == "" {
		return InstallResult{}, fmt.Errorf("skill name is required")
	}
	if req.Source == nil {
		return InstallResult{}, fmt.Errorf("source filesystem is required")
	}

	sourceDir, err := cleanSourceDir(req.SourceDir)
	if err != nil {
		return InstallResult{}, err
	}
	if err := validateSkillSource(req.Source, sourceDir); err != nil {
		return InstallResult{}, err
	}

	destination, err := resolveDestination(req)
	if err != nil {
		return InstallResult{}, err
	}

	result := InstallResult{
		Agent:       req.Agent,
		SkillName:   req.SkillName,
		Destination: destination,
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return InstallResult{}, fmt.Errorf("create destination %q: %w", destination, err)
	}

	err = fs.WalkDir(req.Source, sourceDir, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if sourcePath == sourceDir {
			return nil
		}

		relPath, err := relativeSourcePath(sourceDir, sourcePath)
		if err != nil {
			return fmt.Errorf("resolve relative source path %q: %w", sourcePath, err)
		}
		if relPath == "." || relPath == "" {
			return nil
		}

		targetPath := filepath.Join(destination, filepath.FromSlash(relPath))
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("read source info %q: %w", sourcePath, err)
		}
		if entry.IsDir() {
			return os.MkdirAll(targetPath, dirPerm(info.Mode()))
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := copyFile(req.Source, sourcePath, targetPath, filePerm(info.Mode())); err != nil {
			return err
		}
		result.FilesWritten++
		return nil
	})
	if err != nil {
		return InstallResult{}, err
	}
	return result, nil
}

func relativeSourcePath(sourceDir, sourcePath string) (string, error) {
	if sourceDir == "." {
		return sourcePath, nil
	}
	if sourcePath == sourceDir {
		return ".", nil
	}
	prefix := sourceDir + "/"
	if !strings.HasPrefix(sourcePath, prefix) {
		return "", fmt.Errorf("source path is outside source directory")
	}
	return strings.TrimPrefix(sourcePath, prefix), nil
}

func cleanSourceDir(sourceDir string) (string, error) {
	sourceDir = strings.TrimSpace(sourceDir)
	if sourceDir == "" || sourceDir == "." {
		return ".", nil
	}
	sourceDir = path.Clean(filepath.ToSlash(sourceDir))
	if sourceDir == "." {
		return ".", nil
	}
	if !fs.ValidPath(sourceDir) {
		return "", fmt.Errorf("invalid source directory %q", sourceDir)
	}
	return sourceDir, nil
}

func validateSkillSource(source fs.FS, sourceDir string) error {
	skillPath := path.Join(sourceDir, "SKILL.md")
	info, err := fs.Stat(source, skillPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("skill source %q must contain SKILL.md", sourceDir)
		}
		return fmt.Errorf("validate skill source %q: %w", sourceDir, err)
	}
	if info.IsDir() {
		return fmt.Errorf("skill source %q has directory SKILL.md, expected file", sourceDir)
	}
	return nil
}

func resolveDestination(req InstallRequest) (string, error) {
	if destination := strings.TrimSpace(req.Destination); destination != "" {
		return destination, nil
	}
	homeDir := strings.TrimSpace(req.HomeDir)
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
	}
	return DestinationFor(req.Agent, homeDir, req.SkillName)
}

func copyFile(source fs.FS, sourcePath, targetPath string, perm fs.FileMode) error {
	in, err := source.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", sourcePath, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create parent directory for %q: %w", targetPath, err)
	}
	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("open destination file %q: %w", targetPath, err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("copy %q to %q: %w", sourcePath, targetPath, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close destination file %q: %w", targetPath, closeErr)
	}
	return nil
}

func dirPerm(mode fs.FileMode) fs.FileMode {
	perm := mode.Perm()
	if perm == 0 {
		return 0o755
	}
	return perm | 0o700
}

func filePerm(mode fs.FileMode) fs.FileMode {
	perm := mode.Perm()
	if perm == 0 {
		return 0o644
	}
	return perm | 0o600
}
