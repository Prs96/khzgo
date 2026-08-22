package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"khzgo/internal/browser"
)

func TestInitialDirectoryLoadPopulatesBrowser(t *testing.T) {
	dir := t.TempDir()
	track := filepath.Join(dir, "song.flac")
	if err := os.WriteFile(track, []byte("test"), 0o644); err != nil {
		t.Fatalf("write track: %v", err)
	}

	m := initialModel(nil, dir)
	m.browser = m.browser.SetSize(80, 20)

	cmd := m.browser.Init()
	if cmd == nil {
		t.Fatal("expected browser init command")
	}

	updatedModel, nextCmd := m.Update(cmd())
	if nextCmd != nil {
		t.Fatalf("expected no follow-up command, got %v", nextCmd)
	}

	updated := updatedModel.(model)
	if got := updated.browser.View(); !strings.Contains(got, "song.flac") {
		t.Fatalf("expected browser view to include track, got %q", got)
	}
}

func TestViewShowsCoverArtSection(t *testing.T) {
	m := initialModel(nil, t.TempDir())
	m.width = 100
	m.current = filepath.Join("Music", "Boards of Canada", "Dayvan Cowboy.flac")
	m.duration = 240
	m.pos = 60

	view := m.View()
	if !strings.Contains(view, "COVER ART") {
		t.Fatalf("expected cover art section in view, got %q", view)
	}
	if !strings.Contains(view, "NOW PLAYING") {
		t.Fatalf("expected now playing section in view, got %q", view)
	}
	if !strings.Contains(view, "DAYVAN COWBOY") && !strings.Contains(view, "Dayvan Cowboy") {
		t.Fatalf("expected track title in view, got %q", view)
	}
}

func TestViewFitsWindowHeight(t *testing.T) {
	m := initialModel(nil, t.TempDir())
	updatedModel, cmd := m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	if cmd != nil {
		t.Fatalf("expected no command on window size, got %v", cmd)
	}

	updated := updatedModel.(model)
	view := updated.View()
	lines := strings.Count(view, "\n") + 1
	if lines > 24 {
		t.Fatalf("expected view to fit window height, got %d lines", lines)
	}
	if !strings.Contains(view, "╭") {
		t.Fatalf("expected top border in view, got %q", view)
	}
}

func TestEnqueueDedupesAndPlaysRemoveFromQueue(t *testing.T) {
	m := initialModel(nil, "/tmp")
	track := filepath.Join("/music", "a.flac")

	updatedModel, _ := m.Update(browser.TrackQueuedMsg{Path: track})
	m = updatedModel.(model)
	updatedModel, _ = m.Update(browser.TrackQueuedMsg{Path: track})
	m = updatedModel.(model)

	if len(m.queue) != 1 {
		t.Fatalf("expected queue to dedupe, got %v", m.queue)
	}
	if !m.browser.IsQueued(track) {
		t.Fatal("expected browser queued set to include track")
	}

	updatedModel, _ = m.Update(browser.FileSelectedMsg{Path: track})
	m = updatedModel.(model)
	if len(m.queue) != 0 {
		t.Fatalf("expected queue cleared after playing track, got %v", m.queue)
	}
	if m.current != track {
		t.Fatalf("expected current %q, got %q", track, m.current)
	}
}

func TestQueueAdvancesOnEOFEvent(t *testing.T) {
	m := initialModel(nil, "/tmp")
	first := filepath.Join("/music", "first.flac")
	second := filepath.Join("/music", "second.flac")
	m.queue = []string{first, second}
	m.browser = m.browser.SetQueued(m.queue)

	updatedModel, cmd := m.Update(mpvEventMsg{
		Name: "end-file",
		Raw:  map[string]interface{}{"reason": "eof"},
	})
	if cmd == nil {
		t.Fatal("expected commands after eof advance")
	}

	updated := updatedModel.(model)
	if updated.current != first {
		t.Fatalf("expected first queued track to play, got %q", updated.current)
	}
	if len(updated.queue) != 1 || updated.queue[0] != second {
		t.Fatalf("expected queue to shrink to second track, got %v", updated.queue)
	}
}

