package net

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"mine-and-die/server"
	"mine-and-die/server/internal/net/proto"
)

func TestHTTPResubscribeReturnsStateSnapshot(t *testing.T) {
	hub := server.NewHubWithConfig(server.DefaultHubConfig())

	join := hub.Join()

	handler := NewHTTPHandler(hub, HTTPHandlerConfig{})

	req := httptest.NewRequest(http.MethodPost, "/resubscribe", bytes.NewReader([]byte(`{}`)))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.Code)
	}

	if contentType := resp.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", contentType)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode resubscribe payload: %v", err)
	}

	if payloadType, ok := payload["type"].(string); !ok || payloadType != proto.TypeState {
		t.Fatalf("expected state payload type %q, got %v", proto.TypeState, payload["type"])
	}

	if resync, ok := payload["resync"].(bool); !ok || !resync {
		t.Fatalf("expected resubscribe payload to set resync flag, got %v", payload["resync"])
	}

	playersValue, ok := payload["players"]
	if !ok {
		t.Fatalf("expected resubscribe payload to include players array, payload=%s", resp.Body.String())
	}
	players, ok := playersValue.([]any)
	if !ok {
		t.Fatalf("expected players to decode as array, got %T", playersValue)
	}
	if len(players) == 0 {
		t.Fatalf("expected resubscribe payload to include at least one player, payload=%s", resp.Body.String())
	}

	first, ok := players[0].(map[string]any)
	if !ok {
		t.Fatalf("expected player payload to decode as object, got %T", players[0])
	}
	if id, ok := first["id"].(string); !ok || id != join.ID {
		t.Fatalf("expected first player id %q, got %v", join.ID, first["id"])
	}
}

func TestHTTPResubscribeHonorsOptions(t *testing.T) {
	hub := server.NewHubWithConfig(server.DefaultHubConfig())

	handler := NewHTTPHandler(hub, HTTPHandlerConfig{})

	body := map[string]any{
		"includeSnapshot": false,
		"drainPatches":    true,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/resubscribe", bytes.NewReader(raw))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode resubscribe payload: %v", err)
	}

	if _, ok := payload["players"]; ok {
		t.Fatalf("expected players field to be omitted when includeSnapshot=false")
	}

	if _, ok := payload["resync"]; ok {
		t.Fatalf("expected resync flag to be omitted when draining patches")
	}
}

func TestHTTPResubscribeRejectsInvalidPayload(t *testing.T) {
	hub := server.NewHubWithConfig(server.DefaultHubConfig())

	handler := NewHTTPHandler(hub, HTTPHandlerConfig{})

	req := httptest.NewRequest(http.MethodPost, "/resubscribe", bytes.NewBufferString("{"))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", resp.Code)
	}
}

func TestHTTPResubscribeRejectsWrongMethod(t *testing.T) {
	hub := server.NewHubWithConfig(server.DefaultHubConfig())

	handler := NewHTTPHandler(hub, HTTPHandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/resubscribe", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405 Method Not Allowed, got %d", resp.Code)
	}
}

func TestDiagnosticsIncludesSubscriberQueueTelemetry(t *testing.T) {
	hub := server.NewHubWithConfig(server.DefaultHubConfig())
	handler := NewHTTPHandler(hub, HTTPHandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/diagnostics", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode diagnostics payload: %v", err)
	}

	telemetryValue, ok := payload["telemetry"].(map[string]any)
	if !ok {
		t.Fatalf("expected telemetry object in diagnostics payload, got %T", payload["telemetry"])
	}

	queuesValue, ok := telemetryValue["subscriberQueues"].(map[string]any)
	if !ok {
		t.Fatalf("expected subscriberQueues object in telemetry payload, got %T", telemetryValue["subscriberQueues"])
	}

	if _, ok := queuesValue["depth"].(float64); !ok {
		t.Fatalf("expected queue depth field in diagnostics telemetry, payload=%v", queuesValue)
	}
	if _, ok := queuesValue["maxDepth"].(float64); !ok {
		t.Fatalf("expected queue maxDepth field in diagnostics telemetry, payload=%v", queuesValue)
	}
	if _, ok := queuesValue["drops"].(float64); !ok {
		t.Fatalf("expected queue drops field in diagnostics telemetry, payload=%v", queuesValue)
	}
	if _, ok := queuesValue["dropRatePerSecond"].(float64); !ok {
		t.Fatalf("expected dropRatePerSecond field in diagnostics telemetry, payload=%v", queuesValue)
	}

	broadcastValue, ok := telemetryValue["broadcastQueue"].(map[string]any)
	if !ok {
		t.Fatalf("expected broadcastQueue object in telemetry payload, got %T", telemetryValue["broadcastQueue"])
	}
	if _, ok := broadcastValue["depth"].(float64); !ok {
		t.Fatalf("expected broadcast queue depth field in diagnostics telemetry, payload=%v", broadcastValue)
	}
	if _, ok := broadcastValue["maxDepth"].(float64); !ok {
		t.Fatalf("expected broadcast queue maxDepth field in diagnostics telemetry, payload=%v", broadcastValue)
	}
	if _, ok := broadcastValue["drops"].(float64); !ok {
		t.Fatalf("expected broadcast queue drops field in diagnostics telemetry, payload=%v", broadcastValue)
	}
	if _, ok := broadcastValue["dropRatePerSecond"].(float64); !ok {
		t.Fatalf("expected broadcast dropRatePerSecond field in diagnostics telemetry, payload=%v", broadcastValue)
	}
}

