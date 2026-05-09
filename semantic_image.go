package webcap

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type SemanticImagePayload struct {
	Role       string `json:"role"`
	Path       string `json:"path"`
	MIMEType   string `json:"mime_type"`
	Base64Data string `json:"-"`
	ByteSize   int64  `json:"byte_size"`
}

func prepareSemanticImagePayload(path, role string, maxBytes int64) (SemanticImagePayload, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return SemanticImagePayload{}, newCaptureError(CodeValidation, "prepare_semantic_image", "image path is required", nil)
	}
	if maxBytes <= 0 {
		maxBytes = defaultSemanticDiffMaxImageBytes
	}
	info, err := os.Stat(path)
	if err != nil {
		return SemanticImagePayload{}, wrapCaptureError("stat_semantic_image", err)
	}
	if info.IsDir() {
		return SemanticImagePayload{}, newCaptureError(CodeValidation, "prepare_semantic_image", "semantic diff image path must be a file", nil)
	}
	if info.Size() > maxBytes {
		return SemanticImagePayload{}, newCaptureError(CodeValidation, "prepare_semantic_image", fmt.Sprintf("semantic diff image %s exceeds %d byte limit", path, maxBytes), nil)
	}
	mimeType, err := semanticImageMIMEType(path)
	if err != nil {
		return SemanticImagePayload{}, err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return SemanticImagePayload{}, wrapCaptureError("read_semantic_image", err)
	}
	if detected := http.DetectContentType(payload); strings.HasPrefix(detected, "image/") && detected != "application/octet-stream" {
		if detected == "image/png" || detected == "image/jpeg" {
			mimeType = detected
		}
	}
	return SemanticImagePayload{
		Role:       strings.TrimSpace(role),
		Path:       path,
		MIMEType:   mimeType,
		Base64Data: base64.StdEncoding.EncodeToString(payload),
		ByteSize:   int64(len(payload)),
	}, nil
}

func semanticImageMIMEType(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png", nil
	case ".jpg", ".jpeg":
		return "image/jpeg", nil
	default:
		return "", newCaptureError(CodeValidation, "prepare_semantic_image", "semantic diff supports PNG, JPG, and JPEG images", nil)
	}
}
