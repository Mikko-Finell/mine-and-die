package server

import (
	"math"
	"sort"
	"time"

	ai "mine-and-die/server/internal/ai"
)

func (w *World) runAI(tick uint64, now time.Time) []Command {
	if w == nil || w.aiLibrary == nil || len(w.npcs) == 0 {
		return nil
	}

	aiNPCs := make([]*ai.NPC, 0, len(w.npcs))
	for _, npc := range w.npcs {
		if npc == nil {
			continue
		}
		npc := npc
		aiNPCs = append(aiNPCs, &ai.NPC{
			ID:         npc.ID,
			Type:       string(npc.Type),
			AIConfigID: npc.AIConfigID,
			AIState:    &npc.AIState,
			Position: ai.PositionRef{
				X: &npc.X,
				Y: &npc.Y,
			},
			Facing: ai.FacingAdapter{
				Get: func() string { return string(npc.Facing) },
				Set: func(value string) {
					w.SetNPCFacing(npc.ID, FacingDirection(value))
				},
			},
			Waypoints:  (*[]ai.Vec2)(&npc.Waypoints),
			Home:       (*ai.Vec2)(&npc.Home),
			Blackboard: &npc.Blackboard,
			Hooks: ai.NPCHooks{
				ClearPath: func() { w.clearNPCPath(npc) },
				EnsurePath: func(target ai.Vec2, tick uint64) bool {
					return w.ensureNPCPath(npc, vec2(target), tick)
				},
			},
		})
	}
	if len(aiNPCs) == 0 {
		return nil
	}

	players := make([]ai.Player, 0, len(w.players))
	for id, player := range w.players {
		if player == nil {
			continue
		}
		players = append(players, ai.Player{ID: id, X: player.X, Y: player.Y})
	}
	sort.Slice(players, func(i, j int) bool { return players[i].ID < players[j].ID })

	width, height := w.dimensions()

	runCfg := ai.RunConfig{
		Tick:    tick,
		Now:     now,
		Width:   width,
		Height:  height,
		Library: w.aiLibrary,
		NPCs:    aiNPCs,
		Players: players,
		RandomAngle: func() float64 {
			return w.randomAngle()
		},
		RandomDistance: func(min, max float64) float64 {
			return w.randomDistance(min, max)
		},
		DeriveFacing: func(dx, dy float64, fallback string) string {
			return string(deriveFacing(dx, dy, FacingDirection(fallback)))
		},
		AbilityCommand: func(id ai.AbilityID) (string, bool) {
			switch id {
			case ai.AbilityAttack:
				return effectTypeAttack, true
			case ai.AbilityFireball:
				return effectTypeFireball, true
			default:
				return "", false
			}
		},
		AbilityCooldown: func(id ai.AbilityID) uint64 {
			switch id {
			case ai.AbilityAttack:
				return uint64(math.Ceil(meleeAttackCooldown.Seconds() * float64(tickRate)))
			case ai.AbilityFireball:
				return uint64(math.Ceil(fireballCooldown.Seconds() * float64(tickRate)))
			default:
				return 0
			}
		},
	}

	aiCommands := ai.Run(runCfg)
	if len(aiCommands) == 0 {
		return nil
	}

	commands := make([]Command, 0, len(aiCommands))
	for _, cmd := range aiCommands {
		commands = append(commands, convertAICommand(cmd))
	}
	return commands
}

func convertAICommand(cmd ai.Command) Command {
	result := Command{
		OriginTick: cmd.OriginTick,
		ActorID:    cmd.ActorID,
		Type:       CommandType(cmd.Type),
		IssuedAt:   cmd.IssuedAt,
	}
	if cmd.Move != nil {
		result.Move = &MoveCommand{
			DX:     cmd.Move.DX,
			DY:     cmd.Move.DY,
			Facing: FacingDirection(cmd.Move.Facing),
		}
	}
	if cmd.Action != nil {
		result.Action = &ActionCommand{Name: cmd.Action.Name}
	}
	if cmd.Heartbeat != nil {
		result.Heartbeat = &HeartbeatCommand{
			ReceivedAt: cmd.Heartbeat.ReceivedAt,
			ClientSent: cmd.Heartbeat.ClientSent,
			RTT:        cmd.Heartbeat.RTT,
		}
	}
	if cmd.Path != nil {
		result.Path = &PathCommand{TargetX: cmd.Path.TargetX, TargetY: cmd.Path.TargetY}
	}
	return result
}
