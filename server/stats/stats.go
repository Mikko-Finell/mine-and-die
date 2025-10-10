package stats

import (
	"math"
	"sort"
)

// StatID enumerates the primary attributes tracked by the stats engine.
type StatID uint8

const (
	StatMight StatID = iota
	StatResonance
	StatFocus
	StatSpeed

	StatCount
)

// DerivedID enumerates derived stats computed from the attribute totals.
type DerivedID uint8

const (
	DerivedMaxHealth DerivedID = iota
	DerivedMaxMana
	DerivedDamageScalarPhysical
	DerivedDamageScalarMagical
	DerivedAccuracy
	DerivedEvasion
	DerivedCastSpeed
	DerivedCooldownRate
	DerivedStaggerResist

	DerivedCount
)

// Layer describes the precedence order for additive and multiplicative modifiers.
type Layer uint8

const (
	LayerBase Layer = iota
	LayerPermanent
	LayerEquipment
	LayerTemporary
	LayerEnvironment
	LayerAdmin

	LayerCount
)

// SourceKind identifies the origin of a stat modifier for deterministic ordering.
type SourceKind uint8

const (
	SourceKindUnknown SourceKind = iota
	SourceKindArchetype
	SourceKindProgression
	SourceKindEquipment
	SourceKindTemporary
	SourceKindEnvironment
	SourceKindAdmin
)

// SourceKey uniquely identifies the origin of a modifier inside a layer.
type SourceKey struct {
	Kind SourceKind
	ID   string
}

// ValueSet stores a fixed vector of stat values.
type ValueSet [StatCount]float64

// DerivedSet stores derived stat values.
type DerivedSet [DerivedCount]float64

// OverrideValue represents a stat override entry.
type OverrideValue struct {
	Active bool
	Value  float64
}

// OverrideSet stores per-stat override entries.
type OverrideSet [StatCount]OverrideValue

// LayerStack caches the aggregate contributions for a modifier layer.
type LayerStack struct {
	add      ValueSet
	mul      ValueSet
	override OverrideSet
	version  uint64
}

type layerSource struct {
	delta         StatDelta
	expiresAtTick uint64
}

// Component owns the stats state for an actor and caches derived totals.
type Component struct {
	layers          [LayerCount]LayerStack
	sources         map[Layer]map[SourceKey]*layerSource
	totals          ValueSet
	derived         DerivedSet
	dirty           bool
	version         uint64
	lastResolveTick uint64
}

// StatDelta captures additive, multiplicative, and override contributions supplied by a source.
type StatDelta struct {
	Add      ValueSet
	Mul      ValueSet
	Override OverrideSet
}

// CommandStatChange represents an atomic mutation applied to the component.
type CommandStatChange struct {
	Layer         Layer
	Source        SourceKey
	Delta         StatDelta
	ExpiresAtTick uint64
	Remove        bool
}

// NewComponent constructs a component seeded with the provided base values.
func NewComponent(base ValueSet) Component {
	c := Component{}
	c.ensureInit()
	baseDelta := NewStatDelta()
	baseDelta.Add = base
	_ = c.applySource(LayerBase, SourceKey{Kind: SourceKindArchetype, ID: "base"}, baseDelta, 0)
	c.dirty = true
	return c
}

// ensureInit prepares lazy maps and multipliers.
func (c *Component) ensureInit() {
	if c.sources != nil {
		return
	}
	c.sources = make(map[Layer]map[SourceKey]*layerSource)
	for layer := Layer(0); layer < LayerCount; layer++ {
		c.layers[layer].mul = unitValueSet()
	}
	c.dirty = true
}

// NewStatDelta creates a delta with neutral multiplicative values.
func NewStatDelta() StatDelta {
	d := StatDelta{}
	d.Mul = unitValueSet()
	return d
}

// Apply mutates the component according to the provided command.
func (c *Component) Apply(change CommandStatChange) {
	if c == nil {
		return
	}
	c.ensureInit()
	if change.Layer >= LayerCount {
		return
	}
	if change.Remove {
		if c.removeSource(change.Layer, change.Source) {
			c.dirty = true
		}
		return
	}
	if change.Layer == LayerTemporary && change.ExpiresAtTick == 0 {
		change.ExpiresAtTick = c.lastResolveTick + 1
	}
	if c.applySource(change.Layer, change.Source, change.Delta, change.ExpiresAtTick) {
		c.dirty = true
	}
}

