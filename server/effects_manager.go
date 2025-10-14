package main

import (
	"fmt"
	"math"
	"time"
)

type endDecision struct {
	shouldEnd bool
	reason    EndReason
}

// EffectManager owns the contract-driven effect pipeline. The current
// implementation translates queued intents into minimal EffectInstance records
// and emits the contract transport events so downstream plumbing can observe
// spawn/update/end ordering while the legacy gameplay systems remain
// authoritative.
//
// The manager intentionally lives behind the enableContractEffectManager flag
// so it can collect intents and metrics without altering live gameplay until
// dual-write plumbing is ready. While wiring lands, totalEnqueued and
// totalDrained provide a temporary sanity check that tick execution drains all
// staged intents; expect these counters to be removed or repurposed once
// spawning transitions fully into the manager.
type EffectManager struct {
	intentQueue       []EffectIntent
	instances         map[string]*EffectInstance
	definitions       map[string]*EffectDefinition
	seqByInstance     map[string]Seq
	hooks             map[string]effectHookSet
	worldEffects      map[string]*effectState
	totalEnqueued     int
	totalDrained      int
	lastTickProcessed Tick
	nextInstanceID    uint64
	world             *World
}

type effectHookFunc func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time)

type effectHookSet struct {
	OnSpawn effectHookFunc
	OnTick  effectHookFunc
}

const (
	meleeSpawnHookID          = "melee.spawn"
	projectileLifecycleHookID = "projectile.fireball.lifecycle"
	statusBurningVisualHookID = "status.burning.visual"
	statusBurningDamageHookID = "status.burning.tick"
	visualBloodSplatterHookID = "visual.blood.splatter"
)

func newEffectManager(world *World) *EffectManager {
	return &EffectManager{
		intentQueue:   make([]EffectIntent, 0),
		instances:     make(map[string]*EffectInstance),
		definitions:   defaultEffectDefinitions(),
		hooks:         defaultEffectHookRegistry(world),
		worldEffects:  make(map[string]*effectState),
		seqByInstance: make(map[string]Seq),
		world:         world,
	}
}

// EnqueueIntent stages an EffectIntent for future processing. The skeleton
// version simply records the request for observability while legacy systems
// remain authoritative.
func (m *EffectManager) EnqueueIntent(intent EffectIntent) {
	if m == nil {
		return
	}
	m.intentQueue = append(m.intentQueue, intent)
	m.totalEnqueued++
}

// RunTick advances the manager by one simulation tick. It drains the queued
// intents, instantiates minimal effect records, and emits contract events for
// observers while still letting the legacy systems drive gameplay state.
func (m *EffectManager) RunTick(tick Tick, now time.Time, emit func(EffectLifecycleEvent)) {
	if m == nil {
		return
	}
	if !enableContractEffectManager {
		m.intentQueue = m.intentQueue[:0]
		return
	}
	m.lastTickProcessed = tick

	drainedQueue := append([]EffectIntent(nil), m.intentQueue...)
	if len(m.intentQueue) > 0 {
		for i := range m.intentQueue {
			m.intentQueue[i] = EffectIntent{}
		}
		m.intentQueue = m.intentQueue[:0]
	}

	drained := len(drainedQueue)
	newInstances := make([]*EffectInstance, 0, drained)
	if drained > 0 {
		for _, intent := range drainedQueue {
			if intent.TypeID == "" {
				continue
			}
			instance := m.instantiateIntent(intent, tick)
			if instance == nil {
				continue
			}
			m.instances[instance.ID] = instance
			m.seqByInstance[instance.ID] = 0
			newInstances = append(newInstances, instance)
			m.invokeOnSpawn(instance, tick, now)
		}
	}
	if drained > 0 {
		m.totalDrained += drained
	}
	if emit == nil {
		return
	}
	for _, instance := range newInstances {
		if instance == nil {
			continue
		}
		if !instance.Replication.SendSpawn {
			continue
		}
		spawn := EffectSpawnEvent{
			Tick:     tick,
			Seq:      m.nextSequenceFor(instance.ID),
			Instance: m.cloneInstanceForSpawn(instance),
		}
		emit(spawn)
	}
	ended := make([]string, 0)
	for _, instance := range m.instances {
		if instance == nil {
			continue
		}
		m.invokeOnTick(instance, tick, now)
		if !instance.Replication.SendUpdates {
			// Even when updates are suppressed, lifecycle evaluation still runs.
		} else {
			delivery := m.cloneDeliveryState(instance.DeliveryState)
			behavior := m.cloneBehaviorState(instance.BehaviorState)
			update := EffectUpdateEvent{
				Tick:          tick,
				Seq:           m.nextSequenceFor(instance.ID),
				ID:            instance.ID,
				DeliveryState: &delivery,
				BehaviorState: &behavior,
			}
			emit(update)
		}
		decision := m.evaluateEndPolicy(instance, tick)
		if decision.shouldEnd {
			if instance.Replication.SendEnd {
				end := EffectEndEvent{
					Tick:   tick,
					Seq:    m.nextSequenceFor(instance.ID),
					ID:     instance.ID,
					Reason: decision.reason,
				}
				emit(end)
			}
			ended = append(ended, instance.ID)
		}
	}
	for _, id := range ended {
		delete(m.instances, id)
		delete(m.seqByInstance, id)
		delete(m.worldEffects, id)
	}
}

