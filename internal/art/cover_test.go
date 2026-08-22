package art

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActiveProtocolDefaultsToSymbols(t *testing.T) {
	t.Setenv("KHZGO_ART", "")
	if got := ActiveProtocol(); got != "symbols" {
		t.Fatalf("expected symbols by default, got %q", got)
	}
}

func TestActiveProtocolIgnoresTerminalDetection(t *testing.T) {
	t.Setenv("KHZGO_ART", "")
	t.Setenv("KITTY_WINDOW_ID", "1")
	t.Setenv("TERM", "xterm-kitty")
	if got := ActiveProtocol(); got != "symbols" {
		t.Fatalf("image protocols must be opt-in, got %q", got)
	}
}

func TestActiveProtocolEnvOverride(t *testing.T) {
	t.Setenv("KHZGO_ART", "symbols")
	if got := ActiveProtocol(); got != "symbols" {
		t.Fatalf("expected symbols, got %q", got)
	}

	t.Setenv("KHZGO_ART", "kitty")
	if got := ActiveProtocol(); got != "kitty" {
		t.Fatalf("expected kitty, got %q", got)
	}
}

func TestNormalizeBlockPadsToHeight(t *testing.T) {
	got := normalizeBlock("row1\nrow2\nrow3", 10, 5, "symbols")
	lines := strings.Count(got, "\n") + 1
	if lines != 5 {
		t.Fatalf("expected 5 lines, got %d (%q)", lines, got)
	}
	if strings.Contains(got, "\x1b_Ga=d") {
		t.Fatal("symbols output should not include delete sequence")
	}
}

func TestNormalizeBlockKittyDeletesPreviousImages(t *testing.T) {
	got := normalizeBlock("\x1b_Gf=32...\x1b\\", 20, 8, "kitty")
	if !strings.HasPrefix(got, "\x1b_Ga=d\x1b\\") {
		t.Fatalf("expected kitty delete-all prefix, got %q", got[:min(30, len(got))])
	}
	if strings.Count(strings.TrimPrefix(got, "\x1b_Ga=d\x1b\\"), "\n")+1 != 8 {
		t.Fatalf("expected padded height for image protocol, got %q", got)
	}
}

func TestFindSidecarMatchesPartialCoverName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Michael Jackson - Dangerous.jpg")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	if got := findSidecar(dir); got != path {
		t.Fatalf("expected %q, got %q", path, got)
	}
}

func TestFindSidecarFallsBackToSingleImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "album-art.png")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	if got := findSidecar(dir); got != path {
		t.Fatalf("expected %q, got %q", path, got)
	}
}

func TestFindSidecarSkipsAmbiguousGenericImages(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.png")
	second := filepath.Join(dir, "b.jpg")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write sidecar: %v", err)
		}
	}

	if got := findSidecar(dir); got != "" {
		t.Fatalf("expected no sidecar, got %q", got)
	}
}
