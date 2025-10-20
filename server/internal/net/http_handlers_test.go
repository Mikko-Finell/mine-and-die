package net

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
