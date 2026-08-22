package art

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var sidecarCoverNames = []string{"cover", "folder", "front", "album", "artwork"}
var coverExts = []string{".jpg", ".jpeg", ".png", ".webp"}

func ActiveProtocol() string {
	switch p := strings.ToLower(os.Getenv("KHZGO_ART")); p {
	case "kitty", "sixels":
		return p
	}
	return "symbols"
}

func Render(trackPath string, width, height int) (string, error) {
	if trackPath == "" || width < 4 || height < 4 {
		return "", nil
	}

	source, cleanup, err := coverSource(trackPath)
	if err != nil {
		return "", err
	}
	if cleanup != nil {
		defer cleanup()
	}

	size := fmt.Sprintf("%dx%d", width, height)
	args := []string{
		"--animate=off",
		"--colors=full",
		"--optimize=5",
		"--size", size,
		"--view-size", size,
		"--align", "mid,mid",
		"--probe", "off",
		"--margin-bottom", "0",
		"--margin-right", "0",
	}

	var format string
	switch ActiveProtocol() {
	case "kitty", "sixels":
		format = ActiveProtocol()
		args = append(args, "--polite", "off")
	default:
		format = "symbols"
		args = append(args,
			"--symbols=vhalf+block",
			"--polite", "on",
			"--stretch",
		)
	}
	args = append(args, "--format="+format, source)

	out, err := exec.Command("chafa", args...).Output()
	if err != nil {
		return "", err
	}

	return normalizeBlock(string(out), width, height, format), nil
}

const kittyDeleteAll = "\x1b_Ga=d\x1b\\"

func normalizeBlock(out string, width, height int, protocol string) string {
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return out
	}

	if protocol == "kitty" {
		out = kittyDeleteAll + out
	}

	lines := strings.Count(out, "\n") + 1
	if lines < height {
		out += strings.Repeat("\n", height-lines)
	}
	if protocol == "sixels" {
		out = kittyDeleteAll + out
	}
	return out
}

func coverSource(trackPath string) (string, func(), error) {
	if embedded, cleanup, err := extractEmbedded(trackPath); err == nil {
		return embedded, cleanup, nil
	}

	if sidecar := findSidecar(filepath.Dir(trackPath)); sidecar != "" {
		return sidecar, nil, nil
	}

	return "", nil, fmt.Errorf("no cover art found")
}

func extractEmbedded(trackPath string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "khzgo-cover-*")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	outPath := filepath.Join(tmpDir, "cover.png")
	out, err := exec.Command(
		"ffmpeg",
		"-loglevel", "error",
		"-y",
		"-i", trackPath,
		"-map", "0:v:0",
		"-frames:v", "1",
		outPath,
	).CombinedOutput()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("extract embedded art: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return outPath, cleanup, nil
}

func findSidecar(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	var fallback string
	imageCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		ext := strings.ToLower(filepath.Ext(name))
		if !isCoverExt(ext) {
			continue
		}

		imagePath := filepath.Join(dir, entry.Name())
		imageCount++
		if fallback == "" {
			fallback = imagePath
		}

		for _, base := range sidecarCoverNames {
			if name == base+ext || strings.Contains(strings.TrimSuffix(name, ext), base) {
				return imagePath
			}
		}
	}

	if imageCount == 1 {
		return fallback
	}

	return ""
}

func isCoverExt(ext string) bool {
	for _, coverExt := range coverExts {
		if ext == coverExt {
			return true
		}
	}
	return false
}
