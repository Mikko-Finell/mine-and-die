package server

import (
	"math"
	"time"

	effectcatalog "mine-and-die/server/effects/catalog"
	effectcontract "mine-and-die/server/effects/contract"
	internaleffects "mine-and-die/server/internal/effects"
)

type EffectManager struct {
	core  *internaleffects.Manager
	world *World
}

func newEffectManager(world *World) *EffectManager {
	definitions := effectcontract.BuiltInDefinitions()
	var resolver *effectcatalog.Resolver
	if r, err := effectcatalog.Load(effectcontract.BuiltInRegistry, effectcatalog.DefaultPaths()...); err == nil {
		if loaded := r.DefinitionsByContractID(); len(loaded) > 0 {
			for id, def := range loaded {
				if _, exists := definitions[id]; exists {
					continue
				}
				definitions[id] = def
			}
		}
		resolver = r
	}

	hooks := defaultEffectHookRegistry(world)
	manager := internaleffects.NewManager(internaleffects.ManagerConfig{
		Definitions: definitions,
		Catalog:     resolver,
		Hooks:       hooks,
		OwnerMissing: func(actorID string) bool {
			if actorID == "" || world == nil {
				return false
			}
			if _, ok := world.players[actorID]; ok {
				return false
			}
			if _, ok := world.npcs[actorID]; ok {
				return false
			}
			return true
		},
	})

	return &EffectManager{core: manager, world: world}
}

func (m *EffectManager) Definitions() map[string]*effectcontract.EffectDefinition {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.Definitions()
}

func (m *EffectManager) Hooks() map[string]internaleffects.HookSet {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.Hooks()
}

func (m *EffectManager) Instances() map[string]*effectcontract.EffectInstance {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.Instances()
}

func (m *EffectManager) Catalog() *effectcatalog.Resolver {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.Catalog()
}

func (m *EffectManager) TotalEnqueued() int {
	if m == nil || m.core == nil {
		return 0
	}
	return m.core.TotalEnqueued()
}

func (m *EffectManager) TotalDrained() int {
	if m == nil || m.core == nil {
		return 0
	}
	return m.core.TotalDrained()
}

func (m *EffectManager) LastTickProcessed() effectcontract.Tick {
	if m == nil || m.core == nil {
		return 0
	}
	return m.core.LastTickProcessed()
}

func (m *EffectManager) PendingIntentCount() int {
	if m == nil || m.core == nil {
		return 0
	}
	return m.core.PendingIntentCount()
}

func (m *EffectManager) WorldEffect(id string) *effectState {
	if m == nil || m.core == nil {
		return nil
	}
	return loadWorldEffect(m.core, id)
}

func (m *EffectManager) PendingIntents() []effectcontract.EffectIntent {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.PendingIntents()
}

func (m *EffectManager) ResetPendingIntents() {
	if m == nil || m.core == nil {
		return
	}
	m.core.ResetPendingIntents()
}

func (m *EffectManager) EnqueueIntent(intent effectcontract.EffectIntent) {
	if m == nil || m.core == nil {
		return
	}
	m.core.EnqueueIntent(intent)
}

func (m *EffectManager) RunTick(tick effectcontract.Tick, now time.Time, emit func(effectcontract.EffectLifecycleEvent)) {
	if m == nil || m.core == nil {
		return
	}
	m.core.RunTick(tick, now, emit)
}

