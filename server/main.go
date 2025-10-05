package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
)

type worldState struct {
	Width  int   `json:"width"`
	Height int   `json:"height"`
	Tiles  []int `json:"tiles"`
}

type player struct {
	ID string `json:"id"`
	X  int    `json:"x"`
	Y  int    `json:"y"`
}

type stateMessage struct {
	Type    string     `json:"type"`
	YouID   string     `json:"youId"`
	World   worldState `json:"world"`
	Players []player   `json:"players"`
}

type subscriber struct {
	playerID string
	ch       chan stateMessage
}

type game struct {
	world       worldState
	mu          sync.Mutex
	players     map[string]*player
	subscribers map[string]*subscriber
	nextID      int64
}

func newGame() *game {
	return &game{
		world: worldState{
			Width:  20,
			Height: 15,
			Tiles:  buildDefaultTiles(20, 15),
		},
		players:     make(map[string]*player),
		subscribers: make(map[string]*subscriber),
	}
}

func buildDefaultTiles(width, height int) []int {
	tiles := make([]int, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if x == 0 || y == 0 || x == width-1 || y == height-1 {
				tiles[idx] = 1
				continue
			}
			if (x%4 == 0 && y%3 == 0) || (x == width/2 && y > 2 && y < height-2) {
				tiles[idx] = 1
			}
		}
	}
	return tiles
}

func (g *game) joinPlayer() *player {
	id := fmt.Sprintf("player-%d", atomic.AddInt64(&g.nextID, 1))

	g.mu.Lock()
	defer g.mu.Unlock()

	x, y := g.findSpawn()
	p := &player{ID: id, X: x, Y: y}
	g.players[id] = p
	return p
}

func (g *game) findSpawn() (int, int) {
	for y := 0; y < g.world.Height; y++ {
		for x := 0; x < g.world.Width; x++ {
			if g.isWalkableLocked(x, y) && !g.occupiedLocked(x, y) {
				return x, y
			}
		}
	}
	return g.world.Width / 2, g.world.Height / 2
}

func (g *game) isWalkableLocked(x, y int) bool {
	if x < 0 || y < 0 || x >= g.world.Width || y >= g.world.Height {
		return false
	}
	idx := y*g.world.Width + x
	return g.world.Tiles[idx] == 0
}

func (g *game) occupiedLocked(x, y int) bool {
	for _, p := range g.players {
		if p.X == x && p.Y == y {
			return true
		}
	}
	return false
}

func (g *game) movePlayer(id string, dx, dy int) {
	if dx == 0 && dy == 0 {
		return
	}

	g.mu.Lock()
	p, ok := g.players[id]
	if !ok {
		g.mu.Unlock()
		return
	}

	newX := p.X + clampDelta(dx)
	newY := p.Y + clampDelta(dy)
	if g.isWalkableLocked(newX, newY) && !g.occupiedLocked(newX, newY) {
		p.X = newX
		p.Y = newY
	}

	g.mu.Unlock()

	g.broadcastState()
}

func clampDelta(v int) int {
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return v
}

func (g *game) currentPlayers() []player {
	g.mu.Lock()
	defer g.mu.Unlock()

	list := make([]player, 0, len(g.players))
	for _, p := range g.players {
		list = append(list, *p)
	}
	return list
}

func (g *game) broadcastState() {
	g.mu.Lock()
	subscribers := make([]*subscriber, 0, len(g.subscribers))
	players := make([]player, 0, len(g.players))
	for _, p := range g.players {
		players = append(players, *p)
	}
	for _, sub := range g.subscribers {
		subscribers = append(subscribers, sub)
	}
	world := g.world
	g.mu.Unlock()

	for _, sub := range subscribers {
		msg := stateMessage{
			Type:    "state",
			YouID:   sub.playerID,
			World:   world,
			Players: players,
		}
		select {
		case sub.ch <- msg:
		default:
		}
	}
}

func (g *game) subscribe(id string) (*subscriber, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.players[id]; !ok {
		return nil, fmt.Errorf("unknown player")
	}

	if old, ok := g.subscribers[id]; ok {
		close(old.ch)
	}

	sub := &subscriber{playerID: id, ch: make(chan stateMessage, 4)}
	g.subscribers[id] = sub
	return sub, nil
}

func (g *game) unsubscribe(id string, sub *subscriber) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if current, ok := g.subscribers[id]; ok && current == sub {
		close(current.ch)
		delete(g.subscribers, id)
	}
}

func main() {
	g := newGame()

	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	http.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		p := g.joinPlayer()
		g.broadcastState()

		w.Header().Set("Content-Type", "application/json")
		response := stateMessage{
			Type:    "welcome",
			YouID:   p.ID,
			World:   g.world,
			Players: g.currentPlayers(),
		}
		_ = json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/move", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			ID string `json:"id"`
			DX int    `json:"dx"`
			DY int    `json:"dy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		g.movePlayer(payload.ID, payload.DX, payload.DY)
		w.WriteHeader(http.StatusNoContent)
	})

	http.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}

		sub, err := g.subscribe(id)
		if err != nil {
			http.Error(w, "unknown player", http.StatusNotFound)
			return
		}
		defer g.unsubscribe(id, sub)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "stream unsupported", http.StatusInternalServerError)
			return
		}

		sub.ch <- stateMessage{
			Type:    "state",
			YouID:   id,
			World:   g.world,
			Players: g.currentPlayers(),
		}

		notify := r.Context().Done()
		for {
			select {
			case <-notify:
				return
			case msg, ok := <-sub.ch:
				if !ok {
					return
				}
				data, err := json.Marshal(msg)
				if err != nil {
					log.Printf("marshal state: %v", err)
					continue
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	})

	http.Handle("/", http.FileServer(http.Dir("../client")))

	addr := ":8080"
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
