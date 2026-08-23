package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSavedStateRoundTrip(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)

	music := t.TempDir()
	cur := filepath.Join(music, "a.mp3")
	queued := filepath.Join(music, "b.flac")
	for _, p := range []string{cur, queued} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gone := filepath.Join(music, "deleted.mp3")

	st := savedState{
		Dir:      music,
		Queue:    []string{queued, gone},
		History:  []string{cur},
		Volume:   80,
		Repeat:   true,
		Current:  cur,
		Position: 42.5,
		Paused:   true,
	}
	data, _ := json.MarshalIndent(st, "", "  ")

	cfgDir := filepath.Join(cfgHome, "khzgo")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfgDir, "state.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	gotPath, err := statePath()
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("statePath = %s want %s", gotPath, path)
	}

	got := loadSavedState("/fallback")
	if got.Dir != music || got.Volume != 80 || !got.Repeat ||
		got.Current != cur || got.Position != 42.5 || !got.Paused {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if len(got.Queue) != 1 || got.Queue[0] != queued {
		t.Fatalf("queue mismatch: %v", got.Queue)
	}
	if len(got.History) != 1 || got.History[0] != cur {
		t.Fatalf("history mismatch: %v", got.History)
	}
}

func TestLoadSavedStateMissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	got := loadSavedState("/fallback")
	if got.Dir != "/fallback" {
		t.Fatalf("expected fallback dir, got %+v", got)
	}
}

func TestLoadSavedStateStalePaths(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)

	st := savedState{Dir: "/nonexistent/dir", Current: "/nonexistent/track.mp3"}
	data, _ := json.Marshal(st)
	cfgDir := filepath.Join(cfgHome, "khzgo")
	os.MkdirAll(cfgDir, 0o755)
	os.WriteFile(filepath.Join(cfgDir, "state.json"), data, 0o644)

	got := loadSavedState("/fallback")
	if got.Dir != "/fallback" {
		t.Fatalf("expected fallback dir for stale state, got %+v", got)
	}
	if got.Current != "" || got.Position != 0 || got.Paused {
		t.Fatalf("expected stale current track dropped, got %+v", got)
	}
}