func defaultEffectHookRegistry(world *World) map[string]internaleffects.HookSet {
	registry := make(map[string]internaleffects.HookSet)
	registry[effectcontract.HookMeleeSpawn] = internaleffects.HookSet{
		OnSpawn: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			if instance == nil || world == nil {
				return
			}
			owner, _ := world.abilityOwner(instance.OwnerActorID)
			if owner == nil {
				return
			}
			effect, area := meleeEffectForInstance(instance, owner, now)
			if effect == nil {
				return
			}
			world.resolveMeleeImpact(effect, owner, instance.OwnerActorID, uint64(tick), now, area)
		},
	}
	registry[effectcontract.HookProjectileLifecycle] = internaleffects.HookSet{
		OnSpawn: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			if instance == nil || world == nil {
				return
			}
			if existing := loadWorldEffect(rt, instance.ID); existing != nil {
				owner, _ := world.abilityOwner(instance.OwnerActorID)
				syncProjectileInstance(instance, owner, existing)
				return
			}
			tpl := world.projectileTemplates[instance.DefinitionID]
			if tpl == nil {
				return
			}
			owner, _ := world.abilityOwner(instance.OwnerActorID)
			if owner == nil {
				return
			}
			if effect := world.spawnContractProjectileFromInstance(instance, owner, tpl, now); effect != nil {
				storeWorldEffect(rt, instance.ID, effect)
				syncProjectileInstance(instance, owner, effect)
			}
		},
		OnTick: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			if instance == nil || world == nil {
				return
			}
			effect := loadWorldEffect(rt, instance.ID)
			if effect == nil {
				effect = world.findEffectByID(instance.ID)
				if effect != nil {
					storeWorldEffect(rt, instance.ID, effect)
				}
			}
			tpl := world.projectileTemplates[instance.DefinitionID]
			owner, _ := world.abilityOwner(instance.OwnerActorID)
			if effect == nil {
				if tpl == nil || owner == nil {
					return
				}
				effect = world.spawnContractProjectileFromInstance(instance, owner, tpl, now)
				if effect == nil {
					return
				}
				storeWorldEffect(rt, instance.ID, effect)
			}
			dt := 1.0 / float64(tickRate)
			ended := world.advanceProjectile(effect, now, dt)
			syncProjectileInstance(instance, owner, effect)
			if ended {
				instance.BehaviorState.TicksRemaining = 0
				storeWorldEffect(rt, instance.ID, nil)
			}
		},
	}
	registry[effectcontract.HookStatusBurningVisual] = internaleffects.HookSet{
		OnSpawn: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			if instance == nil || world == nil {
				return
			}
			targetID := instance.FollowActorID
			if targetID == "" {
				targetID = instance.DeliveryState.AttachedActorID
			}
			if targetID == "" {
				return
			}
			actor := world.actorByID(targetID)
			if actor == nil {
				return
			}
			effect := loadWorldEffect(rt, instance.ID)
			if effect == nil {
				lifetime := ticksToDuration(instance.BehaviorState.TicksRemaining)
				if lifetime <= 0 {
					lifetime = burningStatusEffectDuration
				}
				effect = spawnStatusVisualFromInstance(world, instance, actor, lifetime, now)
				if effect == nil {
					return
				}
				attachVisualToStatusEffect(actor, effect)
				storeWorldEffect(rt, instance.ID, effect)
			}
			syncStatusVisualInstance(instance, actor, effect)
		},
		OnTick: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			if instance == nil || world == nil {
				return
			}
			targetID := instance.FollowActorID
			if targetID == "" {
				targetID = instance.DeliveryState.AttachedActorID
			}
			actor := world.actorByID(targetID)
			effect := loadWorldEffect(rt, instance.ID)
			if effect == nil {
				effect = world.findEffectByID(instance.ID)
				if effect != nil {
					storeWorldEffect(rt, instance.ID, effect)
				}
			}
			if effect == nil && actor != nil {
				lifetime := ticksToDuration(instance.BehaviorState.TicksRemaining)
				if lifetime <= 0 {
					lifetime = burningStatusEffectDuration
				}
				effect = spawnStatusVisualFromInstance(world, instance, actor, lifetime, now)
				if effect != nil {
					attachVisualToStatusEffect(actor, effect)
					storeWorldEffect(rt, instance.ID, effect)
				}
			}
			if effect == nil {
				return
			}
			syncStatusVisualInstance(instance, actor, effect)
			if actor != nil && actor.statusEffects != nil {
				if inst, ok := actor.statusEffects[StatusEffectBurning]; ok && inst != nil {
					remaining := inst.ExpiresAt.Sub(now)
					if remaining < 0 {
						remaining = 0
					}
					ticksRemaining := durationToTicks(remaining)
					if remaining > 0 && ticksRemaining < 1 {
						ticksRemaining = 1
					}
					instance.BehaviorState.TicksRemaining = ticksRemaining
				}
			}
		},
	}
	registry[effectcontract.HookStatusBurningDamage] = internaleffects.HookSet{
		OnSpawn: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			if instance == nil || world == nil {
				return
			}
			targetID := instance.FollowActorID
			if targetID == "" {
				targetID = instance.DeliveryState.AttachedActorID
			}
			if targetID == "" {
				return
			}
			actor := world.actorByID(targetID)
			if actor == nil {
				return
			}
			delta := 0.0
			if instance.BehaviorState.Extra != nil {
				if value, ok := instance.BehaviorState.Extra["healthDelta"]; ok {
					delta = float64(value)
				}
			}
			if delta == 0 {
				delta = -lavaDamagePerSecond * burningTickInterval.Seconds()
			}
			statusType := StatusEffectBurning
			if actor.statusEffects != nil {
				if inst, ok := actor.statusEffects[StatusEffectBurning]; ok && inst != nil && inst.Definition != nil {
					statusType = inst.Definition.Type
				}
			}
			world.applyBurningDamage(instance.OwnerActorID, actor, statusType, delta, now)
		},
	}
	registry[effectcontract.HookVisualBloodSplatter] = internaleffects.HookSet{
		OnSpawn: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			ensureBloodDecalInstance(rt, world, instance, now)
		},
		OnTick: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			ensureBloodDecalInstance(rt, world, instance, now)
		},
	}
	return registry
}

