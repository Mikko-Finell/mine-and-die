package main

import "fmt"

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
	totalEnqueued     int
	totalDrained      int
	lastTickProcessed Tick
	nextInstanceID    uint64
	world             *World
}

func newEffectManager(world *World) *EffectManager {
	return &EffectManager{
		intentQueue:   make([]EffectIntent, 0),
		instances:     make(map[string]*EffectInstance),
		definitions:   defaultEffectDefinitions(),
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
func (m *EffectManager) RunTick(tick Tick, emit func(EffectLifecycleEvent)) {
	if m == nil {
		return
	}
	if !enableContractEffectManager {
		m.intentQueue = m.intentQueue[:0]
		return
	}
	m.lastTickProcessed = tick
	drained := len(m.intentQueue)
	newInstances := make([]*EffectInstance, 0, drained)
	if drained > 0 {
		for _, intent := range m.intentQueue {
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
		}
	}
	if len(m.intentQueue) > 0 {
		for i := range m.intentQueue {
			m.intentQueue[i] = EffectIntent{}
		}
		m.intentQueue = m.intentQueue[:0]
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
			Extra:          copyIntMap(intent.Params),
		},
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

func defaultEffectDefinitions() map[string]*EffectDefinition {
	return map[string]*EffectDefinition{
		effectTypeAttack: &EffectDefinition{
			TypeID:        effectTypeAttack,
			Delivery:      DeliveryKindArea,
			Shape:         GeometryShapeRect,
			Motion:        MotionKindInstant,
			Impact:        ImpactPolicyFirstHit,
			LifetimeTicks: 1,
			Client: ReplicationSpec{
				SendSpawn:   true,
				SendUpdates: true,
				SendEnd:     true,
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
			Client: ReplicationSpec{
				SendSpawn:   true,
				SendUpdates: true,
				SendEnd:     true,
			},
			End: EndPolicy{Kind: EndDuration},
		},
	}
}
