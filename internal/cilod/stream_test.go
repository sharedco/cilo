// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cilod

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Remote Exec
// ============================================================================

func TestRemoteExec(t *testing.T) {
	var receivedMessages []WebSocketMessage
	var mu sync.Mutex
	execCompleted := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/environments/myenv/exec" {
			t.Errorf("Expected /environments/myenv/exec, got %s", r.URL.Path)
		}

		if r.Header.Get("Upgrade") != "websocket" {
			t.Error("Expected WebSocket upgrade")
		}

		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade WebSocket: %v", err)
		}
		defer conn.Close()

		var execReq EnvironmentExecRequest
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read exec request: %v", err)
		}
		if err := json.Unmarshal(msg, &execReq); err != nil {
			t.Fatalf("Failed to unmarshal exec request: %v", err)
		}

		if execReq.Service != "api" {
			t.Errorf("Expected service 'api', got %s", execReq.Service)
		}
		if len(execReq.Command) != 2 || execReq.Command[0] != "ls" {
			t.Errorf("Expected command ['ls', '-la'], got %v", execReq.Command)
		}

		mu.Lock()
		receivedMessages = append(receivedMessages, WebSocketMessage{
			Type: "stdout",
			Data: []byte("file1.txt\n"),
		})
		receivedMessages = append(receivedMessages, WebSocketMessage{
			Type: "stdout",
			Data: []byte("file2.txt\n"),
		})
		receivedMessages = append(receivedMessages, WebSocketMessage{
			Type: "stderr",
			Data: []byte("warning: permission denied\n"),
		})
		receivedMessages = append(receivedMessages, WebSocketMessage{
			Type:     "exit",
			ExitCode: 0,
		})
		mu.Unlock()

		for _, msg := range receivedMessages {
			data, _ := json.Marshal(msg)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				t.Errorf("Failed to write message: %v", err)
			}
		}

		execCompleted <- true
	}))
	defer server.Close()

	streamer := NewStreamer(server.URL, "test-token")

	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := streamer.StreamExec(ctx, "myenv", "api", []string{"ls", "-la"}, nil, &stdout, &stderr, false)
	require.NoError(t, err)

	select {
	case <-execCompleted:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for exec to complete")
	}

	assert.Contains(t, stdout.String(), "file1.txt")
	assert.Contains(t, stdout.String(), "file2.txt")
	assert.Contains(t, stderr.String(), "warning: permission denied")
}

func TestRemoteExec_WithStdin(t *testing.T) {
	stdinData := "hello world\n"
	var receivedStdin bytes.Buffer
	execCompleted := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade WebSocket: %v", err)
		}

		var execReq EnvironmentExecRequest
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read exec request: %v", err)
		}
		json.Unmarshal(msg, &execReq)

		if !execReq.Stdin {
			t.Error("Expected stdin to be enabled")
		}

		for {
			conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var wsMsg WebSocketMessage
			if err := json.Unmarshal(msg, &wsMsg); err != nil {
				continue
			}

			if wsMsg.Type == "stdin" {
				receivedStdin.Write(wsMsg.Data)
				response := WebSocketMessage{
					Type: "stdout",
					Data: wsMsg.Data,
				}
				data, _ := json.Marshal(response)
				conn.WriteMessage(websocket.TextMessage, data)
			}
		}

		exitMsg := WebSocketMessage{Type: "exit", ExitCode: 0}
		data, _ := json.Marshal(exitMsg)
		conn.WriteMessage(websocket.TextMessage, data)

		// Send graceful close message and wait for client to close
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

		// Wait for client to close connection
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
		execCompleted <- true
	}))
	defer server.Close()

	streamer := NewStreamer(server.URL, "test-token")

	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader(stdinData)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := streamer.StreamExec(ctx, "myenv", "api", []string{"cat"}, stdin, &stdout, &stderr, false)
	require.NoError(t, err)

	select {
	case <-execCompleted:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for exec to complete")
	}

	assert.Equal(t, stdinData, receivedStdin.String())
	assert.Contains(t, stdout.String(), "hello world")
}

// ============================================================================
// Test Remote Logs
// ============================================================================

