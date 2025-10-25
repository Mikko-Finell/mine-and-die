package server

import worldpkg "mine-and-die/server/internal/world"

type playerPathController struct {
	world *World
}

func newPlayerPathController(world *World) playerPathController {
	return playerPathController{world: world}
}

func (c playerPathController) SetIntent(actorID string, dx, dy float64) {
	if c.world == nil {
		return
	}
	c.world.SetIntent(actorID, dx, dy)
}

func (c playerPathController) SetFacing(actorID string, facing string) {
	if c.world == nil {
		return
	}
	c.world.SetFacing(actorID, FacingDirection(facing))
}

func (c playerPathController) DeriveFacing(dx, dy float64, fallback string) string {
	return string(deriveFacing(dx, dy, FacingDirection(fallback)))
}

func (c playerPathController) Dimensions() (float64, float64) {
	if c.world == nil {
		return 0, 0
	}
	return c.world.dimensions()
}

func (c playerPathController) ComputePlayerPath(actorID string, target worldpkg.Vec2) ([]worldpkg.Vec2, worldpkg.Vec2, bool) {
	if c.world == nil {
		return nil, worldpkg.Vec2{}, false
	}
	player := c.world.players[actorID]
	if player == nil {
		return nil, worldpkg.Vec2{}, false
	}
	return c.world.computePlayerPath(player, target)
}

func toPlayerPathActor(player *playerState) *worldpkg.PlayerPathActor {
	if player == nil {
		return nil
	}
	return &worldpkg.PlayerPathActor{
		ID:     player.ID,
		X:      player.X,
		Y:      player.Y,
		Facing: string(player.Facing),
		Path:   &player.Path,
	}
}

func (w *World) advancePlayerPaths(tick uint64) {
	controller := newPlayerPathController(w)
	for _, player := range w.players {
		worldpkg.FollowPlayerPath(toPlayerPathActor(player), tick, controller)
	}
}

func (w *World) followPlayerPath(player *playerState, tick uint64) {
	worldpkg.FollowPlayerPath(toPlayerPathActor(player), tick, newPlayerPathController(w))
}

func (w *World) finishPlayerPath(player *playerState) {
	worldpkg.FinishPlayerPath(toPlayerPathActor(player), newPlayerPathController(w))
}

func (w *World) clearPlayerPath(player *playerState) {
	worldpkg.ClearPlayerPath(toPlayerPathActor(player))
}

func (w *World) ensurePlayerPath(player *playerState, target vec2, tick uint64) bool {
	return worldpkg.EnsurePlayerPath(toPlayerPathActor(player), worldpkg.Vec2(target), tick, newPlayerPathController(w))
}

func (w *World) recalculatePlayerPath(player *playerState, tick uint64) bool {
	return worldpkg.RecalculatePlayerPath(toPlayerPathActor(player), tick, newPlayerPathController(w))
}

func (w *World) computePlayerPath(player *playerState, target vec2) ([]vec2, vec2, bool) {
	if w == nil || player == nil {
		return nil, vec2{}, false
	}
	return w.computePathFrom(player.X, player.Y, player.ID, target)
}