func (m *EffectManager) instantiateIntent(intent EffectIntent, tick Tick) *EffectInstance {
	m.nextInstanceID++
	id := fmt.Sprintf("contract-effect-%d", m.nextInstanceID)
	geometry := intent.Geometry
	if geometry.Variants != nil {
		geometry.Variants = copyIntMap(geometry.Variants)
	}
	definition := m.definitionFor(intent.TypeID)
	replication := ReplicationSpec{SendSpawn: true, SendUpdates: true, SendEnd: true}
	endPolicy := EndPolicy{Kind: EndDuration}
	if definition != nil {
		replication = definition.Client
		endPolicy = definition.End
	}
	deliveryKind := intent.Delivery
	if deliveryKind == "" && definition != nil {
		deliveryKind = definition.Delivery
	}
	if deliveryKind == "" {
		deliveryKind = DeliveryKindArea
	}
	follow := FollowNone
	if deliveryKind == DeliveryKindTarget {
		follow = FollowTarget
	}
	ticksRemaining := intent.DurationTicks
	if ticksRemaining <= 0 && definition != nil && endPolicy.Kind == EndDuration {
		ticksRemaining = definition.LifetimeTicks
	}
	params := copyIntMap(intent.Params)
	extra := copyIntMap(intent.Params)
	instance := &EffectInstance{
		ID:           id,
		DefinitionID: intent.TypeID,
		Definition:   definition,
		StartTick:    tick,
		DeliveryState: EffectDeliveryState{
			Geometry:        geometry,
			Follow:          follow,
			AttachedActorID: intent.TargetActorID,
		},
		BehaviorState: EffectBehaviorState{
			TicksRemaining: ticksRemaining,
			Extra:          extra,
		},
		Params:        params,
		FollowActorID: intent.TargetActorID,
		OwnerActorID:  intent.SourceActorID,
		Replication:   replication,
		End:           endPolicy,
	}
	return instance
}

func (m *EffectManager) nextSequenceFor(id string) Seq {
	if id == "" {
		return 0
	}
	next := m.seqByInstance[id] + 1
	m.seqByInstance[id] = next
	return next
}

func (m *EffectManager) cloneInstanceForSpawn(instance *EffectInstance) EffectInstance {
	if instance == nil {
		return EffectInstance{}
	}
	clone := *instance
	clone.DeliveryState.Geometry = cloneGeometry(clone.DeliveryState.Geometry)
	clone.BehaviorState.Extra = copyIntMap(clone.BehaviorState.Extra)
	clone.BehaviorState.Stacks = copyIntMap(clone.BehaviorState.Stacks)
	clone.Params = copyIntMap(clone.Params)
	if len(clone.Colors) > 0 {
		clone.Colors = append([]string(nil), clone.Colors...)
	}
	clone.Replication.UpdateFields = copyBoolMap(clone.Replication.UpdateFields)
	return clone
}

func (m *EffectManager) cloneDeliveryState(state EffectDeliveryState) EffectDeliveryState {
	clone := state
	clone.Geometry = cloneGeometry(state.Geometry)
	return clone
}

