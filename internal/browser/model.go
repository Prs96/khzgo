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
}

func (i listItem) Title() string       { return i.Name }
func (i listItem) Description() string { return "" }
func (i listItem) FilterValue() string { return i.Name }

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

type Model struct {
	list   list.Model
	dir    string
	err    error
	width  int
	height int
	state  *playState
}

func New(startDir string) Model {
	state := &playState{queued: map[string]bool{}}
	l := list.New(nil, compactDelegate{state: state}, 0, 0)
	l.Title = shortenPath(startDir)
	l.Styles.Title = ui.Title
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)

	return Model{list: l, dir: startDir, state: state}
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
			items[i] = listItem{e}
		}
		m.list.SetItems(items)
		return m, nil

	case tea.KeyMsg:

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
	return m, cmd
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
