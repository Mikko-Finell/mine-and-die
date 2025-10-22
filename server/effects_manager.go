package server

import (
	"math"
	"time"

	effectcatalog "mine-and-die/server/effects/catalog"
	effectcontract "mine-and-die/server/effects/contract"
	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
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

	var registryProvider func() internaleffects.Registry
	if world != nil {
		registryProvider = world.effectRegistry
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
		Registry: registryProvider,
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
	return internaleffects.LoadRuntimeEffect(m.core, id)
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

type projectileOwnerAdapter struct {
	state *actorState
}

func (a projectileOwnerAdapter) Facing() string {
	if a.state == nil || a.state.Facing == "" {
		return string(defaultFacing)
	}
	return string(a.state.Facing)
}

func (a projectileOwnerAdapter) FacingVector() (float64, float64) {
	return facingToVector(FacingDirection(a.Facing()))
}

func (a projectileOwnerAdapter) Position() (float64, float64) {
	if a.state == nil {
		return 0, 0
	}
	return a.state.X, a.state.Y
}

func defaultEffectHookRegistry(world *World) map[string]internaleffects.HookSet {
	hooks := make(map[string]internaleffects.HookSet)
	hooks[effectcontract.HookMeleeSpawn] = internaleffects.HookSet{
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
	hooks[effectcontract.HookProjectileLifecycle] = internaleffects.HookSet{
		OnSpawn: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			if instance == nil || world == nil {
				return
			}
			if existing := internaleffects.LoadRuntimeEffect(rt, instance.ID); existing != nil {
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
			world.pruneEffects(now)
			effect := internaleffects.SpawnContractProjectileFromInstance(internaleffects.ProjectileSpawnConfig{
				Instance: instance,
				Owner:    projectileOwnerAdapter{state: owner},
				Template: tpl,
				Now:      now,
				TileSize: tileSize,
				TickRate: tickRate,
			})
			if effect == nil {
				return
			}
			if !internaleffects.RegisterRuntimeEffect(rt, effect) {
				instance.BehaviorState.TicksRemaining = 0
				return
			}
			world.recordEffectSpawn(tpl.Type, "projectile")
			internaleffects.StoreRuntimeEffect(rt, instance.ID, effect)
			syncProjectileInstance(instance, owner, effect)
		},
		OnTick: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			if instance == nil || world == nil {
				return
			}
			effect := internaleffects.LoadRuntimeEffect(rt, instance.ID)
			tpl := world.projectileTemplates[instance.DefinitionID]
			owner, _ := world.abilityOwner(instance.OwnerActorID)
			if effect == nil {
				if tpl == nil || owner == nil {
					return
				}
				world.pruneEffects(now)
				effect = internaleffects.SpawnContractProjectileFromInstance(internaleffects.ProjectileSpawnConfig{
					Instance: instance,
					Owner:    projectileOwnerAdapter{state: owner},
					Template: tpl,
					Now:      now,
					TileSize: tileSize,
					TickRate: tickRate,
				})
				if effect == nil {
					return
				}
				if !internaleffects.RegisterRuntimeEffect(rt, effect) {
					instance.BehaviorState.TicksRemaining = 0
					return
				}
				world.recordEffectSpawn(tpl.Type, "projectile")
				internaleffects.StoreRuntimeEffect(rt, instance.ID, effect)
			}
			dt := 1.0 / float64(tickRate)
			ended := world.advanceProjectile(effect, now, dt)
			syncProjectileInstance(instance, owner, effect)
			if ended {
				instance.BehaviorState.TicksRemaining = 0
				internaleffects.UnregisterRuntimeEffect(rt, effect)
				internaleffects.StoreRuntimeEffect(rt, instance.ID, nil)
			}
		},
	}
	lookupContractActor := func(actorID string) *internaleffects.ContractStatusActor {
		if world == nil || actorID == "" {
			return nil
		}
		actor := world.actorByID(actorID)
		if actor == nil {
			return nil
		}
		contractActor := &internaleffects.ContractStatusActor{
			ID: actor.ID,
			X:  actor.X,
			Y:  actor.Y,
			ApplyBurningDamage: func(ownerID string, status internaleffects.StatusEffectType, delta float64, now time.Time) {
				world.applyBurningDamage(ownerID, actor, StatusEffectType(status), delta, now)
			},
		}
		if actor.statusEffects != nil {
			if inst := actor.statusEffects[StatusEffectBurning]; inst != nil {
				contractActor.StatusInstance = &internaleffects.ContractStatusInstance{
					Instance:  inst,
					ExpiresAt: func() time.Time { return inst.ExpiresAt },
				}
			}
		}
		return contractActor
	}
	hooks[effectcontract.HookStatusBurningVisual] = internaleffects.ContractBurningVisualHook(internaleffects.ContractBurningVisualHookConfig{
		StatusEffect:     internaleffects.StatusEffectType(StatusEffectBurning),
		DefaultLifetime:  burningStatusEffectDuration,
		FallbackLifetime: burningTickInterval,
		TileSize:         tileSize,
		DefaultFootprint: playerHalf * 2,
		TickRate:         tickRate,
		LookupActor:      lookupContractActor,
		ExtendLifetime: func(fields worldpkg.StatusEffectLifetimeFields, expiresAt time.Time) {
			worldpkg.ExtendStatusEffectLifetime(fields, expiresAt)
		},
		ExpireLifetime: func(fields worldpkg.StatusEffectLifetimeFields, now time.Time) {
			worldpkg.ExpireStatusEffectLifetime(fields, now)
		},
		RecordEffectSpawn: func(effectType, category string) {
			if world == nil {
				return
			}
			world.recordEffectSpawn(effectType, category)
		},
	})
	hooks[effectcontract.HookStatusBurningDamage] = internaleffects.ContractBurningDamageHook(internaleffects.ContractBurningDamageHookConfig{
		StatusEffect:    internaleffects.StatusEffectType(StatusEffectBurning),
		DamagePerSecond: lavaDamagePerSecond,
		TickInterval:    burningTickInterval,
		LookupActor:     lookupContractActor,
	})
	ensureBloodDecal := func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, _ effectcontract.Tick, now time.Time) {
		internaleffects.EnsureBloodDecalInstance(internaleffects.BloodDecalInstanceConfig{
			Runtime:         rt,
			Instance:        instance,
			Now:             now,
			TileSize:        tileSize,
			TickRate:        tickRate,
			DefaultSize:     playerHalf * 2,
			DefaultDuration: bloodSplatterDuration,
			Params:          newBloodSplatterParams(),
			Colors:          bloodSplatterColors(),
			PruneExpired: func(at time.Time) {
				if world == nil {
					return
				}
				world.pruneEffects(at)
			},
			RecordSpawn: func(effectType, producer string) {
				if world == nil {
					return
				}
				world.recordEffectSpawn(effectType, producer)
			},
		})
	}
	hooks[effectcontract.HookVisualBloodSplatter] = internaleffects.HookSet{
		OnSpawn: ensureBloodDecal,
		OnTick:  ensureBloodDecal,
	}
	return hooks
}

func meleeEffectForInstance(instance *effectcontract.EffectInstance, owner *actorState, now time.Time) (*effectState, Obstacle) {
	if instance == nil || owner == nil {
		return nil, Obstacle{}
	}
	geom := instance.DeliveryState.Geometry
	width := internaleffects.DequantizeWorldCoord(geom.Width, tileSize)
	height := internaleffects.DequantizeWorldCoord(geom.Height, tileSize)
	if width <= 0 {
		width = meleeAttackWidth
	}
	if height <= 0 {
		height = meleeAttackReach
	}
	offsetX := internaleffects.DequantizeWorldCoord(geom.OffsetX, tileSize)
	offsetY := internaleffects.DequantizeWorldCoord(geom.OffsetY, tileSize)
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
	params := internaleffects.IntMapToFloat64(instance.BehaviorState.Extra)
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