func TestStopEventsDoNotAdvanceQueue(t *testing.T) {
	m := initialModel(nil, "/tmp")
	track := filepath.Join("/music", "first.flac")
	m.queue = []string{track}

	updatedModel, cmd := m.Update(mpvEventMsg{
		Name: "end-file",
		Raw:  map[string]interface{}{"reason": "stop"},
	})
	_ = cmd

	updated := updatedModel.(model)
	if updated.current == track {
		t.Fatalf("stop event should not advance queue, but current=%q", updated.current)
	}
	if len(updated.queue) != 1 {
		t.Fatalf("queue should be untouched by stop events, got %v", updated.queue)
	}
}

func TestCoverRequestDedupesAndRerendersOnChange(t *testing.T) {
	m := initialModel(nil, "/tmp")
	m.width = 100

	m2, cmd1 := m.requestCoverArt("/music/a.flac")
	if cmd1 == nil {
		t.Fatal("expected first request to render")
	}

	m3, cmd2 := m2.requestCoverArt("/music/a.flac")
	if cmd2 != nil {
		t.Fatal("expected duplicate request for same song+size to be skipped")
	}

	m4 := m3
	m4.width = 130
	_, cmd3 := m4.requestCoverArt("/music/a.flac")
	if cmd3 == nil {
		t.Fatal("expected size change to trigger re-render")
	}

	m5 := m4
	m5.coverKey = ""
	_, cmd4 := m5.requestCoverArt("/music/b.flac")
	if cmd4 == nil {
		t.Fatal("expected new song to trigger render")
	}
}

func TestStaleCoverResultIgnored(t *testing.T) {
	m := initialModel(nil, "/tmp")
	m.coverKey = "old-key"

	updatedModel, _ := m.Update(coverArtLoadedMsg{key: "new-key", path: "/x.flac", art: "ART"})
	updated := updatedModel.(model)
	if updated.cover != "" {
		t.Fatalf("stale cover result should be ignored, got %q", updated.cover)
	}
}

func TestDequeueRemovesTrack(t *testing.T) {
	m := initialModel(nil, "/tmp")
	first := filepath.Join("/music", "first.flac")
	second := filepath.Join("/music", "second.flac")
	m.queue = []string{first, second}
	m.browser = m.browser.SetQueued(m.queue)

	updatedModel, _ := m.Update(browser.TrackDequeuedMsg{Path: first})
	updated := updatedModel.(model)
	if len(updated.queue) != 1 || updated.queue[0] != second {
		t.Fatalf("expected first removed from queue, got %v", updated.queue)
	}
	if updated.browser.IsQueued(first) {
		t.Fatal("expected browser marker cleared for dequeued track")
	}

	updatedModel, _ = updated.Update(browser.TrackDequeuedMsg{Path: first})
	updated2 := updatedModel.(model)
	if len(updated2.queue) != 1 {
		t.Fatalf("expected queue untouched, got %v", updated2.queue)
	}
}

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestPlayAllKeepsFolderOrder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"c.flac", "a.flac", "b.flac"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write track: %v", err)
		}
	}

	m := initialModel(nil, dir)
	m.browser = m.browser.SetSize(80, 20)
	loadedModel, loadCmd := m.Update(m.browser.Init())
	if loadCmd != nil {
		loadedModel, _ = loadedModel.(model).Update(loadCmd())
	}
	m = loadedModel.(model)

	updatedModel, _ := m.Update(keyMsg("A"))
	updated := updatedModel.(model)

	if !strings.HasSuffix(updated.current, "a.flac") {
		t.Fatalf("expected first file alphabetically to play, got %q", updated.current)
	}
	want := []string{filepath.Join(dir, "b.flac"), filepath.Join(dir, "c.flac")}
	if len(updated.queue) != 2 || updated.queue[0] != want[0] || updated.queue[1] != want[1] {
		t.Fatalf("expected ordered queue %v, got %v", want, updated.queue)
	}
}