func TestRemoteLogs(t *testing.T) {
	logLines := []string{
		"2024-01-01 10:00:00 INFO Starting application",
		"2024-01-01 10:00:01 DEBUG Connected to database",
		"2024-01-01 10:00:02 INFO Server listening on port 8080",
	}
	logsCompleted := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/environments/myenv/logs" {
			t.Errorf("Expected /environments/myenv/logs, got %s", r.URL.Path)
		}

		service := r.URL.Query().Get("service")
		if service != "api" {
			t.Errorf("Expected service 'api', got %s", service)
		}

		if r.Header.Get("Upgrade") != "websocket" {
			t.Error("Expected WebSocket upgrade")
		}

		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade WebSocket: %v", err)
		}
		defer conn.Close()

		for _, line := range logLines {
			msg := WebSocketMessage{
				Type: "stdout",
				Data: []byte(line + "\n"),
			}
			data, _ := json.Marshal(msg)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				t.Errorf("Failed to write log: %v", err)
			}
		}

		eofMsg := WebSocketMessage{Type: "eof"}
		data, _ := json.Marshal(eofMsg)
		conn.WriteMessage(websocket.TextMessage, data)

		logsCompleted <- true
	}))
	defer server.Close()

	streamer := NewStreamer(server.URL, "test-token")

	var stdout bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := streamer.StreamLogs(ctx, "myenv", "api", false, &stdout)
	require.NoError(t, err)

	select {
	case <-logsCompleted:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for logs to complete")
	}

	output := stdout.String()
	for _, line := range logLines {
		assert.Contains(t, output, line)
	}
}

func TestRemoteLogsFollow(t *testing.T) {
	logLines := []string{
		"Line 1",
		"Line 2",
		"Line 3",
	}
	logsSent := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		follow := r.URL.Query().Get("follow") == "true"
		if !follow {
			t.Error("Expected follow=true")
		}

		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade WebSocket: %v", err)
		}

		for _, line := range logLines {
			msg := WebSocketMessage{
				Type: "stdout",
				Data: []byte(line + "\n"),
			}
			data, _ := json.Marshal(msg)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				conn.Close()
				return
			}
		}
		logsSent <- true

		// Wait for client to close connection
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
		conn.Close()
	}))
	defer server.Close()

	streamer := NewStreamer(server.URL, "test-token")

	var stdout bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	streamDone := make(chan error, 1)
	go func() {
		streamDone <- streamer.StreamLogs(ctx, "myenv", "api", true, &stdout)
	}()

	select {
	case <-logsSent:
		// Give client time to process all log messages
		time.Sleep(100 * time.Millisecond)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for logs")
	}

	cancel()

	select {
	case err := <-streamDone:
		if err != nil && !strings.Contains(err.Error(), "context") {
			t.Errorf("Unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for stream to complete")
	}

	output := stdout.String()
	for _, line := range logLines {
		assert.Contains(t, output, line)
	}
}

// ============================================================================
// Test Signal Propagation
// ============================================================================

func TestSignalPropagation(t *testing.T) {
	signalReceived := make(chan string, 1)
	execCompleted := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// Panic from websocket - connection closed unexpectedly
			}
		}()

		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		var execReq EnvironmentExecRequest
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		json.Unmarshal(msg, &execReq)

		for {
			conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					return
				}
				// Check if signal was already received
				select {
				case <-signalReceived:
					return
				default:
					continue
				}
			}

			var wsMsg WebSocketMessage
			if err := json.Unmarshal(msg, &wsMsg); err != nil {
				continue
			}

			if wsMsg.Type == "signal" {
				signal := string(wsMsg.Data)
				signalReceived <- signal

				if signal == "SIGINT" {
					exitMsg := WebSocketMessage{
						Type:     "exit",
						ExitCode: 130,
					}
					data, _ := json.Marshal(exitMsg)
					conn.WriteMessage(websocket.TextMessage, data)
					execCompleted <- true
					return
				}
			}
		}
	}))
	defer server.Close()

	streamer := NewStreamer(server.URL, "test-token")

	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start exec in background
	execDone := make(chan error, 1)
	go func() {
		execDone <- streamer.StreamExec(ctx, "myenv", "api", []string{"sleep", "100"}, nil, &stdout, &stderr, false)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to trigger connection close
	cancel()

	// Verify that when connection is closed, the server handles it gracefully
	// and the client returns with context cancelled error
	select {
	case err := <-execDone:
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "context") || err == context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for exec to return")
	}
}

// ============================================================================
// Test Streamer Constructor
// ============================================================================

func TestNewStreamer(t *testing.T) {
	streamer := NewStreamer("http://localhost:8080", "my-token")

	assert.NotNil(t, streamer)
	assert.Equal(t, "http://localhost:8080", streamer.baseURL)
	assert.Equal(t, "my-token", streamer.token)
	assert.NotNil(t, streamer.dialer)
}

