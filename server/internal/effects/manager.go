package effects

import (
	"fmt"
	"time"

	effectcatalog "mine-and-die/server/effects/catalog"
	effectcontract "mine-and-die/server/effects/contract"
)

type Runtime interface {
	InstanceState(id string) any
	SetInstanceState(id string, state any)
	ClearInstanceState(id string)
	Registry() Registry
}

type HookFunc func(Runtime, *effectcontract.EffectInstance, effectcontract.Tick, time.Time)

type HookSet struct {
	OnSpawn HookFunc
	OnTick  HookFunc
}

type ManagerConfig struct {
	Definitions  map[string]*effectcontract.EffectDefinition
	Catalog      *effectcatalog.Resolver
	Hooks        map[string]HookSet
	OwnerMissing func(actorID string) bool
	Registry     func() Registry
}

type Manager struct {
	intentQueue       []effectcontract.EffectIntent
	instances         map[string]*effectcontract.EffectInstance
	definitions       map[string]*effectcontract.EffectDefinition
	catalog           *effectcatalog.Resolver
	seqByInstance     map[string]effectcontract.Seq
	hooks             map[string]HookSet
	instanceState     map[string]any
	totalEnqueued     int
	totalDrained      int
	lastTickProcessed effectcontract.Tick
	nextInstanceID    uint64
	ownerMissing      func(string) bool
	registry          func() Registry
}

func NewManager(cfg ManagerConfig) *Manager {
	definitions := cfg.Definitions
	if definitions == nil {
		definitions = effectcontract.BuiltInDefinitions()
	}

	hooks := cfg.Hooks
	if hooks == nil {
		hooks = make(map[string]HookSet)
	}

	return &Manager{
		intentQueue:   make([]effectcontract.EffectIntent, 0),
		instances:     make(map[string]*effectcontract.EffectInstance),
		definitions:   definitions,
		catalog:       cfg.Catalog,
		hooks:         hooks,
		instanceState: make(map[string]any),
		seqByInstance: make(map[string]effectcontract.Seq),
		ownerMissing:  cfg.OwnerMissing,
		registry:      cfg.Registry,
	}
}

func (m *Manager) Definitions() map[string]*effectcontract.EffectDefinition {
	if m == nil {
		return nil
	}
	return m.definitions
}

func (m *Manager) Hooks() map[string]HookSet {
	if m == nil {
		return nil
	}
	return m.hooks
}

func (m *Manager) Instances() map[string]*effectcontract.EffectInstance {
	if m == nil {
		return nil
	}
	return m.instances
}

func (m *Manager) Catalog() *effectcatalog.Resolver {
	if m == nil {
		return nil
	}
	return m.catalog
}

func (m *Manager) TotalEnqueued() int {
	if m == nil {
		return 0
	}
	return m.totalEnqueued
}

func (m *Manager) TotalDrained() int {
	if m == nil {
		return 0
	}
	return m.totalDrained
}

func (m *Manager) LastTickProcessed() effectcontract.Tick {
	if m == nil {
		return 0
	}
	return m.lastTickProcessed
}

func (m *Manager) PendingIntentCount() int {
	if m == nil {
		return 0
	}
	return len(m.intentQueue)
}

func (m *Manager) PendingIntents() []effectcontract.EffectIntent {
	if m == nil {
		return nil
	}
	return m.intentQueue
}

func (m *Manager) ResetPendingIntents() {
	if m == nil {
		return
	}
	for i := range m.intentQueue {
		m.intentQueue[i] = effectcontract.EffectIntent{}
	}
	m.intentQueue = m.intentQueue[:0]
}

func (m *Manager) InstanceState(id string) any {
	if m == nil || id == "" {
		return nil
	}
	return m.instanceState[id]
}

func (m *Manager) SetInstanceState(id string, state any) {
	if m == nil || id == "" {
		return
	}
	if state == nil {
		delete(m.instanceState, id)
		return
	}
	m.instanceState[id] = state
}

func (m *Manager) ClearInstanceState(id string) {
	if m == nil || id == "" {
		return
	}
	delete(m.instanceState, id)
}

func (m *Manager) Registry() Registry {
	if m == nil || m.registry == nil {
		return Registry{}
	}
	return m.registry()
}

