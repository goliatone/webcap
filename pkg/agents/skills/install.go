package skills

import (
	"bytes"
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
	Force       bool
}

type InstallResult struct {
	Agent        Agent  `json:"agent"`
	SkillName    string `json:"skill_name"`
	Destination  string `json:"destination"`
	FilesWritten int    `json:"files_written"`
}

type ConflictError struct {
	Path string
}

func (e ConflictError) Error() string {
	return fmt.Sprintf("installed skill file %q already exists with different content; rerun with --force to replace it", e.Path)
}

type installFile struct {
	sourcePath string
	relPath    string
	mode       fs.FileMode
}

type installDir struct {
	relPath string
	mode    fs.FileMode
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
	if err := validateSkillName(req.SkillName); err != nil {
		return InstallResult{}, err
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
	dirs := make([]installDir, 0)
	files := make([]installFile, 0)
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

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("read source info %q: %w", sourcePath, err)
		}
		if entry.IsDir() {
			dirs = append(dirs, installDir{
				relPath: filepath.FromSlash(relPath),
				mode:    dirPerm(info.Mode()),
			})
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		files = append(files, installFile{
			sourcePath: sourcePath,
			relPath:    filepath.FromSlash(relPath),
			mode:       filePerm(info.Mode()),
		})
		return nil
	})
	if err != nil {
		return InstallResult{}, err
	}
	if err := os.MkdirAll(destination, 0o750); err != nil {
		return InstallResult{}, fmt.Errorf("create destination %q: %w", destination, err)
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return InstallResult{}, fmt.Errorf("open destination root %q: %w", destination, err)
	}
	defer func() {
		_ = root.Close()
	}()
	if err := checkConflicts(root, req.Source, files, destination, req.Force); err != nil {
		return InstallResult{}, err
	}
	for _, dir := range dirs {
		if err := root.MkdirAll(dir.relPath, dir.mode); err != nil {
			return InstallResult{}, fmt.Errorf("create destination directory %q: %w", filepath.Join(destination, dir.relPath), err)
		}
	}
	for _, file := range files {
		written, err := copyFileIfNeeded(root, req.Source, file.sourcePath, file.relPath, file.mode, req.Force)
		if err != nil {
			return InstallResult{}, err
		}
		if written {
			result.FilesWritten++
		}
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
	if !info.Mode().IsRegular() {
		return fmt.Errorf("skill source %q has non-regular SKILL.md, expected regular file", sourceDir)
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

func checkConflicts(root *os.Root, source fs.FS, files []installFile, destination string, force bool) error {
	if force {
		return nil
	}
	for _, file := range files {
		targetPath := filepath.Join(destination, file.relPath)
		targetInfo, err := root.Stat(file.relPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("stat destination file %q: %w", targetPath, err)
		}
		if !targetInfo.Mode().IsRegular() {
			return ConflictError{Path: targetPath}
		}
		same, err := sameFileContent(root, source, file.sourcePath, file.relPath)
		if err != nil {
			return err
		}
		if !same {
			return ConflictError{Path: targetPath}
		}
	}
	return nil
}

func copyFileIfNeeded(root *os.Root, source fs.FS, sourcePath, targetPath string, perm fs.FileMode, force bool) (bool, error) {
	if !force {
		same, err := sameRegularFileContent(root, source, sourcePath, targetPath)
		if err != nil {
			return false, err
		}
		if same {
			return false, nil
		}
	}
	if err := copyFile(root, source, sourcePath, targetPath, perm); err != nil {
		return false, err
	}
	return true, nil
}

func sameRegularFileContent(root *os.Root, source fs.FS, sourcePath, targetPath string) (bool, error) {
	info, err := root.Stat(targetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat destination file %q: %w", targetPath, err)
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	return sameFileContent(root, source, sourcePath, targetPath)
}

func sameFileContent(root *os.Root, source fs.FS, sourcePath, targetPath string) (bool, error) {
	sourceBytes, err := fs.ReadFile(source, sourcePath)
	if err != nil {
		return false, fmt.Errorf("read source file %q: %w", sourcePath, err)
	}
	targetBytes, err := root.ReadFile(targetPath)
	if err != nil {
		return false, fmt.Errorf("read destination file %q: %w", targetPath, err)
	}
	return bytes.Equal(sourceBytes, targetBytes), nil
}

func copyFile(root *os.Root, source fs.FS, sourcePath, targetPath string, perm fs.FileMode) (err error) {
	in, err := source.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", sourcePath, err)
	}
	defer func() {
		if closeErr := in.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close source file %q: %w", sourcePath, closeErr)
		}
	}()

	if parent := filepath.Dir(targetPath); parent != "." {
		if err := root.MkdirAll(parent, 0o750); err != nil {
			return fmt.Errorf("create parent directory for %q: %w", targetPath, err)
		}
	}
	out, err := root.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
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
