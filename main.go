package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"khzgo/internal/art"
	"khzgo/internal/browser"
	"khzgo/internal/player"
	"khzgo/internal/ui"
)

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	mpv     *player.MPV
	status  string
	err     error
	current string
	cover   string

	coverKey     string
	coverPending bool

	queue  []string
	volume float64

	history []string

	showBindings bool

	browser browser.Model

	progress progress.Model
	pos      float64
	duration float64
	paused   bool

	width  int
	height int
}

type coverArtLoadedMsg struct {
	key  string
	path string
	art  string
	err  error
}

type mpvEventMsg player.Event

func initialModel(mpv *player.MPV, startDir string) model {
	p := progress.New(progress.WithGradient(ui.ProgressStart, ui.ProgressEnd))
	return model{
		mpv:      mpv,
		status:   "idle",
		browser:  browser.New(startDir),
		progress: p,
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd(), m.browser.Init()}
	if c := m.mpvEventCmd(); c != nil {
		cmds = append(cmds, c)
	}
	return tea.Batch(cmds...)
}

func (m model) mpvEventCmd() tea.Cmd {
	if m.mpv == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-m.mpv.Events
		if !ok {
			return nil
		}
		return mpvEventMsg(ev)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

		nowPlayingHeight := nowPlayingPaneHeight()
		innerW := msg.Width - 4
		innerH := msg.Height - nowPlayingHeight - paneChromeHeight() - queueStripHeight() - helpHeight()
		if innerH < 0 {
			innerH = 0
		}
		m.browser = m.browser.SetSize(innerW, innerH)
		if m.current != "" {
			return m.requestCoverArt(m.current)
		}
		return m, nil

	case browser.FileSelectedMsg:

		m.queue = removePath(m.queue, msg.Path)
		m.browser = m.browser.SetQueued(m.queue)
		m = m.pushHistory()
		if m.mpv == nil {
			m.current = msg.Path
			m.status = "loaded"
			m.browser = m.browser.SetNowPlaying(msg.Path)
			return m.requestCoverArt(msg.Path)
		}
		if err := m.mpv.LoadFile(msg.Path); err != nil {
			m.err = err
		} else {
			m.current = msg.Path
			m.status = "loaded"
			m.paused = false
			m.browser = m.browser.SetNowPlaying(msg.Path)
		}
		return m.requestCoverArt(msg.Path)

	case browser.TrackQueuedMsg:
		if !containsPath(m.queue, msg.Path) {
			m.queue = append(m.queue, msg.Path)
			m.browser = m.browser.SetQueued(m.queue)
		}
		return m, nil

	case browser.TrackDequeuedMsg:
		m.queue = removePath(m.queue, msg.Path)
		m.browser = m.browser.SetQueued(m.queue)
		return m, nil

	case mpvEventMsg:
		cmds := []tea.Cmd{}
		if c := m.mpvEventCmd(); c != nil {
			cmds = append(cmds, c)
		}

		if msg.Name == "end-file" && msg.Raw["reason"] == "eof" {
			var advanceCmd tea.Cmd
			m, advanceCmd = m.skipNext()
			if advanceCmd != nil {
				cmds = append(cmds, advanceCmd)
			}
		}
		if len(cmds) == 0 {
			return m, nil
		}
		return m, tea.Batch(cmds...)

	case coverArtLoadedMsg:
		if msg.key != m.coverKey {
			return m, nil
		}
		m.coverPending = false
		if msg.err != nil {

			m.coverKey = ""
			m.cover = ""
			return m, nil
		}
		m.cover = msg.art
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

		if m.showBindings {
			switch msg.String() {
			case "?", "esc", "q":
				m.showBindings = false
			}
			return m, nil
		}

		switch msg.String() {
		case "q":

			if m.browser.IsFiltering() {
				break
			}
			return m, tea.Quit
		case "?":
			if !m.browser.IsFiltering() {
				m.showBindings = true
			}
			return m, nil
		case "s":
			if !m.browser.IsFiltering() {
				return m.playFolder(true)
			}
		case "A":
			if !m.browser.IsFiltering() {
				return m.playFolder(false)
			}
		case "n":
			if !m.browser.IsFiltering() {
				return m.skipNext()
			}
		case "p":
			if !m.browser.IsFiltering() {
				return m.skipPrev()
			}
		case " ":
			if m.mpv == nil {
				return m, nil
			}
			if err := m.mpv.TogglePause(); err != nil {
				m.err = err
			} else {
				m.paused = !m.paused
			}
			return m, nil
		case "-":
			if m.mpv == nil {
				return m, nil
			}
			vol := clampVolume(m.volume - 5)
			if err := m.mpv.SetVolume(int(vol)); err != nil {
				m.err = err
			} else {
				m.volume = vol
			}
			return m, nil
		case "=":
			if m.mpv == nil {
				return m, nil
			}
			vol := clampVolume(m.volume + 5)
			if err := m.mpv.SetVolume(int(vol)); err != nil {
				m.err = err
			} else {
				m.volume = vol
			}
			return m, nil
		}

		var cmd tea.Cmd
		m.browser, cmd = m.browser.Update(msg)
		return m, cmd

	case tickMsg:
		if m.mpv == nil {
			return m, tickCmd()
		}

		if posRaw, err := m.mpv.GetProperty("time-pos"); err == nil {
			if pos, ok := posRaw.(float64); ok {
				m.pos = pos
			}
		}
		if durRaw, err := m.mpv.GetProperty("duration"); err == nil {
			if dur, ok := durRaw.(float64); ok {
				m.duration = dur
			}
		}
		if pauseRaw, err := m.mpv.GetProperty("pause"); err == nil {
			if paused, ok := pauseRaw.(bool); ok {
				m.paused = paused
			}
		}
		if volRaw, err := m.mpv.GetProperty("volume"); err == nil {
			if vol, ok := volRaw.(float64); ok {
				m.volume = vol
			}
		}
		return m, tickCmd()

	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		if pm, ok := newModel.(progress.Model); ok {
			m.progress = pm
		}
		return m, cmd
	}

	var cmd tea.Cmd
	m.browser, cmd = m.browser.Update(msg)
	return m, cmd
}

