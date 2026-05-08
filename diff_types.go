package webcap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DiffMode string

const (
	DiffModeImage     DiffMode = "image"
	DiffModeDirectory DiffMode = "directory"
)

type DiffRequest struct {
	BasePath     string  `json:"base_path"`
	ComparePath  string  `json:"compare_path"`
	OutputPath   string  `json:"output,omitempty"`
	MetadataPath string  `json:"metadata,omitempty"`
	Threshold    float64 `json:"threshold,omitempty"`
}

type DiffEntry struct {
	RelativePath   string           `json:"relative_path,omitempty"`
	BasePath       string           `json:"base_path"`
	ComparePath    string           `json:"compare_path"`
	OutputPath     string           `json:"output_path,omitempty"`
	MetadataPath   string           `json:"metadata_path,omitempty"`
	ByteSize       int              `json:"byte_size,omitempty"`
	Width          int              `json:"width,omitempty"`
	Height         int              `json:"height,omitempty"`
	TotalPixels    int              `json:"total_pixels,omitempty"`
	ChangedPixels  int              `json:"changed_pixels,omitempty"`
	ChangedPercent float64          `json:"changed_percent,omitempty"`
	Threshold      float64          `json:"threshold"`
	Changed        bool             `json:"changed"`
	MissingBase    bool             `json:"missing_base,omitempty"`
	MissingCompare bool             `json:"missing_compare,omitempty"`
	Warnings       []CaptureWarning `json:"warnings,omitempty"`
}

type DiffSummary struct {
	ComparedFiles       int              `json:"compared_files"`
	ChangedFiles        int              `json:"changed_files"`
	MissingBaseFiles    int              `json:"missing_base_files"`
	MissingCompareFiles int              `json:"missing_compare_files"`
	TotalChangedPixels  int              `json:"total_changed_pixels"`
	Warnings            []CaptureWarning `json:"warnings,omitempty"`
}

type DiffResult struct {
	Mode         DiffMode    `json:"mode"`
	BasePath     string      `json:"base_path"`
	ComparePath  string      `json:"compare_path"`
	OutputPath   string      `json:"output_path"`
	MetadataPath string      `json:"metadata_path,omitempty"`
	Threshold    float64     `json:"threshold"`
	Entry        *DiffEntry  `json:"entry,omitempty"`
	Entries      []DiffEntry `json:"entries,omitempty"`
	Summary      DiffSummary `json:"summary"`
	CreatedAt    time.Time   `json:"created_at"`
}

type DiffService interface {
	Diff(ctx context.Context, req DiffRequest) (DiffResult, error)
}

func NormalizeDiffRequest(req DiffRequest) (DiffRequest, error) {
	req.BasePath = strings.TrimSpace(req.BasePath)
	req.ComparePath = strings.TrimSpace(req.ComparePath)
	req.OutputPath = strings.TrimSpace(req.OutputPath)
	req.MetadataPath = strings.TrimSpace(req.MetadataPath)

	if req.BasePath == "" {
		return DiffRequest{}, newCaptureError(CodeValidation, "normalize_diff_request", "base path is required", nil)
	}
	if req.ComparePath == "" {
		return DiffRequest{}, newCaptureError(CodeValidation, "normalize_diff_request", "compare path is required", nil)
	}
	if req.Threshold < 0 || req.Threshold > 1 {
		return DiffRequest{}, newCaptureError(CodeValidation, "normalize_diff_request", "threshold must be between 0 and 1", nil)
	}
	return req, nil
}

func ResolveDiffPaths(req DiffRequest, mode DiffMode) (DiffRequest, error) {
	req, err := NormalizeDiffRequest(req)
	if err != nil {
		return DiffRequest{}, err
	}

	if req.OutputPath == "" {
		switch mode {
		case DiffModeDirectory:
			req.OutputPath = filepath.Join(defaultOutputDirectory, DefaultDiffDirectoryName(req))
		default:
			req.OutputPath = filepath.Join(defaultOutputDirectory, DefaultDiffImageName(req)+".png")
		}
	}

	if req.MetadataPath == "" {
		switch mode {
		case DiffModeDirectory:
			req.MetadataPath = filepath.Join(req.OutputPath, "diff-summary.json")
		default:
			req.MetadataPath = req.OutputPath + ".json"
		}
	}
	return req, nil
}

func InferDiffMode(basePath, comparePath string) (DiffMode, error) {
	baseInfo, err := os.Stat(strings.TrimSpace(basePath))
	if err != nil {
		return "", wrapCaptureError("diff_stat_base", err)
	}
	compareInfo, err := os.Stat(strings.TrimSpace(comparePath))
	if err != nil {
		return "", wrapCaptureError("diff_stat_compare", err)
	}
	if baseInfo.IsDir() != compareInfo.IsDir() {
		return "", newCaptureError(CodeValidation, "infer_diff_mode", "base and compare paths must both be files or both be directories", nil)
	}
	if baseInfo.IsDir() {
		return DiffModeDirectory, nil
	}
	return DiffModeImage, nil
}

func DefaultDiffImageName(req DiffRequest) string {
	return fmt.Sprintf("%s-vs-%s", sanitizeName(filepath.Base(req.BasePath)), sanitizeName(filepath.Base(req.ComparePath)))
}

func DefaultDiffDirectoryName(req DiffRequest) string {
	return fmt.Sprintf("%s-vs-%s", sanitizeName(filepath.Base(req.BasePath)), sanitizeName(filepath.Base(req.ComparePath)))
}
