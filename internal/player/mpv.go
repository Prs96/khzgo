package player

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type MPV struct {
	conn     net.Conn
	sockPath string
	cmd      *exec.Cmd

	mu        sync.Mutex
	nextID    int
	pending   map[int]chan response
	closeOnce sync.Once

	Events chan Event
}

type Event struct {
	Name string
	Raw  map[string]interface{}
}

type command struct {
	Command   []interface{} `json:"command"`
	RequestID int           `json:"request_id"`
}

type response struct {
	Data      interface{} `json:"data"`
	Error     string      `json:"error"`
	RequestID int         `json:"request_id"`
	Event     string      `json:"event"`
}

func Start(sockPath string) (*MPV, error) {
	cleanupExisting(sockPath)

	cmd := exec.Command("mpv",
		"--no-video",
		"--idle=yes",
		"--input-ipc-server="+sockPath,
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		conn, err = net.Dial("unix", sockPath)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("mpv socket never appeared: %w", err)
	}

	m := &MPV{
		conn:     conn,
		sockPath: sockPath,
		cmd:      cmd,
		pending:  make(map[int]chan response),
		Events:   make(chan Event, 32),
	}

	go m.readLoop()

	return m, nil
}

func (m *MPV) readLoop() {
	reader := bufio.NewReader(m.conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			close(m.Events)
			return
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		if evName, ok := raw["event"].(string); ok {
			select {
			case m.Events <- Event{Name: evName, Raw: raw}:
			default:

			}
			continue
		}

		var resp response
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}

		m.mu.Lock()
		ch, ok := m.pending[resp.RequestID]
		if ok {
			delete(m.pending, resp.RequestID)
		}
		m.mu.Unlock()

		if ok {
			ch <- resp
		}

	}
}

func (m *MPV) send(cmd command) (*response, error) {
	m.mu.Lock()
	m.nextID++
	id := m.nextID
	cmd.RequestID = id
	replyCh := make(chan response, 1)
	m.pending[id] = replyCh
	m.mu.Unlock()

	data, err := json.Marshal(cmd)
	if err != nil {
		m.mu.Lock()
		delete(m.pending, id)
		m.mu.Unlock()
		return nil, err
	}

	if _, err := m.conn.Write(append(data, '\n')); err != nil {
		m.mu.Lock()
		delete(m.pending, id)
		m.mu.Unlock()
		return nil, err
	}

	select {
	case resp := <-replyCh:
		return &resp, nil
	case <-time.After(5 * time.Second):
		m.mu.Lock()
		delete(m.pending, id)
		m.mu.Unlock()
		return nil, fmt.Errorf("mpv command timed out")
	}
}

func (m *MPV) LoadFile(path string) error {
	_, err := m.send(command{Command: []interface{}{"loadfile", path, "replace"}})
	return err
}

func (m *MPV) AppendFile(path string) error {
	_, err := m.send(command{Command: []interface{}{"loadfile", path, "append-play"}})
	return err
}

func (m *MPV) TogglePause() error {
	_, err := m.send(command{Command: []interface{}{"cycle", "pause"}})
	return err
}

func (m *MPV) SetPause(paused bool) error {
	_, err := m.send(command{Command: []interface{}{"set_property", "pause", paused}})
	return err
}

func (m *MPV) SetVolume(v int) error {
	_, err := m.send(command{Command: []interface{}{"set_property", "volume", v}})
	return err
}

func (m *MPV) Seek(seconds float64) error {
	_, err := m.send(command{Command: []interface{}{"seek", seconds, "absolute"}})
	return err
}

func (m *MPV) SeekRelative(seconds float64) error {
	_, err := m.send(command{Command: []interface{}{"seek", seconds, "relative"}})
	return err
}

func (m *MPV) Stop() error {
	_, err := m.send(command{Command: []interface{}{"stop"}})
	return err
}

func (m *MPV) SetStart(seconds float64) error {
	val := strconv.FormatFloat(seconds, 'f', 3, 64)
	_, err := m.send(command{Command: []interface{}{"set_property", "start", val}})
	return err
}

func (m *MPV) ClearStart() error {
	_, err := m.send(command{Command: []interface{}{"set_property", "start", "none"}})
	return err
}

func (m *MPV) SetLoopFile(loop bool) error {
	val := interface{}("no")
	if loop {
		val = "inf"
	}
	_, err := m.send(command{Command: []interface{}{"set_property", "loop-file", val}})
	return err
}

func (m *MPV) GetProperty(prop string) (interface{}, error) {
	resp, err := m.send(command{Command: []interface{}{"get_property", prop}})
	if err != nil {
		return nil, err
	}
	if resp.Error != "success" {
		return nil, fmt.Errorf("mpv error: %s", resp.Error)
	}
	return resp.Data, nil
}

func (m *MPV) Close() error {
	var closeErr error

	m.closeOnce.Do(func() {
		if m.conn != nil {
			_, _ = m.conn.Write([]byte("{\"command\":[\"quit\"]}\n"))
			if err := m.conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				closeErr = err
			}
		}

		if m.cmd != nil && m.cmd.Process != nil {
			done := make(chan error, 1)
			go func() {
				done <- m.cmd.Wait()
			}()

			select {
			case err := <-done:
				if err != nil && closeErr == nil && !errors.Is(err, os.ErrProcessDone) {
					closeErr = err
				}
			case <-time.After(2 * time.Second):
				_ = m.cmd.Process.Kill()
				if err := <-done; err != nil && closeErr == nil && !errors.Is(err, os.ErrProcessDone) {
					closeErr = err
				}
			}
		}

		if m.sockPath != "" {
			_ = os.Remove(m.sockPath)
		}
	})

	return closeErr
}

func cleanupExisting(sockPath string) {
	if _, err := os.Stat(sockPath); err != nil {
		return
	}

	conn, err := net.DialTimeout("unix", sockPath, 500*time.Millisecond)
	if err == nil {
		_, _ = conn.Write([]byte("{\"command\":[\"quit\"]}\n"))
		_ = conn.Close()
		for i := 0; i < 10; i++ {
			if _, statErr := os.Stat(sockPath); errors.Is(statErr, os.ErrNotExist) {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	_ = os.Remove(sockPath)
}