func (m *EffectManager) cloneBehaviorState(state EffectBehaviorState) EffectBehaviorState {
	clone := state
	clone.Extra = copyIntMap(state.Extra)
	clone.Stacks = copyIntMap(state.Stacks)
	return clone
}

func (m *EffectManager) invokeOnSpawn(instance *EffectInstance, tick Tick, now time.Time) {
	if instance == nil || m == nil || m.hooks == nil {
		return
	}
	def := instance.Definition
	if def == nil || def.Hooks.OnSpawn == "" {
		return
	}
	hook, ok := m.hooks[def.Hooks.OnSpawn]
	if !ok || hook.OnSpawn == nil {
		return
	}
	hook.OnSpawn(m, instance, tick, now)
}

func (m *EffectManager) invokeOnTick(instance *EffectInstance, tick Tick, now time.Time) {
	if instance == nil || m == nil || m.hooks == nil {
		return
	}
	def := instance.Definition
	if def == nil || def.Hooks.OnTick == "" {
		return
	}
	hook, ok := m.hooks[def.Hooks.OnTick]
	if !ok || hook.OnTick == nil {
		return
	}
	hook.OnTick(m, instance, tick, now)
}

func (m *EffectManager) definitionFor(typeID string) *EffectDefinition {
	if typeID == "" {
		return nil
	}
	if def, ok := m.definitions[typeID]; ok {
		return def
	}
	return nil
}

func (m *EffectManager) evaluateEndPolicy(instance *EffectInstance, tick Tick) endDecision {
	if instance == nil {
		return endDecision{}
	}
	switch instance.End.Kind {
	case EndInstant:
		if tick >= instance.StartTick {
			return endDecision{shouldEnd: true, reason: EndReasonExpired}
		}
	case EndDuration:
		if instance.BehaviorState.TicksRemaining > 0 {
			instance.BehaviorState.TicksRemaining--
		}
		if instance.BehaviorState.TicksRemaining <= 0 {
			return endDecision{shouldEnd: true, reason: EndReasonExpired}
		}
	case EndCondition:
		if instance.End.Conditions.OnOwnerLost && m.ownerMissing(instance.OwnerActorID) {
			return endDecision{shouldEnd: true, reason: EndReasonOwnerLost}
		}
	}
	return endDecision{}
}

func (m *EffectManager) ownerMissing(actorID string) bool {
	if actorID == "" || m == nil || m.world == nil {
		return false
	}
	if _, ok := m.world.players[actorID]; ok {
		return false
	}
	if _, ok := m.world.npcs[actorID]; ok {
		return false
	}
	return true
}

func cloneGeometry(src EffectGeometry) EffectGeometry {
	dst := src
	if src.Variants != nil {
		dst.Variants = copyIntMap(src.Variants)
	}
	return dst
}

func copyIntMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]int, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyBoolMap(src map[string]bool) map[string]bool {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
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

func defaultEffectHookRegistry(world *World) map[string]effectHookSet {
	registry := make(map[string]effectHookSet)
	registry[meleeSpawnHookID] = effectHookSet{
		OnSpawn: func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time) {
			if !enableContractEffectManager || !enableContractMeleeDefinitions {
				return
			}
			if m == nil || instance == nil || m.world == nil {
				return
			}
			owner, _ := m.world.abilityOwner(instance.OwnerActorID)
			if owner == nil {
				return
			}
			effect, area := m.meleeEffectForInstance(instance, owner, now)
			if effect == nil {
				return
			}
			m.world.resolveMeleeImpact(effect, owner, instance.OwnerActorID, uint64(tick), now, area)
		},
	}
	registry[projectileLifecycleHookID] = effectHookSet{
		OnSpawn: func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time) {
			if !enableContractEffectManager || !enableContractProjectileDefinitions {
				return
			}
			if m == nil || instance == nil || m.world == nil {
				return
			}
			if existing := m.worldEffects[instance.ID]; existing != nil {
				owner, _ := m.world.abilityOwner(instance.OwnerActorID)
				m.syncProjectileInstance(instance, owner, existing)
				return
			}
			tpl := m.world.projectileTemplates[instance.DefinitionID]
			if tpl == nil {
				return
			}
			owner, _ := m.world.abilityOwner(instance.OwnerActorID)
			if owner == nil {
				return
			}
			if effect := m.world.spawnContractProjectileFromInstance(instance, owner, tpl, now); effect != nil {
				m.worldEffects[instance.ID] = effect
				m.syncProjectileInstance(instance, owner, effect)
			}
		},
		OnTick: func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time) {
			if !enableContractEffectManager || !enableContractProjectileDefinitions {
				return
			}
			if m == nil || instance == nil || m.world == nil {
				return
			}
			effect := m.worldEffects[instance.ID]
			if effect == nil {
				effect = m.world.findEffectByID(instance.ID)
				if effect != nil {
					m.worldEffects[instance.ID] = effect
				}
			}
			tpl := m.world.projectileTemplates[instance.DefinitionID]
			owner, _ := m.world.abilityOwner(instance.OwnerActorID)
			if effect == nil {
				if tpl == nil || owner == nil {
					return
				}
				effect = m.world.spawnContractProjectileFromInstance(instance, owner, tpl, now)
				if effect == nil {
					return
				}
				m.worldEffects[instance.ID] = effect
			}
			dt := 1.0 / float64(tickRate)
			ended := m.world.advanceProjectile(effect, now, dt)
			m.syncProjectileInstance(instance, owner, effect)
			if ended {
				instance.BehaviorState.TicksRemaining = 0
				delete(m.worldEffects, instance.ID)
			}
		},
	}
	registry[statusBurningVisualHookID] = effectHookSet{
		OnSpawn: func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time) {
			if !enableContractEffectManager || !enableContractBurningDefinitions {
				return
			}
			if m == nil || instance == nil || m.world == nil {
				return
			}
			targetID := instance.FollowActorID
			if targetID == "" {
				targetID = instance.DeliveryState.AttachedActorID
			}
			if targetID == "" {
				return
			}
			actor := m.world.actorByID(targetID)
			if actor == nil {
				return
			}
			effect := m.worldEffects[instance.ID]
			if effect == nil {
				lifetime := ticksToDuration(instance.BehaviorState.TicksRemaining)
				if lifetime <= 0 {
					lifetime = burningStatusEffectDuration
				}
				effect = m.spawnStatusVisualFromInstance(instance, actor, lifetime, now)
				if effect == nil {
					return
				}
				m.attachVisualToStatusEffect(actor, effect)
			}
			m.syncStatusVisualInstance(instance, actor, effect)
		},
		OnTick: func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time) {
			if !enableContractEffectManager || !enableContractBurningDefinitions {
				return
			}
			if m == nil || instance == nil || m.world == nil {
				return
			}
			targetID := instance.FollowActorID
			if targetID == "" {
				targetID = instance.DeliveryState.AttachedActorID
			}
			actor := m.world.actorByID(targetID)
			effect := m.worldEffects[instance.ID]
			if effect == nil {
				effect = m.world.findEffectByID(instance.ID)
				if effect != nil {
					m.worldEffects[instance.ID] = effect
				}
			}
			if effect == nil && actor != nil {
				lifetime := ticksToDuration(instance.BehaviorState.TicksRemaining)
				if lifetime <= 0 {
					lifetime = burningStatusEffectDuration
				}
				effect = m.spawnStatusVisualFromInstance(instance, actor, lifetime, now)
				if effect != nil {
					m.attachVisualToStatusEffect(actor, effect)
				}
			}
			if effect == nil {
				return
			}
			m.syncStatusVisualInstance(instance, actor, effect)
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
	registry[statusBurningDamageHookID] = effectHookSet{
		OnSpawn: func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time) {
			if !enableContractEffectManager || !enableContractBurningDefinitions {
				return
			}
			if m == nil || instance == nil || m.world == nil {
				return
			}
			targetID := instance.FollowActorID
			if targetID == "" {
				targetID = instance.DeliveryState.AttachedActorID
			}
			if targetID == "" {
				return
			}
			actor := m.world.actorByID(targetID)
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
			m.world.applyBurningDamage(instance.OwnerActorID, actor, statusType, delta, now, telemetrySourceContract)
		},
	}
	registry[visualBloodSplatterHookID] = effectHookSet{
		OnSpawn: func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time) {
			if !enableContractEffectManager || !enableContractBloodDecalDefinitions {
				return
			}
			m.ensureBloodDecalInstance(instance, now)
		},
		OnTick: func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time) {
			if !enableContractEffectManager || !enableContractBloodDecalDefinitions {
				return
			}
			m.ensureBloodDecalInstance(instance, now)
		},
	}
	return registry
}