func (m model) View() string {
	w := m.width
	if w < 20 {
		w = 80
	}
	h := m.height
	if h < 5 {
		h = 24
	}

	if m.showBindings {
		return bindingsView(w, h)
	}
	browserPane := ui.PaneBorderActive.
		Width(w - 2).
		Render(m.browser.View())

	track := currentTrackTitle(m.current)

	state := "playing"
	if m.paused {
		state = "paused"
	}
	if m.duration == 0 {
		state = "idle"
	}

	var frac float64
	if m.duration > 0 {
		frac = m.pos / m.duration
	}

	coverWidth := coverArtWidth(w)
	metaWidth := w - coverWidth - 7
	if metaWidth < 18 {
		metaWidth = w - 6
	}
	m.progress.Width = max(18, min(metaWidth-6, 36))
	trackInfoInner := lipgloss.JoinVertical(lipgloss.Center,
		ui.SectionLabel.Render("NOW PLAYING"),
		ui.TrackTitle.Render(track)+"  "+ui.TrackState.Render("["+strings.ToUpper(state)+"]"),
		ui.DimText.Render(trackLocation(m.current)),
		m.progress.ViewAs(frac),
		ui.TimeLabel.Render(fmtDuration(m.pos)+" / "+fmtDuration(m.duration)),
		renderVolumeMeter(m.volume),
	)
	trackInfo := lipgloss.Place(metaWidth, coverArtPanelHeight(), lipgloss.Center, lipgloss.Center, trackInfoInner)

	nowPlaying := lipgloss.JoinHorizontal(lipgloss.Top,
		renderCoverArt(m.cover, m.current, coverWidth),
		trackInfo,
	)

	nowPlayingPane := ui.PaneBorderAlt.
		Width(w - 2).
		Render(nowPlaying)

	queueLine := renderQueueStrip(m.queue, w-4)
	body := lipgloss.JoinVertical(lipgloss.Left, browserPane, queueLine, nowPlayingPane)

	if m.err != nil {
		body += "\n" + ui.ErrorText.Render("error: "+m.err.Error())
	}

	help := ui.StatusBar.Render("enter play   a queue   d dequeue   n/p next/prev   s shuffle   A play all   -/= volume   ? keys   q quit")
	return body + "\n" + help
}

func renderQueueStrip(queue []string, width int) string {
	if len(queue) == 0 {
		return ui.DimText.Render(strings.TrimSpace(fmt.Sprintf(
			"%-*s", width, "queue empty — press a on a track to enqueue",
		)))
	}

	label := ui.SectionLabel.Render(fmt.Sprintf("QUEUE [%d]", len(queue)))

	remaining := width - lipgloss.Width(label) - 1
	var titles []string
	for _, p := range queue {
		t := currentTrackTitle(p)
		if lipgloss.Width(strings.Join(titles, " → "))+lipgloss.Width(t)+3*len(titles) > remaining {
			break
		}
		titles = append(titles, t)
	}
	if len(titles) == 0 && len(queue) > 0 {
		titles = []string{"..."}
	}

	line := label + " " + ui.DimText.Render(strings.Join(titles, " → "))
	return line
}

