package webcap

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

func ResolvePersistedPaths(req CaptureRequest, outputDir string) (CaptureRequest, string, bool, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		outputDir = defaultOutputDirectory
	}

	baseName := DefaultArtifactBaseName(req)
	outputGenerated := false

	if strings.TrimSpace(req.OutputPath) == "" {
		req.OutputPath = filepath.Join(outputDir, baseName+"."+defaultImageFormat)
		outputGenerated = true
	}
	if filepath.Ext(req.OutputPath) == "" {
		req.OutputPath += "." + defaultImageFormat
	}
	if explicitBase := strings.TrimSuffix(filepath.Base(req.OutputPath), filepath.Ext(req.OutputPath)); explicitBase != "" {
		baseName = explicitBase
	}
	if strings.TrimSpace(req.MetadataPath) == "" {
		req.MetadataPath = req.OutputPath + ".json"
	}
	return req, baseName, outputGenerated, nil
}

func DefaultArtifactBaseName(req CaptureRequest) string {
	u, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil {
		return "capture"
	}

	host := sanitizeName(u.Hostname())
	if host == "" {
		host = "capture"
	}
	pagePath := sanitizeName(strings.Trim(path.Clean(u.EscapedPath()), "/"))
	if pagePath == "" || pagePath == "." {
		pagePath = "home"
	}

	mode := string(req.Mode())
	base := host + "-" + pagePath + "-" + sanitizeName(mode)

	if selectors, _ := req.TargetSelectors(); len(selectors) > 0 {
		digest := selectorDigest(selectors)
		first := sanitizeName(selectors[0])
		if first == "" {
			first = "selector"
		}
		if len(first) > 24 {
			first = first[:24]
		}
		base = fmt.Sprintf("%s-%s-%s", base, first, digest[:8])
	}
	if req.DevicePreset != "" {
		base += "-" + sanitizeName(req.DevicePreset)
	} else if req.ViewportPreset != "" {
		base += "-" + sanitizeName(req.ViewportPreset)
	}
	return strings.Trim(base, "-")
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var out []rune
	lastDash := false
	for _, r := range value {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			out = append(out, r)
			lastDash = false
			continue
		}
		if lastDash {
			continue
		}
		out = append(out, '-')
		lastDash = true
	}
	return strings.Trim(string(out), "-")
}

func selectorDigest(selectors []string) string {
	sum := sha1.Sum([]byte(strings.Join(selectors, "|")))
	return hex.EncodeToString(sum[:])
}
