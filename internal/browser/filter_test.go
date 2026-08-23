package browser

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func execCmd(cmd tea.Cmd) tea.Msg {
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		return msg
	case <-time.After(500 * time.Millisecond):
		return nil
	}
}

func pump(m Model, cmd tea.Cmd, depth int) Model {
	if cmd == nil || depth > 8 {
		return m
	}
	switch msg := execCmd(cmd).(type) {
	case tea.BatchMsg:
		for _, c := range []tea.Cmd(msg) {
			m = pump(m, c, depth+1)
		}
	case list.FilterMatchesMsg:
		var next tea.Cmd
		m, next = m.Update(msg)
		pump(m, next, depth+1)
	}
	return m
}

func sendKey(m Model, key string) (Model, tea.Cmd) {
	var km tea.KeyMsg
	switch key {
	case "enter":
		km = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		km = tea.KeyMsg{Type: tea.KeyEscape}
	default:
		km = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	m, cmd := m.Update(km)
	return m, cmd
}

func TestRecursiveFilterFlow(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("Artist One/Album Alpha/sunrise.mp3")
	mk("Artist One/Album Alpha/moonwalk.mp3")
	mk("Artist Two/Album Beta/neon-night.flac")
	mk("top-level.mp3")

	m := New(root, root).SetSize(80, 24)
	m, _ = m.Update(execCmd(m.Init()))
	if n := len(m.list.Items()); n != 3 {
		t.Fatalf("initial dir items = %d, want 3", n)
	}

	{
		var cmd tea.Cmd
		m, cmd = sendKey(m, "enter")
		dl, ok := execCmd(cmd).(dirLoadedMsg)
		if !ok {
			t.Fatal("enter on dir should load it")
		}
		m, _ = m.Update(dl)
	}
	if m.Dir() != filepath.Join(root, "Artist One") {
		t.Fatalf("browsed into %q", m.Dir())
	}

	startSearch := func() {
		t.Helper()
		var cmd tea.Cmd
		m, cmd = sendKey(m, "/")
		msg := execCmd(cmd)
		rm, ok := msg.(recursiveLoadedMsg)
		if !ok {
			t.Fatalf("expected recursiveLoadedMsg, got %T", msg)
		}
		m, _ = m.Update(rm)
		if got := m.list.FilterState(); got != list.Filtering {
			t.Fatalf("filter state = %v, want Filtering", got)
		}
		if n := len(m.list.Items()); n != 4 {
			t.Fatalf("recursive items = %d, want 4", n)
		}
	}

	startSearch()

	for _, k := range []string{"a", "l", "p"} {
		var cmd tea.Cmd
		m, cmd = sendKey(m, k)
		m = pump(m, cmd, 0)
	}
	vis := m.list.VisibleItems()
	if len(vis) != 2 {
		t.Fatalf("visible after 'alp' = %d (%v), want 2", len(vis), visibleNames(vis))
	}

	m, _ = sendKey(m, "enter")
	if got := m.list.FilterState(); got != list.FilterApplied {
		t.Fatalf("after accept, state = %v, want FilterApplied", got)
	}
	if n := len(m.list.VisibleItems()); n != 2 {
		t.Fatalf("after accept, visible = %d, want 2", n)
	}

	startSearch()
	for _, k := range []string{"b", "e", "t", "a"} {
		var cmd tea.Cmd
		m, cmd = sendKey(m, k)
		m = pump(m, cmd, 0)
	}
	vis = m.list.VisibleItems()
	if len(vis) != 1 || visibleNames(vis)[0] != "neon-night.flac" {
		t.Fatalf("visible after 'beta' = %v, want [neon-night.flac]", visibleNames(vis))
	}

	m2 := New(root, filepath.Join(root, "Artist Two")).SetSize(80, 24)
	m2, _ = m2.Update(execCmd(m2.Init()))
	m2, cmd := sendKey(m2, "/")
	m2, _ = m2.Update(execCmd(cmd).(recursiveLoadedMsg))
	for _, k := range []string{"m", "o", "o", "n"} {
		var c tea.Cmd
		m2, c = sendKey(m2, k)
		m2 = pump(m2, c, 0)
	}
	names := visibleNames(m2.list.VisibleItems())
	if len(names) != 1 || names[0] != "moonwalk.mp3" {
		t.Fatalf("'moon' matched %v, want [moonwalk.mp3]", names)
	}

	m, restore := sendKey(m, "esc")
	dl, ok := execCmd(restore).(dirLoadedMsg)
	if !ok {
		t.Fatal("esc should trigger dir restore")
	}
	m, _ = m.Update(dl)
	if got := m.list.FilterState(); got != list.Unfiltered {
		t.Fatalf("after restore, state = %v", got)
	}
	if n := len(m.list.Items()); n != 1 {
		t.Fatalf("restored items = %d, want 1 (Artist One listing)", n)
	}
}

func visibleNames(items []list.Item) []string {
	var out []string
	for _, it := range items {
		if li, ok := it.(listItem); ok {
			out = append(out, li.Name)
		}
	}
	return out
}
