package server

import worldpkg "mine-and-die/server/internal/world"

type npcPathController struct {
	world *World
}

func newNPCPathController(world *World) npcPathController {
	return npcPathController{world: world}
}

func (c npcPathController) SetIntent(actorID string, dx, dy float64) {
	if c.world == nil {
		return
	}
	npc := c.world.npcs[actorID]
	if npc == nil {
		return
	}
	npc.intentX = dx
	npc.intentY = dy
}

func (c npcPathController) SetFacing(actorID string, facing string) {
	if c.world == nil {
		return
	}
	c.world.SetNPCFacing(actorID, FacingDirection(facing))
}

func (c npcPathController) DeriveFacing(dx, dy float64, fallback string) string {
	return string(deriveFacing(dx, dy, FacingDirection(fallback)))
}

func (c npcPathController) ComputeNPCPath(actorID string, target worldpkg.Vec2) ([]worldpkg.Vec2, worldpkg.Vec2, bool) {
	if c.world == nil {
		return nil, worldpkg.Vec2{}, false
	}
	npc := c.world.npcs[actorID]
	if npc == nil {
		return nil, worldpkg.Vec2{}, false
	}
	return c.world.computeNPCPath(npc, vec2(target))
}

func toNPCPathActor(npc *npcState) *worldpkg.NPCPathActor {
	if npc == nil {
		return nil
	}
	return &worldpkg.NPCPathActor{
		ID:           npc.ID,
		X:            npc.X,
		Y:            npc.Y,
		Facing:       string(npc.Facing),
		Path:         &npc.Blackboard.NPCPathState,
		StuckCounter: npc.Blackboard.StuckCounter,
	}
}

func (w *World) advanceNPCPaths(tick uint64) {
	controller := newNPCPathController(w)
	for _, npc := range w.npcs {
		worldpkg.FollowNPCPath(toNPCPathActor(npc), tick, controller)
	}
}

func (w *World) followNPCPath(npc *npcState, tick uint64) {
	worldpkg.FollowNPCPath(toNPCPathActor(npc), tick, newNPCPathController(w))
}

func (w *World) finishNPCPath(npc *npcState) {
	worldpkg.FinishNPCPath(toNPCPathActor(npc), newNPCPathController(w))
}

func (w *World) clearNPCPath(npc *npcState) {
	worldpkg.ClearNPCPath(toNPCPathActor(npc), newNPCPathController(w))
}

func (w *World) ensureNPCPath(npc *npcState, target vec2, tick uint64) bool {
	return worldpkg.EnsureNPCPath(toNPCPathActor(npc), worldpkg.Vec2(target), tick, newNPCPathController(w))
}

func (w *World) recalculateNPCPath(npc *npcState, tick uint64) bool {
	return worldpkg.RecalculateNPCPath(toNPCPathActor(npc), tick, newNPCPathController(w))
}

func (w *World) computeNPCPath(npc *npcState, target vec2) ([]vec2, vec2, bool) {
	if w == nil || npc == nil {
		return nil, vec2{}, false
	}
	return w.computePathFrom(npc.X, npc.Y, npc.ID, target)
}