// Resolve folds all layers in deterministic order and recomputes derived stats.
func (c *Component) Resolve(tick uint64) {
	if c == nil {
		return
	}
	c.ensureInit()
	c.cullExpired(tick)
	if !c.dirty && c.lastResolveTick == tick {
		return
	}

	total := c.layers[LayerBase].add
	multiplyValueSet(&total, c.layers[LayerBase].mul)
	applyOverrides(&total, c.layers[LayerBase].override)

	for layer := LayerPermanent; layer < LayerCount; layer++ {
		stack := &c.layers[layer]
		addValueSet(&total, stack.add)
		multiplyValueSet(&total, stack.mul)
		applyOverrides(&total, stack.override)
	}

	c.totals = total
	c.derived = computeDerived(total)
	c.version++
	c.lastResolveTick = tick
	c.dirty = false
}

// Totals returns the cached total stat values.
func (c *Component) Totals() ValueSet {
	return c.totals
}

// GetTotal returns the cached total for a specific stat.
func (c *Component) GetTotal(id StatID) float64 {
	if id >= StatCount {
		return 0
	}
	return c.totals[id]
}

// GetDerived returns the cached derived stat value.
func (c *Component) GetDerived(id DerivedID) float64 {
	if id >= DerivedCount {
		return 0
	}
	return c.derived[id]
}

// DerivedValues returns a copy of the derived set.
func (c *Component) DerivedValues() DerivedSet {
	return c.derived
}

// Version returns the component version updated on each resolve.
func (c *Component) Version() uint64 {
	return c.version
}

func (c *Component) applySource(layer Layer, key SourceKey, delta StatDelta, expires uint64) bool {
	if c.sources[layer] == nil {
		c.sources[layer] = make(map[SourceKey]*layerSource)
	}
	current := c.sources[layer][key]
	if current != nil {
		if sourcesEqual(current.delta, delta) && current.expiresAtTick == expires {
			return false
		}
	} else {
		current = &layerSource{}
		c.sources[layer][key] = current
	}
	current.delta = delta
	current.expiresAtTick = expires
	c.rebuildLayerStack(layer)
	return true
}

func (c *Component) removeSource(layer Layer, key SourceKey) bool {
	entries := c.sources[layer]
	if len(entries) == 0 {
		return false
	}
	if _, ok := entries[key]; !ok {
		return false
	}
	delete(entries, key)
	if len(entries) == 0 {
		delete(c.sources, layer)
	}
	c.rebuildLayerStack(layer)
	return true
}

func (c *Component) rebuildLayerStack(layer Layer) {
	stack := &c.layers[layer]
	stack.add = ValueSet{}
	stack.mul = unitValueSet()
	stack.override = OverrideSet{}
	entries := c.sources[layer]
	if len(entries) == 0 {
		stack.version++
		return
	}
	keys := make([]SourceKey, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Kind != keys[j].Kind {
			return keys[i].Kind < keys[j].Kind
		}
		return keys[i].ID < keys[j].ID
	})
	for _, key := range keys {
		src := entries[key]
		addValueSet(&stack.add, src.delta.Add)
		multiplyValueSet(&stack.mul, src.delta.Mul)
		mergeOverrides(&stack.override, src.delta.Override)
	}
	stack.version++
}

func (c *Component) cullExpired(tick uint64) {
	entries := c.sources[LayerTemporary]
	if len(entries) == 0 {
		return
	}
	removed := false
	for key, src := range entries {
		if src.expiresAtTick > 0 && tick >= src.expiresAtTick {
			delete(entries, key)
			removed = true
		}
	}
	if removed {
		if len(entries) == 0 {
			delete(c.sources, LayerTemporary)
		}
		c.rebuildLayerStack(LayerTemporary)
		c.dirty = true
	}
}

func addValueSet(target *ValueSet, other ValueSet) {
	for i := range target {
		target[i] += other[i]
	}
}

func multiplyValueSet(target *ValueSet, other ValueSet) {
	for i := range target {
		target[i] *= other[i]
	}
}

func applyOverrides(target *ValueSet, overrides OverrideSet) {
	for i := range overrides {
		if overrides[i].Active {
			target[i] = overrides[i].Value
		}
	}
}

func mergeOverrides(target *OverrideSet, other OverrideSet) {
	for i := range other {
		if other[i].Active {
			target[i] = other[i]
		}
	}
}

func unitValueSet() ValueSet {
	var vs ValueSet
	for i := range vs {
		vs[i] = 1
	}
	return vs
}

func sourcesEqual(a, b StatDelta) bool {
	for i := range a.Add {
		if math.Abs(a.Add[i]-b.Add[i]) > 1e-9 {
			return false
		}
		if math.Abs(a.Mul[i]-b.Mul[i]) > 1e-9 {
			return false
		}
		if a.Override[i].Active != b.Override[i].Active {
			return false
		}
		if a.Override[i].Active && math.Abs(a.Override[i].Value-b.Override[i].Value) > 1e-9 {
			return false
		}
	}
	return true
}