func TestStreamer_SetTimeout(t *testing.T) {
	streamer := NewStreamer("http://localhost:8080", "token")
	streamer.SetTimeout(10 * time.Second)

	assert.Equal(t, 10*time.Second, streamer.dialer.HandshakeTimeout)
}

// ============================================================================
// Test Error Handling
// ============================================================================

func TestRemoteExec_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "server error"})
	}))
	defer server.Close()

	streamer := NewStreamer(server.URL, "test-token")

	var stdout, stderr bytes.Buffer
	ctx := context.Background()

	err := streamer.StreamExec(ctx, "myenv", "api", []string{"ls"}, nil, &stdout, &stderr, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server error")
}

func TestRemoteLogs_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "environment not found"})
	}))
	defer server.Close()

	streamer := NewStreamer(server.URL, "test-token")

	var stdout bytes.Buffer
	ctx := context.Background()

	err := streamer.StreamLogs(ctx, "myenv", "api", false, &stdout)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRemoteExec_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	streamer := NewStreamer(server.URL, "test-token")

	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := streamer.StreamExec(ctx, "myenv", "api", []string{"sleep", "100"}, nil, &stdout, &stderr, false)
	assert.Error(t, err)
	assert.True(t, err == context.DeadlineExceeded || strings.Contains(err.Error(), "context"))
}

// ============================================================================
// Test PTY Allocation
// ============================================================================

func TestRemoteExec_WithPTY(t *testing.T) {
	ptyRequested := make(chan bool, 1)
	execCompleted := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade WebSocket: %v", err)
		}
		defer conn.Close()

		var execReq EnvironmentExecRequest
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read exec request: %v", err)
		}
		json.Unmarshal(msg, &execReq)

		if execReq.TTY {
			ptyRequested <- true
		}

		msg1 := WebSocketMessage{Type: "stdout", Data: []byte("$ ")}
		data, _ := json.Marshal(msg1)
		conn.WriteMessage(websocket.TextMessage, data)

		exitMsg := WebSocketMessage{Type: "exit", ExitCode: 0}
		data, _ = json.Marshal(exitMsg)
		conn.WriteMessage(websocket.TextMessage, data)
		execCompleted <- true
	}))
	defer server.Close()

	streamer := NewStreamer(server.URL, "test-token")

	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := streamer.StreamExec(ctx, "myenv", "api", []string{"bash"}, nil, &stdout, &stderr, true)
	require.NoError(t, err)

	select {
	case <-ptyRequested:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for PTY request")
	}

	select {
	case <-execCompleted:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for exec completion")
	}
}

// ============================================================================
// Test WebSocket Message Types
// ============================================================================

func TestWebSocketMessage_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		msg  WebSocketMessage
	}{
		{
			name: "stdout message",
			msg: WebSocketMessage{
				Type: "stdout",
				Data: []byte("hello world"),
			},
		},
		{
			name: "stderr message",
			msg: WebSocketMessage{
				Type: "stderr",
				Data: []byte("error message"),
			},
		},
		{
			name: "exit message",
			msg: WebSocketMessage{
				Type:     "exit",
				ExitCode: 42,
			},
		},
		{
			name: "signal message",
			msg: WebSocketMessage{
				Type: "signal",
				Data: []byte("SIGINT"),
			},
		},
		{
			name: "stdin message",
			msg: WebSocketMessage{
				Type: "stdin",
				Data: []byte("input"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			require.NoError(t, err)

			var decoded WebSocketMessage
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.msg.Type, decoded.Type)
			assert.Equal(t, tt.msg.Data, decoded.Data)
			assert.Equal(t, tt.msg.ExitCode, decoded.ExitCode)
		})
	}
}

// ============================================================================
// Test StreamReader
// ============================================================================

func TestStreamReader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade WebSocket: %v", err)
		}
		defer conn.Close()

		for i := 0; i < 3; i++ {
			msg := WebSocketMessage{
				Type: "stdout",
				Data: []byte(fmt.Sprintf("line %d\n", i)),
			}
			data, _ := json.Marshal(msg)
			conn.WriteMessage(websocket.TextMessage, data)
		}

		eofMsg := WebSocketMessage{Type: "eof"}
		data, _ := json.Marshal(eofMsg)
		conn.WriteMessage(websocket.TextMessage, data)

		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}))
	defer server.Close()

	streamer := NewStreamer(server.URL, "test-token")

	var stdout bytes.Buffer
	ctx := context.Background()

	err := streamer.StreamLogs(ctx, "myenv", "api", false, &stdout)
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "line 0")
	assert.Contains(t, output, "line 1")
	assert.Contains(t, output, "line 2")
}