func renderVolumeMeter(volume float64) string {
	const blocks = 13
	filled := int(clampVolume(volume)/10 + 0.5)
	if filled < 0 {
		filled = 0
	}
	if filled > blocks {
		filled = blocks
	}
	bar := ui.VolumeFill.Render(strings.Repeat("█", filled)) +
		ui.VolumeEmpty.Render(strings.Repeat("░", blocks-filled))
	pct := fmt.Sprintf("%3.0f%%", clampVolume(volume))
	return ui.TimeLabel.Render("VOL ") + bar + " " + ui.TimeLabel.Render(pct)
}

const maxHistory = 10

func (m model) pushHistory() model {
	if m.current != "" {
		m.history = append(m.history, m.current)
		if len(m.history) > maxHistory {
			m.history = m.history[len(m.history)-maxHistory:]
		}
	}
	return m
}

func (m model) skipNext() (model, tea.Cmd) {
	if len(m.queue) == 0 {
		next, ok := m.randomTrackFromDir()
		if !ok {
			return m, nil
		}
		m = m.pushHistory()
		return m.startTrack(next)
	}
	next := m.queue[0]
	m.queue = m.queue[1:]
	m = m.pushHistory()
	return m.startTrack(next)
}

func (m model) randomTrackFromDir() (string, bool) {
	entries, err := browser.Scan(m.browser.Dir())
	if err != nil {
		return "", false
	}
	var candidates []string
	for _, e := range entries {
		if !e.IsDir && e.Path != m.current {
			candidates = append(candidates, e.Path)
		}
	}
	if len(candidates) == 0 {
		return "", false
	}
	return candidates[rand.Intn(len(candidates))], true
}

func (m model) skipPrev() (model, tea.Cmd) {
	if len(m.history) == 0 {
		return m, nil
	}
	prev := m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]
	if m.current != "" {
		m.queue = append([]string{m.current}, m.queue...)
	}
	return m.startTrack(prev)
}

func (m model) startTrack(path string) (model, tea.Cmd) {
	m.cover = ""
	m.coverKey = ""
	m.current = path
	m.status = "loaded"
	m.paused = false
	m.browser = m.browser.SetNowPlaying(path).SetQueued(m.queue)

	if m.mpv != nil {
		if err := m.mpv.LoadFile(path); err != nil {
			m.err = err
		}
	}
	return m.requestCoverArt(path)
}

func (m model) playFolder(shuffle bool) (tea.Model, tea.Cmd) {
	entries, err := browser.Scan(m.browser.Dir())
	if err != nil {
		m.err = err
		return m, nil
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir {
			files = append(files, e.Path)
		}
	}
	if len(files) == 0 {
		return m, nil
	}

	if shuffle {
		rand.Shuffle(len(files), func(i, j int) {
			files[i], files[j] = files[j], files[i]
		})
	}

	first := files[0]
	m.queue = files[1:]
	m.browser = m.browser.SetQueued(m.queue)
	m = m.pushHistory()
	return m.startTrack(first)
}

func bindingsView(width, height int) string {
	rows := [][2]string{
		{"enter / l", "play selected"},
		{"a", "add to queue"},
		{"d", "remove from queue"},
		{"s", "shuffle-play this folder"},
		{"A", "play all (folder order)"},
		{"n", "next (random if queue empty)"},
		{"p", "previous track"},
		{"space", "pause / resume"},
		{"- / =", "volume down / up"},
		{"backspace / h", "up one directory"},
		{"j / k / arrows", "navigate"},
		{"/", "filter"},
		{"?", "toggle help"},
		{"q", "quit"},
	}

	var body strings.Builder
	body.WriteString(ui.SectionLabel.Render("KEYBINDINGS") + "\n\n")
	for _, r := range rows {
		key := ui.TrackState.Render(fmt.Sprintf("%-16s", r[0]))
		body.WriteString(key + r[1] + "\n")
	}
	body.WriteString("\n" + ui.DimText.Render("? or esc to close"))

	box := ui.PaneBorderActive.Width(46).Render(strings.TrimRight(body.String(), "\n"))
	return lipgloss.Place(width, max(height, 1), lipgloss.Center, lipgloss.Center, box)
}