func meleeEffectForInstance(instance *effectcontract.EffectInstance, owner *actorState, now time.Time) (*effectState, Obstacle) {
	if instance == nil || owner == nil {
		return nil, Obstacle{}
	}
	geom := instance.DeliveryState.Geometry
	width := dequantizeWorldCoord(geom.Width)
	height := dequantizeWorldCoord(geom.Height)
	if width <= 0 {
		width = meleeAttackWidth
	}
	if height <= 0 {
		height = meleeAttackReach
	}
	offsetX := dequantizeWorldCoord(geom.OffsetX)
	offsetY := dequantizeWorldCoord(geom.OffsetY)
	centerX := owner.X + offsetX
	centerY := owner.Y + offsetY
	rectX := centerX - width/2
	rectY := centerY - height/2
	motion := instance.DeliveryState.Motion
	motion.PositionX = quantizeWorldCoord(centerX)
	motion.PositionY = quantizeWorldCoord(centerY)
	motion.VelocityX = 0
	motion.VelocityY = 0
	instance.DeliveryState.Motion = motion
	params := intMapToFloat64(instance.BehaviorState.Extra)
	if params == nil {
		params = make(map[string]float64)
	}
	if _, ok := params["healthDelta"]; !ok {
		params["healthDelta"] = -meleeAttackDamage
	}
	if _, ok := params["reach"]; !ok {
		params["reach"] = meleeAttackReach
	}
	if _, ok := params["width"]; !ok {
		params["width"] = meleeAttackWidth
	}
	effect := &effectState{
		ID:                 instance.ID,
		Type:               instance.DefinitionID,
		Owner:              instance.OwnerActorID,
		Start:              now.UnixMilli(),
		Duration:           meleeAttackDuration.Milliseconds(),
		X:                  rectX,
		Y:                  rectY,
		Width:              width,
		Height:             height,
		Params:             params,
		Instance:           *instance,
		TelemetrySpawnTick: instance.StartTick,
	}
	area := Obstacle{X: rectX, Y: rectY, Width: width, Height: height}
	return effect, area
}