func (m *Manager) EnqueueIntent(intent effectcontract.EffectIntent) {
	if m == nil {
		return
	}
	m.intentQueue = append(m.intentQueue, intent)
	m.totalEnqueued++
}

func (m *Manager) RunTick(tick effectcontract.Tick, now time.Time, emit func(effectcontract.EffectLifecycleEvent)) {
	if m == nil {
		return
	}
	m.lastTickProcessed = tick

	drainedQueue := append([]effectcontract.EffectIntent(nil), m.intentQueue...)
	if len(m.intentQueue) > 0 {
		for i := range m.intentQueue {
			m.intentQueue[i] = effectcontract.EffectIntent{}
		}
		m.intentQueue = m.intentQueue[:0]
	}

	drained := len(drainedQueue)
	newInstances := make([]*effectcontract.EffectInstance, 0, drained)
	if drained > 0 {
		for _, intent := range drainedQueue {
			instance := m.instantiateIntent(intent, tick)
			if instance == nil {
				continue
			}
			m.instances[instance.ID] = instance
			m.seqByInstance[instance.ID] = 0
			newInstances = append(newInstances, instance)
			m.invokeOnSpawn(instance, tick, now)
		}
		m.totalDrained += drained
	}

	if emit != nil {
		for _, instance := range newInstances {
			if instance == nil {
				continue
			}
			if !instance.Replication.SendSpawn {
				continue
			}
			spawn := effectcontract.EffectSpawnEvent{
				Tick:     tick,
				Seq:      m.nextSequenceFor(instance.ID),
				Instance: m.cloneInstanceForSpawn(instance),
			}
			emit(spawn)
		}
	}

	ended := make([]string, 0)
	for _, instance := range m.instances {
		if instance == nil {
			continue
		}
		shouldTick := m.shouldInvokeOnTick(instance)
		if shouldTick {
			m.invokeOnTick(instance, tick, now)
		}
		if instance.Replication.SendUpdates && shouldTick && emit != nil {
			delivery := m.cloneDeliveryState(instance.DeliveryState)
			behavior := m.cloneBehaviorState(instance.BehaviorState)
			update := effectcontract.EffectUpdateEvent{
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
			if instance.Replication.SendEnd && emit != nil {
				end := effectcontract.EffectEndEvent{
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
		if value := m.InstanceState(id); value != nil {
			if effect, ok := value.(*State); ok {
				UnregisterEffect(m.Registry(), effect)
			}
		}
		delete(m.instances, id)
		delete(m.seqByInstance, id)
		m.ClearInstanceState(id)
	}
}

func (m *Manager) instantiateIntent(intent effectcontract.EffectIntent, tick effectcontract.Tick) *effectcontract.EffectInstance {
	m.nextInstanceID++
	id := fmt.Sprintf("contract-effect-%d", m.nextInstanceID)
	geometry := intent.Geometry
	if geometry.Variants != nil {
		geometry.Variants = copyIntMap(geometry.Variants)
	}
	entryID := intent.EntryID
	if entryID == "" {
		entryID = intent.TypeID
	}
	if entryID == "" {
		return nil
	}
	definition, definitionID := m.resolveDefinition(entryID)
	if definitionID == "" {
		definitionID = intent.TypeID
	}
	if definitionID == "" {
		definitionID = entryID
	}
	replication := effectcontract.ReplicationSpec{SendSpawn: true, SendUpdates: true, SendEnd: true}
	endPolicy := effectcontract.EndPolicy{Kind: effectcontract.EndDuration}
	if definition != nil {
		replication = definition.Client
		endPolicy = definition.End
	}
	deliveryKind := intent.Delivery
	if deliveryKind == "" && definition != nil {
		deliveryKind = definition.Delivery
	}
	if deliveryKind == "" {
		deliveryKind = effectcontract.DeliveryKindArea
	}
	follow := effectcontract.FollowNone
	if deliveryKind == effectcontract.DeliveryKindTarget {
		follow = effectcontract.FollowTarget
	}
	ticksRemaining := intent.DurationTicks
	if ticksRemaining <= 0 && definition != nil && endPolicy.Kind == effectcontract.EndDuration {
		ticksRemaining = definition.LifetimeTicks
	}
	params := copyIntMap(intent.Params)
	extra := copyIntMap(intent.Params)
	cadence := normalizedTickCadence(intent.TickCadence)
	cooldown := 0
	if cadence > 1 {
		cooldown = cadence
	}
	instance := &effectcontract.EffectInstance{
		ID:           id,
		EntryID:      entryID,
		DefinitionID: definitionID,
		Definition:   definition,
		StartTick:    tick,
		DeliveryState: effectcontract.EffectDeliveryState{
			Geometry:        geometry,
			Follow:          follow,
			AttachedActorID: intent.TargetActorID,
		},
		BehaviorState: effectcontract.EffectBehaviorState{
			TicksRemaining: ticksRemaining,
			CooldownTicks:  cooldown,
			TickCadence:    cadence,
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

func normalizedTickCadence(raw int) int {
	if raw <= 0 {
		return 1
	}
	return raw
}

func (m *Manager) nextSequenceFor(id string) effectcontract.Seq {
	if id == "" {
		return 0
	}
	next := m.seqByInstance[id] + 1
	m.seqByInstance[id] = next
	return next
}

func (m *Manager) cloneInstanceForSpawn(instance *effectcontract.EffectInstance) effectcontract.EffectInstance {
	if instance == nil {
		return effectcontract.EffectInstance{}
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

func (m *Manager) cloneDeliveryState(state effectcontract.EffectDeliveryState) effectcontract.EffectDeliveryState {
	clone := state
	clone.Geometry = cloneGeometry(state.Geometry)
	return clone
}

func (m *Manager) cloneBehaviorState(state effectcontract.EffectBehaviorState) effectcontract.EffectBehaviorState {
	clone := state
	clone.Extra = copyIntMap(state.Extra)
	clone.Stacks = copyIntMap(state.Stacks)
	return clone
}

func (m *Manager) shouldInvokeOnTick(instance *effectcontract.EffectInstance) bool {
	if m == nil || instance == nil {
		return false
	}
	cadence := instance.BehaviorState.TickCadence
	if cadence <= 1 {
		if cadence <= 0 {
			instance.BehaviorState.TickCadence = 1
		}
		instance.BehaviorState.CooldownTicks = 0
		return true
	}
	if instance.BehaviorState.CooldownTicks <= 1 {
		instance.BehaviorState.CooldownTicks = cadence
		return true
	}
	instance.BehaviorState.CooldownTicks--
	return false
}

func (m *Manager) invokeOnSpawn(instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
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

func (m *Manager) invokeOnTick(instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
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

func (m *Manager) resolveDefinition(typeID string) (*effectcontract.EffectDefinition, string) {
	if typeID == "" {
		return nil, ""
	}
	contractID := typeID
	var catalogDef *effectcontract.EffectDefinition
	if m != nil && m.catalog != nil {
		if entry, ok := m.catalog.Resolve(typeID); ok {
			contractID = entry.ContractID
			catalogDef = entry.Definition
		}
	}
	if m == nil {
		return catalogDef, contractID
	}
	if def, ok := m.definitions[contractID]; ok && def != nil {
		return def, contractID
	}
	if catalogDef != nil {
		return catalogDef, contractID
	}
	return nil, contractID
}

func (m *Manager) evaluateEndPolicy(instance *effectcontract.EffectInstance, tick effectcontract.Tick) endDecision {
	if instance == nil {
		return endDecision{}
	}
	switch instance.End.Kind {
	case effectcontract.EndInstant:
		if tick >= instance.StartTick {
			return endDecision{shouldEnd: true, reason: effectcontract.EndReasonExpired}
		}
	case effectcontract.EndDuration:
		if instance.BehaviorState.TicksRemaining > 0 {
			instance.BehaviorState.TicksRemaining--
		}
		if instance.BehaviorState.TicksRemaining <= 0 {
			return endDecision{shouldEnd: true, reason: effectcontract.EndReasonExpired}
		}
	case effectcontract.EndCondition:
		if instance.End.Conditions.OnOwnerLost && m.ownerMissing != nil && m.ownerMissing(instance.OwnerActorID) {
			return endDecision{shouldEnd: true, reason: effectcontract.EndReasonOwnerLost}
		}
	}
	return endDecision{}
}

type endDecision struct {
	shouldEnd bool
	reason    effectcontract.EndReason
}

func cloneGeometry(src effectcontract.EffectGeometry) effectcontract.EffectGeometry {
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