func fmtDuration(secs float64) string {
	if secs < 0 {
		secs = 0
	}
	d := time.Duration(secs) * time.Second
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func currentTrackTitle(path string) string {
	if path == "" {
		return "no track loaded"
	}
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}

func trackLocation(path string) string {
	if path == "" {
		return "browse and press enter to start playback"
	}
	parent := filepath.Base(filepath.Dir(path))
	grand := filepath.Base(filepath.Dir(filepath.Dir(path)))
	if grand != "." && grand != string(filepath.Separator) && grand != parent {
		return grand + " / " + parent
	}
	if parent != "." && parent != string(filepath.Separator) {
		return parent
	}
	return filepath.Dir(path)
}

func renderCoverArt(artString, path string, width int) string {
	panelWidth := max(8, width-4)
	content := lipgloss.JoinVertical(lipgloss.Center,
		ui.SectionLabel.Render("COVER ART"),
		renderCoverBody(artString, path, panelWidth),
	)

	return ui.CoverArtFrame.
		Width(width).
		Render(lipgloss.Place(width-2, coverArtPanelHeight(), lipgloss.Center, lipgloss.Center, content))
}

func renderCoverBody(artString, path string, width int) string {
	if artString != "" {
		return artString
	}

	badge := ui.CoverArtBadge.
		Width(max(8, width)).
		Align(lipgloss.Center).
		Padding(1, 0)

	meta := ui.CoverArtMeta.
		Width(max(8, width)).
		Align(lipgloss.Center)

	return lipgloss.JoinVertical(lipgloss.Center,
		badge.Render(coverInitials(path)),
		meta.Render("no artwork"),
	)
}

func coverInitials(path string) string {
	if path == "" {
		return "--"
	}
	name := currentTrackTitle(path)
	parts := strings.FieldsFunc(name, func(r rune) bool {
		switch r {
		case ' ', '-', '_', '.', '(', ')', '[', ']':
			return true
		default:
			return false
		}
	})
	if len(parts) == 0 {
		return strings.ToUpper(name[:min(2, len(name))])
	}
	if len(parts) == 1 {
		return strings.ToUpper(parts[0][:min(2, len(parts[0]))])
	}
	return strings.ToUpper(parts[0][:1] + parts[1][:1])
}

func coverArtWidth(totalWidth int) int {
	if totalWidth < 72 {
		return 16
	}
	if totalWidth > 110 {
		return 24
	}
	return 20
}

func coverArtRenderWidth(totalWidth int) int {
	return max(10, coverArtWidth(max(totalWidth, 80))-4)
}

func coverArtRenderHeight() int {
	return 8
}

func coverArtPanelHeight() int {
	return 10
}

func paneChromeHeight() int {
	return 2
}

func nowPlayingPaneHeight() int {
	return coverArtPanelHeight() + paneChromeHeight()*2
}

func helpHeight() int {
	return 1
}

func queueStripHeight() int {
	return 1
}

func containsPath(paths []string, path string) bool {
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}

func removePath(paths []string, path string) []string {
	out := paths[:0:0]
	for _, p := range paths {
		if p != path {
			out = append(out, p)
		}
	}
	return out
}

func clampVolume(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 130 {
		return 130
	}
	return v
}

func loadCoverArtCmd(key, path string, width, height int) tea.Cmd {
	if path == "" || width < 4 || height < 4 {
		return nil
	}

	return func() tea.Msg {
		rendered, err := art.Render(path, width, height)
		return coverArtLoadedMsg{key: key, path: path, art: rendered, err: err}
	}
}

func (m model) requestCoverArt(path string) (model, tea.Cmd) {
	if path == "" {
		return m, nil
	}

	key := fmt.Sprintf("%s|%s|%d|%d",
		path,
		art.ActiveProtocol(),
		coverArtRenderWidth(m.width),
		coverArtRenderHeight(),
	)

	if key == m.coverKey && (m.cover != "" || m.coverPending) {
		return m, nil
	}

	m.coverKey = key
	m.coverPending = true
	return m, loadCoverArtCmd(key, path, coverArtRenderWidth(m.width), coverArtRenderHeight())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	startDir := "."
	if len(os.Args) > 1 {
		startDir = os.Args[1]
	}
	if abs, err := filepath.Abs(startDir); err == nil {
		startDir = abs
	}

	mpv, err := player.Start("/tmp/mmp-mpv-socket")
	if err != nil {
		fmt.Println("error starting mpv:", err)
		os.Exit(1)
	}
	defer mpv.Close()

	p := tea.NewProgram(initialModel(mpv, startDir), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("error running program:", err)
		os.Exit(1)
	}
}