func syncProjectileInstance(instance *effectcontract.EffectInstance, owner *actorState, effect *effectState) {
	if instance == nil || effect == nil {
		return
	}
	if instance.BehaviorState.Extra == nil {
		instance.BehaviorState.Extra = make(map[string]int)
	}
	if instance.Params == nil {
		instance.Params = make(map[string]int)
	}
	if effect.Projectile != nil {
		instance.BehaviorState.Extra["remainingRange"] = int(math.Round(effect.Projectile.RemainingRange))
		instance.Params["remainingRange"] = instance.BehaviorState.Extra["remainingRange"]
		instance.BehaviorState.Extra["dx"] = QuantizeCoord(effect.Projectile.VelocityUnitX)
		instance.Params["dx"] = instance.BehaviorState.Extra["dx"]
		instance.BehaviorState.Extra["dy"] = QuantizeCoord(effect.Projectile.VelocityUnitY)
		instance.Params["dy"] = instance.BehaviorState.Extra["dy"]
		if tpl := effect.Projectile.Template; tpl != nil && tpl.MaxDistance > 0 {
			instance.BehaviorState.Extra["range"] = int(math.Round(tpl.MaxDistance))
			instance.Params["range"] = instance.BehaviorState.Extra["range"]
		}
	}
	geometry := instance.DeliveryState.Geometry
	if effect.Width > 0 {
		geometry.Width = quantizeWorldCoord(effect.Width)
	}
	if effect.Height > 0 {
		geometry.Height = quantizeWorldCoord(effect.Height)
	}
	radius := effect.Params["radius"]
	if radius <= 0 {
		radius = effect.Width / 2
	}
	geometry.Radius = quantizeWorldCoord(radius)
	if owner != nil {
		geometry.OffsetX = quantizeWorldCoord(centerX(effect) - owner.X)
		geometry.OffsetY = quantizeWorldCoord(centerY(effect) - owner.Y)
	}
	motion := instance.DeliveryState.Motion
	motion.PositionX = quantizeWorldCoord(centerX(effect))
	motion.PositionY = quantizeWorldCoord(centerY(effect))
	if effect.Projectile != nil {
		speed := 0.0
		if tpl := effect.Projectile.Template; tpl != nil {
			speed = tpl.Speed
		}
		motion.VelocityX = QuantizeVelocity(effect.Projectile.VelocityUnitX * speed)
		motion.VelocityY = QuantizeVelocity(effect.Projectile.VelocityUnitY * speed)
	}
	instance.DeliveryState.Motion = motion
	instance.DeliveryState.Geometry = geometry
}

func syncStatusVisualInstance(instance *effectcontract.EffectInstance, actor *actorState, effect *effectState) {
	if instance == nil || effect == nil {
		return
	}
	geometry := instance.DeliveryState.Geometry
	width := effect.Width
	if width <= 0 {
		width = playerHalf * 2
	}
	height := effect.Height
	if height <= 0 {
		height = playerHalf * 2
	}
	geometry.Width = quantizeWorldCoord(width)
	geometry.Height = quantizeWorldCoord(height)
	if actor != nil {
		geometry.OffsetX = quantizeWorldCoord(centerX(effect) - actor.X)
		geometry.OffsetY = quantizeWorldCoord(centerY(effect) - actor.Y)
	}
	centerXVal := centerX(effect)
	centerYVal := centerY(effect)
	if actor != nil {
		centerXVal = actor.X
		centerYVal = actor.Y
	}
	motion := instance.DeliveryState.Motion
	motion.PositionX = quantizeWorldCoord(centerXVal)
	motion.PositionY = quantizeWorldCoord(centerYVal)
	motion.VelocityX = 0
	motion.VelocityY = 0
	instance.DeliveryState.Motion = motion
	instance.DeliveryState.Geometry = geometry
}