func TestShufflePlayUsesAllFolderTracks(t *testing.T) {
	dir := t.TempDir()
	names := map[string]bool{}
	for _, name := range []string{"one.flac", "two.flac", "three.flac", "four.flac"} {
		names[filepath.Join(dir, name)] = true
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write track: %v", err)
		}
	}

	m := initialModel(nil, dir)
	m.browser = m.browser.SetSize(80, 20)
	loadedModel, loadCmd := m.Update(m.browser.Init())
	if loadCmd != nil {
		loadedModel, _ = loadedModel.(model).Update(loadCmd())
	}
	m = loadedModel.(model)

	updatedModel, _ := m.Update(keyMsg("s"))
	updated := updatedModel.(model)

	if updated.current == "" {
		t.Fatal("expected a track to start playing")
	}
	if names[updated.current] == false {
		t.Fatalf("current %q not from folder", updated.current)
	}
	if len(updated.queue) != 3 {
		t.Fatalf("expected remaining 3 tracks queued, got %v", updated.queue)
	}
	for _, q := range append([]string{updated.current}, updated.queue...) {
		if !names[q] {
			t.Fatalf("unexpected track in playback set: %q", q)
		}
	}
}

func TestPlayFolderEmptyDirIsNoop(t *testing.T) {
	m := initialModel(nil, t.TempDir())
	updatedModel, cmd := m.Update(keyMsg("s"))
	if cmd != nil {
		t.Fatalf("expected no command, got %v", cmd)
	}
	if updatedModel.(model).current != "" {
		t.Fatal("expected nothing to play")
	}
}

func TestBindingsModalTogglesAndSwallowsKeys(t *testing.T) {
	m := initialModel(nil, t.TempDir())
	m.width, m.height = 100, 30

	openedModel, _ := m.Update(keyMsg("?"))
	m = openedModel.(model)
	if !m.showBindings {
		t.Fatal("expected modal open after ?")
	}
	view := m.View()
	if !strings.Contains(view, "KEYBINDINGS") {
		t.Fatalf("expected bindings view, got %q", view)
	}

	swallowedModel, cmd := m.Update(keyMsg("j"))
	if cmd != nil {
		t.Fatalf("expected keys swallowed in modal, got %v", cmd)
	}
	if !swallowedModel.(model).showBindings {
		t.Fatal("expected modal to stay open")
	}

	closedModel, _ := m.Update(keyMsg("?"))
	m = closedModel.(model)
	if m.showBindings {
		t.Fatal("expected modal closed after second ?")
	}
	if strings.Contains(m.View(), "KEYBINDINGS") {
		t.Fatal("expected normal view restored")
	}
}

func TestSkipNextAndPrevNavigateHistory(t *testing.T) {
	m := initialModel(nil, "/tmp")
	a := filepath.Join("/music", "a.flac")
	b := filepath.Join("/music", "b.flac")
	c := filepath.Join("/music", "c.flac")

	m.current = a
	m.queue = []string{b, c}
	m.browser = m.browser.SetQueued(m.queue)

	updatedModel, _ := m.Update(keyMsg("n"))
	m = updatedModel.(model)
	if m.current != b {
		t.Fatalf("expected b after skip next, got %q", m.current)
	}
	if len(m.queue) != 1 || m.queue[0] != c {
		t.Fatalf("expected [c] queued, got %v", m.queue)
	}
	if len(m.history) != 1 || m.history[0] != a {
		t.Fatalf("expected history [a], got %v", m.history)
	}

	updatedModel, _ = m.Update(keyMsg("p"))
	m = updatedModel.(model)
	if m.current != a {
		t.Fatalf("expected a after skip prev, got %q", m.current)
	}
	if len(m.queue) != 2 || m.queue[0] != b || m.queue[1] != c {
		t.Fatalf("expected [b c] queued, got %v", m.queue)
	}
	if len(m.history) != 0 {
		t.Fatalf("expected empty history, got %v", m.history)
	}

	updatedModel, _ = m.Update(keyMsg("n"))
	m = updatedModel.(model)
	if m.current != b || len(m.queue) != 1 || m.queue[0] != c {
		t.Fatalf("expected round-trip to b with [c] queued, got %q %v", m.current, m.queue)
	}
}