func (m *EffectManager) meleeEffectForInstance(instance *EffectInstance, owner *actorState, now time.Time) (*effectState, Obstacle) {
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
		Effect: Effect{
			ID:       instance.ID,
			Type:     instance.DefinitionID,
			Owner:    instance.OwnerActorID,
			Start:    now.UnixMilli(),
			Duration: meleeAttackDuration.Milliseconds(),
			X:        rectX,
			Y:        rectY,
			Width:    width,
			Height:   height,
			Params:   params,
		},
		telemetrySource:    telemetrySourceContract,
		telemetrySpawnTick: instance.StartTick,
	}
	area := Obstacle{X: rectX, Y: rectY, Width: width, Height: height}
	return effect, area
}

func (m *EffectManager) syncProjectileInstance(instance *EffectInstance, owner *actorState, effect *effectState) {
	if m == nil || instance == nil || effect == nil {
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
		instance.BehaviorState.Extra["dx"] = int(math.Round(effect.Projectile.VelocityUnitX))
		instance.Params["dx"] = instance.BehaviorState.Extra["dx"]
		instance.BehaviorState.Extra["dy"] = int(math.Round(effect.Projectile.VelocityUnitY))
		instance.Params["dy"] = instance.BehaviorState.Extra["dy"]
		if tpl := effect.Projectile.Template; tpl != nil && tpl.MaxDistance > 0 {
			instance.BehaviorState.Extra["range"] = int(math.Round(tpl.MaxDistance))
			instance.Params["range"] = instance.BehaviorState.Extra["range"]
		}
	}
	geometry := instance.DeliveryState.Geometry
	if effect.Effect.Width > 0 {
		geometry.Width = quantizeWorldCoord(effect.Effect.Width)
	}
	if effect.Effect.Height > 0 {
		geometry.Height = quantizeWorldCoord(effect.Effect.Height)
	}
	radius := effect.Effect.Params["radius"]
	if radius <= 0 {
		radius = effect.Effect.Width / 2
	}
	geometry.Radius = quantizeWorldCoord(radius)
	if owner != nil {
		geometry.OffsetX = quantizeWorldCoord(centerX(effect) - owner.X)
		geometry.OffsetY = quantizeWorldCoord(centerY(effect) - owner.Y)
	}
	instance.DeliveryState.Geometry = geometry
}

func (m *EffectManager) syncStatusVisualInstance(instance *EffectInstance, actor *actorState, effect *effectState) {
	if m == nil || instance == nil || effect == nil {
		return
	}
	geometry := instance.DeliveryState.Geometry
	width := effect.Effect.Width
	if width <= 0 {
		width = playerHalf * 2
	}
	height := effect.Effect.Height
	if height <= 0 {
		height = playerHalf * 2
	}
	geometry.Width = quantizeWorldCoord(width)
	geometry.Height = quantizeWorldCoord(height)
	if actor != nil {
		geometry.OffsetX = quantizeWorldCoord(centerX(effect) - actor.X)
		geometry.OffsetY = quantizeWorldCoord(centerY(effect) - actor.Y)
	}
	instance.DeliveryState.Geometry = geometry
}

func (m *EffectManager) spawnStatusVisualFromInstance(instance *EffectInstance, actor *actorState, lifetime time.Duration, now time.Time) *effectState {
	if m == nil || m.world == nil || instance == nil || actor == nil {
		return nil
	}
	if lifetime <= 0 {
		lifetime = burningTickInterval
	}
	effect := m.world.attachStatusEffectVisual(actor, instance.DefinitionID, lifetime, now)
	if effect == nil {
		return nil
	}
	effect.contractManaged = true
	effect.Effect.ID = instance.ID
	effect.StatusEffect = StatusEffectBurning
	effect.FollowActorID = actor.ID
	effect.telemetrySource = telemetrySourceContract
	effect.telemetrySpawnTick = instance.StartTick
	m.worldEffects[instance.ID] = effect
	return effect
}

