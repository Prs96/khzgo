package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRecursive(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("A/album1/02 song.flac")
	mk("A/album1/01 intro.mp3")
	mk("B/track.opus")
	mk("B/.hidden/nested.mp3")
	mk("C/.DS_Store.txt")
	mk("C/notes.md")

	files, err := ScanRecursive(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range files {
		if f.IsDir {
			t.Fatalf("unexpected dir entry: %s", f.Path)
		}
		got = append(got, f.Path)
	}
	want := []string{
		filepath.Join(root, "A", "album1", "01 intro.mp3"),
		filepath.Join(root, "A", "album1", "02 song.flac"),
		filepath.Join(root, "B", "track.opus"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry %d: got %s want %s", i, got[i], want[i])
		}
	}
}

func TestScanRecursiveRootMissing(t *testing.T) {
	if _, err := ScanRecursive("/nonexistent/khzgo/test"); err == nil {
		t.Fatal("expected error for missing root")
	}
}