func TestSkipWithEmptyQueueAndHistoryIsNoop(t *testing.T) {
	m := initialModel(nil, "/tmp")
	for _, key := range []string{"n", "p"} {
		updatedModel, cmd := m.Update(keyMsg(key))
		if cmd != nil {
			t.Fatalf("expected no command for %q, got %v", key, cmd)
		}
		if updatedModel.(model).current != "" {
			t.Fatalf("expected no track change for %q", key)
		}
	}
}

func TestHistoryCappedAtTen(t *testing.T) {
	m := initialModel(nil, "/tmp")
	for i := 0; i < 12; i++ {
		m.history = append(m.history, filepath.Join("/music", fmt.Sprintf("old-%02d.flac", i)))
	}
	m.current = "/music/current.flac"
	m.queue = []string{"/music/next.flac"}

	updatedModel, _ := m.Update(keyMsg("n"))
	updated := updatedModel.(model)

	if len(updated.history) != 10 {
		t.Fatalf("expected history capped at 10, got %d", len(updated.history))
	}
	if updated.history[0] != "/music/old-03.flac" {
		t.Fatalf("expected oldest entries dropped, got %v", updated.history)
	}
	if updated.history[9] != "/music/current.flac" {
		t.Fatalf("expected current appended last, got %v", updated.history)
	}
}

func TestSkipNextPlaysRandomWhenQueueEmpty(t *testing.T) {
	dir := t.TempDir()
	tracks := map[string]bool{}
	for _, name := range []string{"x.flac", "y.flac", "z.flac"} {
		p := filepath.Join(dir, name)
		tracks[p] = true
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write track: %v", err)
		}
	}

	m := initialModel(nil, dir)
	m.current = filepath.Join(dir, "x.flac")

	updatedModel, cmd := m.Update(keyMsg("n"))
	if cmd == nil {
		t.Fatal("expected cover render command for random track")
	}
	updated := updatedModel.(model)

	if !tracks[updated.current] {
		t.Fatalf("expected random track from dir, got %q", updated.current)
	}
	if updated.current == filepath.Join(dir, "x.flac") {
		t.Fatal("random pick should exclude the playing track when alternatives exist")
	}
	if len(updated.history) != 1 || updated.history[0] != filepath.Join(dir, "x.flac") {
		t.Fatalf("expected outgoing track recorded in history, got %v", updated.history)
	}
	if len(updated.queue) != 0 {
		t.Fatalf("expected queue still empty, got %v", updated.queue)
	}
}

func TestSkipNextRandomNoopOnSingleTrackDir(t *testing.T) {
	dir := t.TempDir()
	only := filepath.Join(dir, "only.flac")
	if err := os.WriteFile(only, []byte("x"), 0o644); err != nil {
		t.Fatalf("write track: %v", err)
	}

	m := initialModel(nil, dir)
	m.current = only

	updatedModel, cmd := m.Update(keyMsg("n"))
	if cmd != nil {
		t.Fatalf("expected no command, got %v", cmd)
	}
	if updatedModel.(model).current != only {
		t.Fatalf("expected no track change, got %q", updatedModel.(model).current)
	}
}

func TestViewShowsVolumeMeterAndQueueStrip(t *testing.T) {
	m := initialModel(nil, t.TempDir())
	m.width = 100
	m.volume = 65
	m.queue = []string{filepath.Join("/music", "next.flac")}

	view := m.View()
	if !strings.Contains(view, "VOL") {
		t.Fatalf("expected volume meter in view, got %q", view)
	}
	if !strings.Contains(view, "QUEUE [1]") {
		t.Fatalf("expected queue strip in view, got %q", view)
	}
	if !strings.Contains(view, "next") {
		t.Fatalf("expected queued title in view, got %q", view)
	}
}
