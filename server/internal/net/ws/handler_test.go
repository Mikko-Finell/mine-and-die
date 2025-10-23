package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"

	"mine-and-die/server"
	"mine-and-die/server/internal/net/proto"
)

func TestHandleSubscribeInitialStateUsesSharedGroundItems(t *testing.T) {
	hub := server.NewHubWithConfig(server.DefaultHubConfig())
	join := hub.Join()

	ack, handled := hub.HandleConsoleCommand(join.ID, "drop_gold", 3)
	if !handled {
		t.Fatalf("expected drop_gold command to be handled")
	}
	if ack.Status != "ok" {
		t.Fatalf("drop_gold command failed: %+v", ack)
	}

	handler := NewHandler(hub, HandlerConfig{})
	srv := httptest.NewServer(http.HandlerFunc(handler.Handle))
	t.Cleanup(srv.Close)

	conn, resp, err := websocket.DefaultDialer.Dial(websocketURL(t, srv.URL, join.ID), nil)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatalf("failed to open websocket connection: %v", err)
	}
	t.Cleanup(func() {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		conn.Close()
		if resp != nil {
			resp.Body.Close()
		}
	})

	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial state: %v", err)
	}

	assertGroundItemPayload(t, payload, ack.StackID, ack.Qty)
}

func TestHandleResubscribeInitialStateUsesSharedGroundItems(t *testing.T) {
	hub := server.NewHubWithConfig(server.DefaultHubConfig())
	join := hub.Join()

	ack, handled := hub.HandleConsoleCommand(join.ID, "drop_gold", 4)
	if !handled {
		t.Fatalf("expected drop_gold command to be handled")
	}
	if ack.Status != "ok" {
		t.Fatalf("drop_gold command failed: %+v", ack)
	}

	handler := NewHandler(hub, HandlerConfig{})
	srv := httptest.NewServer(http.HandlerFunc(handler.Handle))
	t.Cleanup(srv.Close)

	firstConn, firstResp, err := websocket.DefaultDialer.Dial(websocketURL(t, srv.URL, join.ID), nil)
	if err != nil {
		if firstResp != nil {
			firstResp.Body.Close()
		}
		t.Fatalf("failed to open initial websocket connection: %v", err)
	}
	if firstResp != nil {
		t.Cleanup(func() {
			firstResp.Body.Close()
		})
	}
	t.Cleanup(func() {
		firstConn.Close()
	})

	_, firstPayload, err := firstConn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial subscription payload: %v", err)
	}
	assertGroundItemPayload(t, firstPayload, ack.StackID, ack.Qty)

	secondConn, secondResp, err := websocket.DefaultDialer.Dial(websocketURL(t, srv.URL, join.ID), nil)
	if err != nil {
		if secondResp != nil {
			secondResp.Body.Close()
		}
		t.Fatalf("failed to open resubscribe websocket connection: %v", err)
	}
	t.Cleanup(func() {
		secondConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		secondConn.Close()
		if secondResp != nil {
			secondResp.Body.Close()
		}
	})

	_, secondPayload, err := secondConn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read resubscribe payload: %v", err)
	}
	assertGroundItemPayload(t, secondPayload, ack.StackID, ack.Qty)
}

func websocketURL(t *testing.T, baseURL, playerID string) string {
	t.Helper()

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("failed to parse test server url: %v", err)
	}
	parsed.Scheme = "ws"
	parsed.Path = "/"
	query := parsed.Query()
	query.Set("id", playerID)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func assertGroundItemPayload(t *testing.T, payload []byte, expectedID string, expectedQty int) {
	t.Helper()

	var frame map[string]any
	if err := json.Unmarshal(payload, &frame); err != nil {
		t.Fatalf("failed to decode websocket payload: %v", err)
	}

	msgType, ok := frame["type"].(string)
	if !ok || msgType != proto.TypeState {
		t.Fatalf("expected state payload type %q, got %v", proto.TypeState, frame["type"])
	}

	rawItems, ok := frame["groundItems"].([]any)
	if !ok {
		t.Fatalf("expected groundItems array in payload, got %T", frame["groundItems"])
	}
	if len(rawItems) == 0 {
		t.Fatalf("expected groundItems to include at least one entry")
	}

	var selected map[string]any
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]any)
		if !ok {
			t.Fatalf("expected ground item to decode as object, got %T", rawItem)
		}
		if expectedID != "" {
			if id, ok := item["id"].(string); ok && id == expectedID {
				selected = item
				break
			}
			continue
		}
		if id, ok := item["id"].(string); ok && id != "" {
			selected = item
			break
		}
	}

	if selected == nil {
		t.Fatalf("expected to find ground item with id %q", expectedID)
	}

	if id, ok := selected["id"].(string); !ok || id == "" {
		t.Fatalf("expected ground item to include non-empty id, got %v", selected["id"])
	}
	if expectedID != "" && selected["id"].(string) != expectedID {
		t.Fatalf("expected ground item id %q, got %v", expectedID, selected["id"])
	}

	if typ, ok := selected["type"].(string); !ok || typ != string(server.ItemTypeGold) {
		t.Fatalf("expected ground item type %q, got %v", server.ItemTypeGold, selected["type"])
	}

	if _, ok := selected["fungibility_key"].(string); !ok {
		t.Fatalf("expected fungibility_key field in ground item, got %v", selected)
	}

	qtyValue, ok := selected["qty"].(float64)
	if !ok {
		t.Fatalf("expected qty to decode as number, got %T", selected["qty"])
	}
	if int(qtyValue) != expectedQty {
		t.Fatalf("expected qty %d, got %d", expectedQty, int(qtyValue))
	}
}
