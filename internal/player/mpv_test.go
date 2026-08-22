package player

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
)

func TestLoadFileUsesReplaceMode(t *testing.T) {
	req := roundTripLoad(t, func(m *MPV) error { return m.LoadFile("/tmp/song.flac") })

	if len(req.Command) != 3 {
		t.Fatalf("expected 3 command args, got %v", req.Command)
	}
	if got := req.Command[2]; got != "replace" {
		t.Fatalf("expected replace mode, got %v", got)
	}
}

func TestAppendFileUsesAppendPlayMode(t *testing.T) {
	req := roundTripLoad(t, func(m *MPV) error { return m.AppendFile("/tmp/next.flac") })

	if len(req.Command) != 3 {
		t.Fatalf("expected 3 command args, got %v", req.Command)
	}
	if got := req.Command[0]; got != "loadfile" {
		t.Fatalf("expected loadfile command, got %v", got)
	}
	if got := req.Command[2]; got != "append-play" {
		t.Fatalf("expected append-play mode, got %v", got)
	}
}

func roundTripLoad(t *testing.T, call func(*MPV) error) command {
	t.Helper()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	m := &MPV{
		conn:    client,
		pending: make(map[int]chan response),
		Events:  make(chan Event, 1),
	}
	go m.readLoop()

	requestCh := make(chan command, 1)
	go func() {
		line, err := bufio.NewReader(server).ReadBytes('\n')
		if err != nil {
			requestCh <- command{}
			return
		}

		var req command
		if err := json.Unmarshal(line, &req); err != nil {
			requestCh <- command{}
			return
		}

		requestCh <- req
		_, _ = server.Write([]byte(`{"request_id":1,"error":"success"}` + "\n"))
	}()

	if err := call(m); err != nil {
		t.Fatalf("player call: %v", err)
	}

	return <-requestCh
}
