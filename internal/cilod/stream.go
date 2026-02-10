// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cilod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketMessage is the envelope for WebSocket communication
type WebSocketMessage struct {
	Type     string `json:"type"`                // "stdout", "stderr", "stdin", "error", "exit", "signal", "eof"
	Data     []byte `json:"data"`                // Message payload
	ExitCode int    `json:"exit_code,omitempty"` // For exec exit
}

// Streamer provides WebSocket-based streaming for exec and logs
type Streamer struct {
	baseURL string
	token   string
	dialer  *websocket.Dialer
}

// NewStreamer creates a new WebSocket streamer
func NewStreamer(baseURL, token string) *Streamer {
	return &Streamer{
		baseURL: baseURL,
		token:   token,
		dialer: &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		},
	}
}

// SetTimeout sets the WebSocket handshake timeout
func (s *Streamer) SetTimeout(timeout time.Duration) {
	s.dialer.HandshakeTimeout = timeout
}

// StreamExec executes a command in a container via WebSocket
// Supports bidirectional streaming (stdin/stdout/stderr) and PTY allocation
func (s *Streamer) StreamExec(ctx context.Context, env, service string, cmd []string, stdin io.Reader, stdout, stderr io.Writer, tty bool) error {
	// Build WebSocket URL
	wsURL, err := s.buildWebSocketURL(fmt.Sprintf("/environments/%s/exec", env))
	if err != nil {
		return fmt.Errorf("build WebSocket URL: %w", err)
	}

	// Connect to WebSocket
	conn, resp, err := s.dialer.DialContext(ctx, wsURL, s.authHeaders())
	if err != nil {
		if resp != nil && resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return parseErrorResponse(resp.StatusCode, body)
		}
		return fmt.Errorf("dial WebSocket: %w", err)
	}
	defer conn.Close()

	// Send exec request
	execReq := EnvironmentExecRequest{
		Service: service,
		Command: cmd,
		TTY:     tty,
		Stdin:   stdin != nil,
	}

	reqData, err := json.Marshal(execReq)
	if err != nil {
		return fmt.Errorf("marshal exec request: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, reqData); err != nil {
		return fmt.Errorf("send exec request: %w", err)
	}

	// Setup signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Create context for goroutine coordination
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	exitCode := 0
	var exitMu sync.Mutex
	var streamErr error
	var errMu sync.Mutex

	// Goroutine to handle stdin -> WebSocket
	if stdin != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.streamStdin(ctx, conn, stdin)
		}()
	}

	// Goroutine to handle signals -> WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case sig := <-sigChan:
				sigName := sig.String()
				if sig == syscall.SIGINT {
					sigName = "SIGINT"
				} else if sig == syscall.SIGTERM {
					sigName = "SIGTERM"
				}
				msg := WebSocketMessage{
					Type: "signal",
					Data: []byte(sigName),
				}
				data, _ := json.Marshal(msg)
				conn.WriteMessage(websocket.TextMessage, data)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Main loop: handle WebSocket messages
	connFailed := false
	for {
		select {
		case <-ctx.Done():
			// Send close message
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			wg.Wait()
			return ctx.Err()
		default:
		}

		if connFailed {
			break
		}

		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		// Recover from potential panics due to connection state
		var msg []byte
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("websocket panic: %v", r)
				}
			}()
			_, msg, err = conn.ReadMessage()
		}()

		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				break
			}
			if ctx.Err() != nil {
				wg.Wait()
				return ctx.Err()
			}
			if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
				continue
			}
			// Check if this is a panic from closed connection - not a real error
			if strings.Contains(err.Error(), "repeated read") {
				connFailed = true
				break
			}
			// Connection error - mark as failed and exit
			errMu.Lock()
			streamErr = fmt.Errorf("read WebSocket: %w", err)
			errMu.Unlock()
			connFailed = true
			break
		}

		var wsMsg WebSocketMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			continue
		}

		switch wsMsg.Type {
		case "stdout":
			if stdout != nil {
				stdout.Write(wsMsg.Data)
			}
		case "stderr":
			if stderr != nil {
				stderr.Write(wsMsg.Data)
			}
		case "exit":
			exitMu.Lock()
			exitCode = wsMsg.ExitCode
			exitMu.Unlock()
			cancel()
			wg.Wait()
			if exitCode != 0 {
				return fmt.Errorf("exit code %d", exitCode)
			}
			return nil
		case "error":
			errMu.Lock()
			streamErr = fmt.Errorf("remote error: %s", string(wsMsg.Data))
			errMu.Unlock()
			cancel()
			wg.Wait()
			return streamErr
		}
	}

	wg.Wait()
	errMu.Lock()
	defer errMu.Unlock()
	return streamErr
}

// StreamLogs streams logs from a service via WebSocket
// Supports following logs with --follow flag
func (s *Streamer) StreamLogs(ctx context.Context, env, service string, follow bool, stdout io.Writer) error {
	// Build WebSocket URL with query params
	path := fmt.Sprintf("/environments/%s/logs?service=%s", env, url.QueryEscape(service))
	if follow {
		path += "&follow=true"
	}

	wsURL, err := s.buildWebSocketURL(path)
	if err != nil {
		return fmt.Errorf("build WebSocket URL: %w", err)
	}

	// Connect to WebSocket
	conn, resp, err := s.dialer.DialContext(ctx, wsURL, s.authHeaders())
	if err != nil {
		if resp != nil && resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return parseErrorResponse(resp.StatusCode, body)
		}
		return fmt.Errorf("dial WebSocket: %w", err)
	}
	defer conn.Close()

	// Main loop: handle log messages
	for {
		select {
		case <-ctx.Done():
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return ctx.Err()
		default:
		}

		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
				continue
			}
			return fmt.Errorf("read WebSocket: %w", err)
		}

		var wsMsg WebSocketMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			continue
		}

		switch wsMsg.Type {
		case "stdout", "stderr":
			if stdout != nil {
				stdout.Write(wsMsg.Data)
			}
		case "eof":
			return nil
		case "error":
			return fmt.Errorf("remote error: %s", string(wsMsg.Data))
		}
	}
}

// streamStdin reads from stdin and sends to WebSocket
func (s *Streamer) streamStdin(ctx context.Context, conn *websocket.Conn, stdin io.Reader) {
	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := stdin.Read(buf)
		if err != nil {
			if err != io.EOF {
				// Log error but don't crash
				fmt.Fprintf(os.Stderr, "stdin read error: %v\n", err)
			}
			return
		}

		if n > 0 {
			msg := WebSocketMessage{
				Type: "stdin",
				Data: buf[:n],
			}
			data, _ := json.Marshal(msg)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}
}

// buildWebSocketURL converts an HTTP URL to WebSocket URL
func (s *Streamer) buildWebSocketURL(path string) (string, error) {
	base := s.baseURL
	if strings.HasPrefix(base, "http://") {
		base = "ws://" + base[7:]
	} else if strings.HasPrefix(base, "https://") {
		base = "wss://" + base[8:]
	} else {
		base = "ws://" + base
	}

	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return base + path, nil
}

// authHeaders returns headers with authentication
func (s *Streamer) authHeaders() http.Header {
	headers := http.Header{}
	if s.token != "" {
		headers.Set("Authorization", "Bearer "+s.token)
	}
	return headers
}

// ExecOptions provides options for remote exec
type ExecOptions struct {
	Service string
	Command []string
	TTY     bool
	Stdin   bool
}

// LogOptions provides options for log streaming
type LogOptions struct {
	Service string
	Follow  bool
	Tail    int
}
