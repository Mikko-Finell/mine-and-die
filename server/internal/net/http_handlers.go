package net

import (
	"encoding/json"
	"io"
	"log"
	nethttp "net/http"
	"net/http/pprof"
	"time"

	"mine-and-die/server"
	itemspkg "mine-and-die/server/internal/items"
	"mine-and-die/server/internal/net/proto"
	"mine-and-die/server/internal/net/ws"
	"mine-and-die/server/internal/observability"
	"mine-and-die/server/internal/sim"
	"mine-and-die/server/internal/telemetry"
)

type HTTPHandlerConfig struct {
	ClientDir     string
	Logger        telemetry.Logger
	Observability observability.Config
}

func NewHTTPHandler(hub *server.Hub, cfg HTTPHandlerConfig) nethttp.Handler {
	telemetryLogger := cfg.Logger
	if telemetryLogger == nil {
		telemetryLogger = telemetry.WrapLogger(log.Default())
	}

	mux := nethttp.NewServeMux()

	registerPprofHandlers(mux, cfg.Observability.EnablePprofTrace)

	mux.HandleFunc("/health", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/diagnostics", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		payload := struct {
			Status     string `json:"status"`
			ServerTime int64  `json:"serverTime"`
			Players    any    `json:"players"`
			TickRate   int    `json:"tickRate"`
			Heartbeat  int64  `json:"heartbeatMillis"`
			Telemetry  any    `json:"telemetry"`
		}{
			Status:     "ok",
			ServerTime: time.Now().UnixMilli(),
			Players:    hub.DiagnosticsSnapshot(),
			TickRate:   server.TickRate(),
			Heartbeat:  server.HeartbeatInterval().Milliseconds(),
			Telemetry:  hub.TelemetrySnapshot(),
		}

		data, err := json.Marshal(payload)
		if err != nil {
			httpError(w, "failed to encode", nethttp.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	mux.HandleFunc("/world/reset", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodPost {
			httpError(w, "method not allowed", nethttp.StatusMethodNotAllowed)
			return
		}

		cfg := hub.CurrentConfig()

		type resetRequest struct {
			Obstacles      *bool   `json:"obstacles"`
			ObstaclesCount *int    `json:"obstaclesCount"`
			GoldMines      *bool   `json:"goldMines"`
			GoldMineCount  *int    `json:"goldMineCount"`
			NPCs           *bool   `json:"npcs"`
			GoblinCount    *int    `json:"goblinCount"`
			RatCount       *int    `json:"ratCount"`
			NPCCount       *int    `json:"npcCount"`
			Lava           *bool   `json:"lava"`
			LavaCount      *int    `json:"lavaCount"`
			Seed           *string `json:"seed"`
		}

		if r.Body != nil {
			defer r.Body.Close()
			var req resetRequest
			decoder := json.NewDecoder(r.Body)
			if err := decoder.Decode(&req); err != nil && err != io.EOF {
				httpError(w, "invalid payload", nethttp.StatusBadRequest)
				return
			}
			if req.Obstacles != nil {
				cfg.Obstacles = *req.Obstacles
			}
			if req.ObstaclesCount != nil {
				cfg.ObstaclesCount = *req.ObstaclesCount
			}
			if req.GoldMines != nil {
				cfg.GoldMines = *req.GoldMines
			}
			if req.GoldMineCount != nil {
				cfg.GoldMineCount = *req.GoldMineCount
			}
			if req.NPCs != nil {
				cfg.NPCs = *req.NPCs
			}
			if req.GoblinCount != nil {
				cfg.GoblinCount = *req.GoblinCount
			}
			if req.RatCount != nil {
				cfg.RatCount = *req.RatCount
			}
			if req.NPCCount != nil {
				cfg.NPCCount = *req.NPCCount
				if req.GoblinCount == nil && req.RatCount == nil {
					goblins := cfg.NPCCount
					if goblins > 2 {
						goblins = 2
					}
					if goblins < 0 {
						goblins = 0
					}
					cfg.GoblinCount = goblins
					rats := cfg.NPCCount - goblins
					if rats < 0 {
						rats = 0
					}
					cfg.RatCount = rats
				}
			}
			if req.Lava != nil {
				cfg.Lava = *req.Lava
			}
			if req.LavaCount != nil {
				cfg.LavaCount = *req.LavaCount
			}
			if req.Seed != nil {
				cfg.Seed = *req.Seed
			}
		}

		cfg = cfg.Normalized()

		players, npcs := hub.ResetWorld(cfg)
		hub.ForceKeyframe()
		hub.BroadcastState(players, npcs, nil, nil)

		response := struct {
			Status string `json:"status"`
			Config any    `json:"config"`
		}{
			Status: "ok",
			Config: cfg,
		}

		data, err := json.Marshal(response)
		if err != nil {
			httpError(w, "failed to encode", nethttp.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	mux.HandleFunc("/join", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodPost {
			httpError(w, "method not allowed", nethttp.StatusMethodNotAllowed)
			return
		}

		join := hub.Join()
		data, err := proto.EncodeJoinResponse(join)
		if err != nil {
			httpError(w, "failed to encode", nethttp.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	mux.HandleFunc("/resubscribe", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodPost {
			httpError(w, "method not allowed", nethttp.StatusMethodNotAllowed)
			return
		}

		type resubscribeRequest struct {
			Players         []sim.Player          `json:"players"`
			NPCs            []sim.NPC             `json:"npcs"`
			EffectTriggers  []sim.EffectTrigger   `json:"effectTriggers"`
			GroundItems     []itemspkg.GroundItem `json:"groundItems"`
			DrainPatches    *bool                 `json:"drainPatches"`
			IncludeSnapshot *bool                 `json:"includeSnapshot"`
		}

		var req resubscribeRequest
		if r.Body != nil {
			defer r.Body.Close()
			decoder := json.NewDecoder(r.Body)
			if err := decoder.Decode(&req); err != nil && err != io.EOF {
				httpError(w, "invalid payload", nethttp.StatusBadRequest)
				return
			}
		}

		drainPatches := false
		if req.DrainPatches != nil {
			drainPatches = *req.DrainPatches
		}

		includeSnapshot := true
		if req.IncludeSnapshot != nil {
			includeSnapshot = *req.IncludeSnapshot
		}

		data, _, err := hub.MarshalState(req.Players, req.NPCs, req.EffectTriggers, req.GroundItems, drainPatches, includeSnapshot)
		if err != nil {
			httpError(w, "failed to encode", nethttp.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	mux.HandleFunc("/effects/catalog", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodGet {
			httpError(w, "method not allowed", nethttp.StatusMethodNotAllowed)
			return
		}

		catalog := hub.EffectCatalogSnapshot()
		var payloadCatalog any = catalog
		if payloadCatalog == nil {
			payloadCatalog = map[string]any{}
		}

		payload := struct {
			Catalog any `json:"effectCatalog"`
		}{Catalog: payloadCatalog}

		data, err := json.Marshal(payload)
		if err != nil {
			httpError(w, "failed to encode", nethttp.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	wsHandler := ws.NewHandler(hub, ws.HandlerConfig{
		Logger: telemetryLogger,
	})
	mux.HandleFunc("/ws", wsHandler.Handle)

	if cfg.ClientDir != "" {
		fs := nethttp.FileServer(nethttp.Dir(cfg.ClientDir))
		mux.Handle("/", fs)
	}

	return mux
}

func httpError(w nethttp.ResponseWriter, msg string, code int) {
	nethttp.Error(w, msg, code)
}

func registerPprofHandlers(mux *nethttp.ServeMux, enableTrace bool) {
	mux.HandleFunc("/debug/pprof/", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.URL.Path != "/debug/pprof/" {
			nethttp.NotFound(w, r)
			return
		}
		pprof.Index(w, r)
	})

	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)

	profiles := []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"}
	for _, name := range profiles {
		mux.Handle("/debug/pprof/"+name, pprof.Handler(name))
	}

	if enableTrace {
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		return
	}

	mux.HandleFunc("/debug/pprof/trace", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		httpError(w, "pprof trace disabled", nethttp.StatusNotFound)
	})
}
