package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type savedState struct {
	Dir      string   `json:"dir"`
	Queue    []string `json:"queue"`
	History  []string `json:"history"`
	Volume   float64  `json:"volume"`
	Repeat   bool     `json:"repeat"`
	Current  string   `json:"current"`
	Position float64  `json:"position"`
	Paused   bool     `json:"paused"`
}

func statePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "khzgo", "state.json"), nil
}

func saveState(m model) {
	path, err := statePath()
	if err != nil {
		return
	}

	st := savedState{
		Dir:      m.browser.Dir(),
		Queue:    m.queue,
		History:  m.history,
		Volume:   m.volume,
		Repeat:   m.repeat,
		Current:  m.current,
		Position: m.pos,
		Paused:   m.paused,
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func loadSavedState(fallbackDir string) savedState {
	st := savedState{Dir: fallbackDir}

	path, err := statePath()
	if err != nil {
		return st
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return savedState{Dir: fallbackDir}
	}

	if st.Dir == "" || !dirExists(st.Dir) {
		st.Dir = fallbackDir
	}
	st.Queue = filterExistingFiles(st.Queue)
	st.History = filterExistingFiles(st.History)
	if len(st.History) > maxHistory {
		st.History = st.History[len(st.History)-maxHistory:]
	}
	if st.Current != "" && !fileExists(st.Current) {
		st.Current = ""
		st.Position = 0
		st.Paused = false
	}
	st.Volume = clampVolume(st.Volume)

	return st
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func filterExistingFiles(paths []string) []string {
	out := paths[:0:0]
	for _, p := range paths {
		if fileExists(p) {
			out = append(out, p)
		}
	}
	return out
}