func spawnStatusVisualFromInstance(world *World, instance *effectcontract.EffectInstance, actor *actorState, lifetime time.Duration, now time.Time) *effectState {
	if world == nil || instance == nil || actor == nil {
		return nil
	}
	if lifetime <= 0 {
		lifetime = burningTickInterval
	}
	effect := world.attachStatusEffectVisual(actor, instance.DefinitionID, lifetime, now)
	if effect == nil {
		return nil
	}
	effect.ContractManaged = true
	effect.ID = instance.ID
	effect.Instance = *instance
	effect.StatusEffect = StatusEffectBurning
	effect.FollowActorID = actor.ID
	effect.TelemetrySpawnTick = instance.StartTick
	return effect
}

func attachVisualToStatusEffect(actor *actorState, effect *effectState) {
	if actor == nil || effect == nil {
		return
	}
	if actor.statusEffects == nil {
		return
	}
	inst, ok := actor.statusEffects[StatusEffectBurning]
	if !ok || inst == nil {
		return
	}
	inst.attachedEffect = effect
	if inst.Definition != nil {
		effect.StatusEffect = inst.Definition.Type
	} else {
		effect.StatusEffect = StatusEffectBurning
	}
}

func ensureBloodDecalInstance(rt internaleffects.Runtime, world *World, instance *effectcontract.EffectInstance, now time.Time) *effectState {
	if instance == nil || world == nil {
		return nil
	}
	effect := loadWorldEffect(rt, instance.ID)
	if effect == nil {
		effect = world.findEffectByID(instance.ID)
		if effect != nil {
			storeWorldEffect(rt, instance.ID, effect)
		}
	}
	if effect == nil {
		effect = world.spawnContractBloodDecalFromInstance(instance, now)
		if effect == nil {
			return nil
		}
		storeWorldEffect(rt, instance.ID, effect)
	}
	syncBloodDecalInstance(instance, effect)
	return effect
}

func syncBloodDecalInstance(instance *effectcontract.EffectInstance, effect *effectState) {
	if instance == nil || effect == nil {
		return
	}
	geometry := instance.DeliveryState.Geometry
	width := effect.Width
	if width <= 0 {
		width = playerHalf * 2
	}
	height := effect.Height
	if height <= 0 {
		height = playerHalf * 2
	}
	geometry.Width = quantizeWorldCoord(width)
	geometry.Height = quantizeWorldCoord(height)
	instance.DeliveryState.Geometry = geometry
	if instance.BehaviorState.Extra == nil {
		instance.BehaviorState.Extra = make(map[string]int)
	}
	if instance.Params == nil {
		instance.Params = make(map[string]int)
	}
	centerX := quantizeWorldCoord(centerX(effect))
	centerY := quantizeWorldCoord(centerY(effect))
	instance.BehaviorState.Extra["centerX"] = centerX
	instance.BehaviorState.Extra["centerY"] = centerY
	instance.Params["centerX"] = centerX
	instance.Params["centerY"] = centerY
	instance.Colors = bloodSplatterColors()
}

func storeWorldEffect(rt internaleffects.Runtime, id string, effect *effectState) {
	if rt == nil || id == "" {
		return
	}
	if effect == nil {
		rt.ClearInstanceState(id)
		return
	}
	rt.SetInstanceState(id, effect)
}

func loadWorldEffect(rt internaleffects.Runtime, id string) *effectState {
	if rt == nil || id == "" {
		return nil
	}
	if value := rt.InstanceState(id); value != nil {
		if effect, ok := value.(*effectState); ok {
			return effect
		}
	}
	return nil
}

func ticksToDuration(ticks int) time.Duration {
	if ticks <= 0 {
		return 0
	}
	seconds := float64(ticks) / float64(tickRate)
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func intMapToFloat64(src map[string]int) map[string]float64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = float64(v)
	}
	return dst
}

func dequantizeWorldCoord(value int) float64 {
	if value == 0 {
		return 0
	}
	return DequantizeCoord(value) * tileSize
}
