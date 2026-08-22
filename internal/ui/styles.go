package ui

import "github.com/charmbracelet/lipgloss"

const (
	ProgressStart = "#8B7BFF"
	ProgressEnd   = "#46C7B8"
)

var (
	Accent    = lipgloss.AdaptiveColor{Light: "#6759c9", Dark: "#8b7bff"}
	AccentAlt = lipgloss.AdaptiveColor{Light: "#157a72", Dark: "#46c7b8"}
	Glow      = lipgloss.AdaptiveColor{Light: "#c97720", Dark: "#f1a64d"}
	Subtle    = lipgloss.AdaptiveColor{Light: "#8a8a8a", Dark: "#6c6c6c"}
	Fg        = lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#e4e4e4"}
	Dim       = lipgloss.AdaptiveColor{Light: "#a0a0a0", Dark: "#5c5c5c"}
	Error     = lipgloss.AdaptiveColor{Light: "#c4302b", Dark: "#e06c75"}
)

var (
	PaneBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Subtle).
			Padding(0, 1)

	PaneBorderActive = PaneBorder.BorderForeground(Accent)

	PaneBorderAlt = PaneBorder.BorderForeground(AccentAlt)

	Title = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	DirEntry = lipgloss.NewStyle().
			Foreground(AccentAlt).
			Bold(true)

	FileEntry = lipgloss.NewStyle().
			Foreground(Fg)

	SelectedEntry = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	PlayingEntry = lipgloss.NewStyle().
			Foreground(Glow).
			Bold(true)

	QueuedMark = lipgloss.NewStyle().Foreground(Dim)

	VolumeFill = lipgloss.NewStyle().Foreground(AccentAlt)

	VolumeEmpty = lipgloss.NewStyle().Foreground(Subtle)

	SectionLabel = lipgloss.NewStyle().
			Foreground(AccentAlt).
			Bold(true)

	TrackState = lipgloss.NewStyle().
			Foreground(Glow).
			Bold(true)

	DimText = lipgloss.NewStyle().Foreground(Dim)

	ErrorText = lipgloss.NewStyle().Foreground(Error)

	StatusBar = lipgloss.NewStyle().Foreground(Subtle)

	TrackTitle = lipgloss.NewStyle().Foreground(Fg).Bold(true)

	TimeLabel = lipgloss.NewStyle().Foreground(Subtle)

	CoverArtFrame = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Glow).
			Padding(0, 1)

	CoverArtBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f8f8f8")).
			Background(Accent).
			Bold(true)

	CoverArtMeta = lipgloss.NewStyle().Foreground(Subtle)
)