func (m *EffectManager) attachVisualToStatusEffect(actor *actorState, effect *effectState) {
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

func (m *EffectManager) ensureBloodDecalInstance(instance *EffectInstance, now time.Time) *effectState {
	if m == nil || instance == nil || m.world == nil {
		return nil
	}
	effect := m.worldEffects[instance.ID]
	if effect == nil {
		effect = m.world.findEffectByID(instance.ID)
		if effect != nil {
			m.worldEffects[instance.ID] = effect
		}
	}
	if effect == nil {
		effect = m.world.spawnContractBloodDecalFromInstance(instance, now)
		if effect == nil {
			return nil
		}
		m.worldEffects[instance.ID] = effect
	}
	m.syncBloodDecalInstance(instance, effect)
	return effect
}

func (m *EffectManager) syncBloodDecalInstance(instance *EffectInstance, effect *effectState) {
	if m == nil || instance == nil || effect == nil {
		return
	}
	geometry := instance.DeliveryState.Geometry
	width := effect.Effect.Width
	if width <= 0 {
		width = playerHalf * 2
	}
	height := effect.Effect.Height
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

func defaultEffectDefinitions() map[string]*EffectDefinition {
	return map[string]*EffectDefinition{
		effectTypeAttack: &EffectDefinition{
			TypeID:        effectTypeAttack,
			Delivery:      DeliveryKindArea,
			Shape:         GeometryShapeRect,
			Motion:        MotionKindInstant,
			Impact:        ImpactPolicyAllInPath,
			LifetimeTicks: 1,
			Hooks: EffectHooks{
				OnSpawn: meleeSpawnHookID,
			},
			Client: ReplicationSpec{
				SendSpawn:       true,
				SendUpdates:     true,
				SendEnd:         true,
				ManagedByClient: true,
			},
			End: EndPolicy{Kind: EndInstant},
		},
		effectTypeFireball: &EffectDefinition{
			TypeID:        effectTypeFireball,
			Delivery:      DeliveryKindArea,
			Shape:         GeometryShapeCircle,
			Motion:        MotionKindLinear,
			Impact:        ImpactPolicyFirstHit,
			LifetimeTicks: 45,
			Hooks: EffectHooks{
				OnSpawn: projectileLifecycleHookID,
				OnTick:  projectileLifecycleHookID,
			},
			Client: ReplicationSpec{
				SendSpawn:   true,
				SendUpdates: true,
				SendEnd:     true,
			},
			End: EndPolicy{Kind: EndDuration},
		},
		effectTypeBurningTick: &EffectDefinition{
			TypeID:        effectTypeBurningTick,
			Delivery:      DeliveryKindTarget,
			Shape:         GeometryShapeRect,
			Motion:        MotionKindInstant,
			Impact:        ImpactPolicyFirstHit,
			LifetimeTicks: 1,
			Hooks: EffectHooks{
				OnSpawn: statusBurningDamageHookID,
			},
			Client: ReplicationSpec{
				SendSpawn:   true,
				SendUpdates: false,
				SendEnd:     true,
			},
			End: EndPolicy{Kind: EndInstant},
		},
		effectTypeBurningVisual: &EffectDefinition{
			TypeID:        effectTypeBurningVisual,
			Delivery:      DeliveryKindTarget,
			Shape:         GeometryShapeRect,
			Motion:        MotionKindFollow,
			Impact:        ImpactPolicyFirstHit,
			LifetimeTicks: durationToTicks(burningStatusEffectDuration),
			Hooks: EffectHooks{
				OnSpawn: statusBurningVisualHookID,
				OnTick:  statusBurningVisualHookID,
			},
			Client: ReplicationSpec{
				SendSpawn:   true,
				SendUpdates: true,
				SendEnd:     true,
			},
			End: EndPolicy{Kind: EndDuration},
		},
		effectTypeBloodSplatter: &EffectDefinition{
			TypeID:        effectTypeBloodSplatter,
			Delivery:      DeliveryKindVisual,
			Shape:         GeometryShapeRect,
			Motion:        MotionKindNone,
			Impact:        ImpactPolicyNone,
			LifetimeTicks: durationToTicks(bloodSplatterDuration),
			Hooks: EffectHooks{
				OnSpawn: visualBloodSplatterHookID,
				OnTick:  visualBloodSplatterHookID,
			},
			Client: ReplicationSpec{
				SendSpawn:       true,
				SendUpdates:     false,
				SendEnd:         true,
				ManagedByClient: true,
			},
			End: EndPolicy{Kind: EndDuration},
		},
	}
}