func TestDiagnosticsReportsSubscriberQueueOverflow(t *testing.T) {
	hub := server.NewHubWithConfig(server.DefaultHubConfig())
	join := hub.Join()

	conn := newDiagnosticsBlockingSubscriberConn()
	sub, _, _, _, ok := hub.Subscribe(join.ID, conn)
	if !ok {
		t.Fatalf("expected subscribe to succeed for %s", join.ID)
	}
	t.Cleanup(func() {
		sub.Close()
		conn.Close()
	})

	for i := 0; i < 128; i++ {
		hub.BroadcastState(nil, nil, nil, nil)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot := hub.TelemetrySnapshot()
		if snapshot.SubscriberQueues.Drops > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if snapshot := hub.TelemetrySnapshot(); snapshot.SubscriberQueues.Drops == 0 {
		t.Fatalf("expected telemetry snapshot to record subscriber queue drops")
	}

	handler := NewHTTPHandler(hub, HTTPHandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/diagnostics", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode diagnostics payload: %v", err)
	}

	telemetryValue, ok := payload["telemetry"].(map[string]any)
	if !ok {
		t.Fatalf("expected telemetry object in diagnostics payload, got %T", payload["telemetry"])
	}

	queuesValue, ok := telemetryValue["subscriberQueues"].(map[string]any)
	if !ok {
		t.Fatalf("expected subscriberQueues object in telemetry payload, got %T", telemetryValue["subscriberQueues"])
	}

	drops, ok := queuesValue["drops"].(float64)
	if !ok {
		t.Fatalf("expected drops field in diagnostics telemetry, payload=%v", queuesValue)
	}
	if drops == 0 {
		t.Fatalf("expected diagnostics telemetry drops to be non-zero, payload=%v", queuesValue)
	}

	maxDepth, ok := queuesValue["maxDepth"].(float64)
	if !ok {
		t.Fatalf("expected maxDepth field in diagnostics telemetry, payload=%v", queuesValue)
	}
	if maxDepth == 0 {
		t.Fatalf("expected diagnostics telemetry maxDepth to be non-zero, payload=%v", queuesValue)
	}
}

type diagnosticsBlockingSubscriberConn struct {
	tokens    chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
}

func newDiagnosticsBlockingSubscriberConn() *diagnosticsBlockingSubscriberConn {
	return &diagnosticsBlockingSubscriberConn{
		tokens: make(chan struct{}, 1024),
		closed: make(chan struct{}),
	}
}

func (c *diagnosticsBlockingSubscriberConn) Write([]byte) error {
	select {
	case <-c.closed:
		return io.ErrClosedPipe
	case <-c.tokens:
		return nil
	}
}

func (c *diagnosticsBlockingSubscriberConn) SetWriteDeadline(time.Time) error { return nil }

func (c *diagnosticsBlockingSubscriberConn) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
	})
	return nil
}

func (c *diagnosticsBlockingSubscriberConn) allow(tokens int) {
	for i := 0; i < tokens; i++ {
		select {
		case c.tokens <- struct{}{}:
		case <-c.closed:
			return
		}
	}
}
