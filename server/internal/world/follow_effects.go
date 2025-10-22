package world

import "time"

// LegacyFollowEffect captures the subset of legacy effect fields required to
// update follow attachments without importing the legacy world types.
type LegacyFollowEffect struct {
	FollowActorID string
	Width         float64
	Height        float64
}

// LegacyFollowEffectAdvanceConfig bundles the callbacks required to iterate
// legacy effects and update their follow attachments inside the world package.
type LegacyFollowEffectAdvanceConfig struct {
	Now time.Time

	ForEachEffect func(func(effect any))
	Inspect       func(effect any) LegacyFollowEffect
	ActorByID     func(string) *Actor

	Expire      func(effect any, now time.Time)
	ClearFollow func(effect any)
	SetPosition func(effect any, x, y float64)
}

// LegacyFollowEffectUpdateConfig carries the dependencies required to update a
// single follow effect instance.
type LegacyFollowEffectUpdateConfig struct {
	Effect any
	Fields LegacyFollowEffect
	Actor  *Actor
	Now    time.Time

	Expire      func(effect any, now time.Time)
	ClearFollow func(effect any)
	SetPosition func(effect any, x, y float64)
}

// AdvanceLegacyFollowEffects walks the provided legacy effects and updates any
// follow attachments using the supplied callbacks.
func AdvanceLegacyFollowEffects(cfg LegacyFollowEffectAdvanceConfig) {
	if cfg.ForEachEffect == nil || cfg.Inspect == nil {
		return
	}

	cfg.ForEachEffect(func(effect any) {
		fields := cfg.Inspect(effect)
		var actor *Actor
		if fields.FollowActorID != "" && cfg.ActorByID != nil {
			actor = cfg.ActorByID(fields.FollowActorID)
		}
		UpdateLegacyFollowEffect(LegacyFollowEffectUpdateConfig{
			Effect:      effect,
			Fields:      fields,
			Actor:       actor,
			Now:         cfg.Now,
			Expire:      cfg.Expire,
			ClearFollow: cfg.ClearFollow,
			SetPosition: cfg.SetPosition,
		})
	})
}

// UpdateLegacyFollowEffect applies follow-attachment semantics for a single
// legacy effect instance using the provided callbacks.
func UpdateLegacyFollowEffect(cfg LegacyFollowEffectUpdateConfig) {
	if cfg.Effect == nil {
		return
	}

	followID := cfg.Fields.FollowActorID
	if followID == "" {
		return
	}

	if cfg.Actor == nil {
		if cfg.Expire != nil {
			cfg.Expire(cfg.Effect, cfg.Now)
		}
		if cfg.ClearFollow != nil {
			cfg.ClearFollow(cfg.Effect)
		}
		return
	}

	width := cfg.Fields.Width
	if width <= 0 {
		width = PlayerHalf * 2
	}
	height := cfg.Fields.Height
	if height <= 0 {
		height = PlayerHalf * 2
	}

	if cfg.SetPosition != nil {
		cfg.SetPosition(cfg.Effect, cfg.Actor.X-width/2, cfg.Actor.Y-height/2)
	}
}
