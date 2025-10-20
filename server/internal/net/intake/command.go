package intake

import (
	"time"

	"mine-and-die/server"
	effectcontract "mine-and-die/server/effects/contract"
	"mine-and-die/server/internal/net/proto"
	"mine-and-die/server/internal/sim"
)

type CommandContext struct {
	Engine    sim.Engine
	HasPlayer func(string) bool
	Tick      func() uint64
	Now       func() time.Time
}

func StageClientCommand(ctx CommandContext, playerID string, msg proto.ClientMessage) (sim.Command, bool, string) {
	var zero sim.Command

	command, ok := proto.ClientCommand(msg)
	if !ok {
		return zero, false, server.CommandRejectInvalidAction
	}

	switch command.Type {
	case sim.CommandMove:
		if command.Move == nil {
			return zero, false, server.CommandRejectInvalidAction
		}
	case sim.CommandSetPath:
		if command.Path == nil {
			return zero, false, server.CommandRejectInvalidAction
		}
	case sim.CommandClearPath:
	case sim.CommandAction:
		if command.Action == nil {
			return zero, false, server.CommandRejectInvalidAction
		}
		switch command.Action.Name {
		case effectcontract.EffectIDAttack, effectcontract.EffectIDFireball:
		default:
			return zero, false, server.CommandRejectInvalidAction
		}
	default:
		return zero, false, server.CommandRejectInvalidAction
	}

	if ctx.HasPlayer != nil && !ctx.HasPlayer(playerID) {
		return zero, false, server.CommandRejectUnknownActor
	}

	command.ActorID = playerID
	if ctx.Tick != nil {
		command.OriginTick = ctx.Tick()
	}
	if ctx.Now != nil {
		command.IssuedAt = ctx.Now()
	} else {
		command.IssuedAt = time.Now()
	}

	if ctx.Engine == nil {
		return zero, false, sim.CommandRejectQueueFull
	}
	if ok, reason := ctx.Engine.Enqueue(command); !ok {
		return zero, false, reason
	}

	return command, true, ""
}
