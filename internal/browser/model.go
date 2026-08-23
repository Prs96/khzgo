package browser

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"khzgo/internal/ui"
)

type playState struct {
	mu     sync.Mutex
	now    string
	queued map[string]bool
}

type listItem struct {
	Entry
	filterValue string
}

func (i listItem) Title() string       { return i.Name }
func (i listItem) Description() string { return "" }
func (i listItem) FilterValue() string {
	if i.filterValue != "" {
		return i.filterValue
	}
	return i.Name
}

type compactDelegate struct {
	state *playState
}

func (d compactDelegate) Height() int                         { return 1 }
func (d compactDelegate) Spacing() int                        { return 0 }
func (d compactDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d compactDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(listItem)
	if !ok {
		return
	}

	label := it.Name
	if !it.IsDir && it.filterValue != "" && it.filterValue != it.Name {
		if d := shortDir(it.filterValue); d != "" {
			label += "  " + ui.DimText.Render("· "+d)
		}
	}
	var style lipgloss.Style
	if it.IsDir {
		style = ui.DirEntry
		label = label + "/"
	} else {
		style = ui.FileEntry

		d.state.mu.Lock()
		isNow := d.state.now != "" && d.state.now == it.Path
		_, isQueued := d.state.queued[it.Path]
		d.state.mu.Unlock()

		if isNow {
			label += " ♪"
			style = ui.PlayingEntry
		} else if isQueued {
			label += " +"
		}
	}

	cursor := "  "
	if index == m.Index() {
		cursor = "> "
		style = ui.SelectedEntry
	}

	fmt.Fprint(w, cursor+style.Render(label))
}

type FileSelectedMsg struct {
	Path string
}

type TrackQueuedMsg struct {
	Path string
}

type TrackDequeuedMsg struct {
	Path string
}

type recursiveLoadedMsg struct {
	dir     string
	entries []listItem
}

type Model struct {
	list       list.Model
	root       string
	dir        string
	err        error
	width      int
	height     int
	state      *playState
	prevFilter list.FilterState
}

func New(rootDir, openDir string) Model {
	state := &playState{queued: map[string]bool{}}
	l := list.New(nil, compactDelegate{state: state}, 0, 0)
	l.Title = shortenPath(openDir)
	l.Styles.Title = ui.Title
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)

	return Model{list: l, root: rootDir, dir: openDir, state: state}
}

func (m Model) SetNowPlaying(path string) Model {
	m.state.mu.Lock()
	m.state.now = path
	m.state.mu.Unlock()
	return m
}

func (m Model) SetQueued(paths []string) Model {
	m.state.mu.Lock()
	m.state.queued = make(map[string]bool, len(paths))
	for _, p := range paths {
		m.state.queued[p] = true
	}
	m.state.mu.Unlock()
	return m
}

func (m Model) IsQueued(path string) bool {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	return m.state.queued[path]
}

func (m Model) Dir() string { return m.dir }

func shortenPath(p string) string {

	parts := strings.Split(filepath.Clean(p), string(filepath.Separator))
	if len(parts) <= 3 {
		return p
	}
	return ".../" + filepath.Join(parts[len(parts)-2:]...)
}

func (m Model) Init() tea.Cmd {
	return m.loadDir(m.dir)
}

type dirLoadedMsg struct {
	dir     string
	entries []Entry
	err     error
}

func (m Model) loadDir(dir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := Scan(dir)
		return dirLoadedMsg{dir: dir, entries: entries, err: err}
	}
}

func (m Model) IsFiltering() bool {
	return m.list.FilterState() == list.Filtering
}

func (m Model) SetSize(w, h int) Model {
	m.width, m.height = w, h
	m.list.SetSize(w, h)
	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {

	case dirLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.dir = msg.dir
		m.list.Title = shortenPath(msg.dir)
		items := make([]list.Item, len(msg.entries))
		for i, e := range msg.entries {
			items[i] = listItem{Entry: e}
		}
		m.list.SetItems(items)
		return m, nil

	case recursiveLoadedMsg:
		if msg.dir != m.root {
			return m, nil
		}
		items := make([]list.Item, len(msg.entries))
		for i, it := range msg.entries {
			items[i] = it
		}
		m.list.SetItems(items)
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
		m.prevFilter = m.list.FilterState()
		return m, cmd

	case tea.KeyMsg:
		if msg.String() == "/" && m.list.FilterState() == list.FilterApplied {
			m.list, _ = m.list.Update(tea.KeyMsg{Type: tea.KeyEscape})
			m.prevFilter = m.list.FilterState()
			return m, m.loadDirRecursive(m.root)
		}
		if msg.String() == "/" && m.list.FilterState() == list.Unfiltered {
			return m, m.loadDirRecursive(m.root)
		}
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "enter", "l":
			if sel, ok := m.list.SelectedItem().(listItem); ok {
				if sel.IsDir {
					return m, m.loadDir(sel.Path)
				}
				return m, func() tea.Msg {
					return FileSelectedMsg{Path: sel.Path}
				}
			}
		case "a":
			if sel, ok := m.list.SelectedItem().(listItem); ok && !sel.IsDir {
				return m, func() tea.Msg {
					return TrackQueuedMsg{Path: sel.Path}
				}
			}
		case "d":
			if sel, ok := m.list.SelectedItem().(listItem); ok && !sel.IsDir && m.IsQueued(sel.Path) {
				return m, func() tea.Msg {
					return TrackDequeuedMsg{Path: sel.Path}
				}
			}
		case "backspace", "h":
			return m, m.loadDir(parentDir(m.dir))
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if m.prevFilter != list.Unfiltered && m.list.FilterState() == list.Unfiltered {
		m.prevFilter = list.Unfiltered
		return m, m.loadDir(m.dir)
	}
	m.prevFilter = m.list.FilterState()
	return m, cmd
}

func (m Model) loadDirRecursive(dir string) tea.Cmd {
	return func() tea.Msg {
		files, err := ScanRecursive(dir)
		if err != nil {
			return dirLoadedMsg{dir: m.dir, err: err}
		}
		items := make([]listItem, len(files))
		for i, f := range files {
			rel, rerr := filepath.Rel(dir, f.Path)
			if rerr != nil {
				rel = f.Name
			}
			items[i] = listItem{Entry: f, filterValue: rel}
		}
		return recursiveLoadedMsg{dir: dir, entries: items}
	}
}

func (m Model) View() string {
	if m.err != nil {
		return ui.ErrorText.Render(fmt.Sprintf("error reading directory: %v", m.err)) +
			"\n" + ui.DimText.Render("[backspace] go up")
	}
	return m.list.View()
}

func parentDir(dir string) string {
	return filepath.Dir(dir)
}

func shortDir(relPath string) string {
	d := filepath.Dir(relPath)
	if d == "." {
		return ""
	}
	parts := strings.Split(d, string(filepath.Separator))
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	return filepath.Join(parts...)
}
