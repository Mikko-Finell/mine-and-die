package items

import (
	"fmt"
	"math"
	"sort"

	"mine-and-die/server/internal/journal"
)

const positionEpsilon = 1e-6

func positionsEqual(ax, ay, bx, by float64) bool {
	return math.Abs(ax-bx) <= positionEpsilon && math.Abs(ay-by) <= positionEpsilon
}

// GroundTileKey identifies the map tile that currently contains a ground item stack.
type GroundTileKey struct {
	X int
	Y int
}

// GroundItem mirrors the legacy ground-item snapshot exposed to callers.
type GroundItem struct {
	ID             string  `json:"id"`
	Type           string  `json:"type"`
	FungibilityKey string  `json:"fungibility_key"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Qty            int     `json:"qty"`
}

// GroundItemState tracks a ground item along with its tile metadata.
type GroundItemState struct {
	GroundItem
	Tile    GroundTileKey
	Version uint64
}

// Actor captures the minimal actor metadata required for ground item placement.
type Actor struct {
	ID string
	X  float64
	Y  float64
}

// ItemStack describes a fungible stack that can be moved to the ground.
type ItemStack struct {
	Type           string
	FungibilityKey string
	Quantity       int
}

// ScatterConfig carries the geometry parameters used when scattering items inside a tile.
type ScatterConfig struct {
	TileSize    float64
	MinDistance float64
	MaxDistance float64
	Padding     float64
}

// TileForPosition computes the grid coordinate for the provided point.
func TileForPosition(x, y, tileSize float64) GroundTileKey {
	return GroundTileKey{X: int(math.Floor(x / tileSize)), Y: int(math.Floor(y / tileSize))}
}

// TileCenter returns the midpoint for the provided tile.
func TileCenter(tile GroundTileKey, tileSize float64) (float64, float64) {
	return float64(tile.X)*tileSize + tileSize/2, float64(tile.Y)*tileSize + tileSize/2
}

// ScatterGroundItemPosition selects a deterministic position for a ground item inside the tile.
func ScatterGroundItemPosition(
	actor *Actor,
	tile GroundTileKey,
	cfg ScatterConfig,
	randomAngle func() float64,
	randomDistance func(min, max float64) float64,
) (float64, float64) {
	if actor == nil {
		return TileCenter(tile, cfg.TileSize)
	}

	angle := 0.0
	if randomAngle != nil {
		angle = randomAngle()
	}

	distance := cfg.MinDistance
	if randomDistance != nil {
		distance = randomDistance(cfg.MinDistance, cfg.MaxDistance)
	}

	baseX := actor.X
	baseY := actor.Y
	x := baseX + math.Cos(angle)*distance
	y := baseY + math.Sin(angle)*distance

	left := float64(tile.X) * cfg.TileSize
	top := float64(tile.Y) * cfg.TileSize
	right := left + cfg.TileSize
	bottom := top + cfg.TileSize

	padding := cfg.Padding
	if padding*2 >= cfg.TileSize {
		padding = 0
	}

	minX := left + padding
	maxX := right - padding
	minY := top + padding
	maxY := bottom - padding

	return clampFloat(x, minX, maxX), clampFloat(y, minY, maxY)
}

// UpsertGroundItem merges the provided stack into the store, creating a new entry when required.
// The ensureKey callback should populate the stack's fungibility key when missing, returning true on success.
// Setters and logDrop mirror the legacy world helpers so wrappers can record patches and telemetry.
func UpsertGroundItem(
	items map[string]*GroundItemState,
	itemsByTile map[GroundTileKey]map[string]*GroundItemState,
	nextID *uint64,
	actor *Actor,
	stack ItemStack,
	reason string,
	cfg ScatterConfig,
	randomAngle func() float64,
	randomDistance func(min, max float64) float64,
	ensureKey func(*ItemStack) bool,
	setQuantity func(*GroundItemState, int),
	setPosition func(*GroundItemState, float64, float64),
	logDrop func(*Actor, ItemStack, string, string),
) *GroundItemState {
	if items == nil || itemsByTile == nil || nextID == nil || actor == nil {
		return nil
	}
	if stack.Quantity <= 0 || stack.Type == "" {
		return nil
	}

	if setQuantity == nil || setPosition == nil {
		return nil
	}

	tile := TileForPosition(actor.X, actor.Y, cfg.TileSize)
	x, y := ScatterGroundItemPosition(actor, tile, cfg, randomAngle, randomDistance)

	itemsByType := itemsByTile[tile]
	if itemsByType == nil {
		itemsByType = make(map[string]*GroundItemState)
		itemsByTile[tile] = itemsByType
	}

	if stack.FungibilityKey == "" {
		if ensureKey == nil || !ensureKey(&stack) {
			return nil
		}
	}
	if stack.FungibilityKey == "" {
		return nil
	}

	if existing := itemsByType[stack.FungibilityKey]; existing != nil {
		setQuantity(existing, existing.Qty+stack.Quantity)
		existing.Tile = tile
		setPosition(existing, x, y)
		if logDrop != nil {
			logDrop(actor, stack, reason, existing.ID)
		}
		return existing
	}

	*nextID = *nextID + 1
	id := fmt.Sprintf("ground-%d", *nextID)

	item := &GroundItemState{
		GroundItem: GroundItem{
			ID:             id,
			Type:           stack.Type,
			FungibilityKey: stack.FungibilityKey,
			X:              x,
			Y:              y,
			Qty:            stack.Quantity,
		},
		Tile: tile,
	}

	items[id] = item
	itemsByType[stack.FungibilityKey] = item

	if logDrop != nil {
		logDrop(actor, stack, reason, id)
	}

	return item
}

// RemoveGroundItem deletes the provided ground item from the store and tile index.
func RemoveGroundItem(
	items map[string]*GroundItemState,
	itemsByTile map[GroundTileKey]map[string]*GroundItemState,
	item *GroundItemState,
	appendPatch func(journal.Patch),
) {
	if items == nil || item == nil || appendPatch == nil {
		return
	}

	setQuantity := GroundItemQuantityJournalSetter(appendPatch)

	if item.Qty > 0 {
		setQuantity(item, 0)
	}

	delete(items, item.ID)

	if itemsByTile == nil {
		return
	}

	if itemsByType, ok := itemsByTile[item.Tile]; ok {
		delete(itemsByType, item.FungibilityKey)
		if len(itemsByType) == 0 {
			delete(itemsByTile, item.Tile)
		}
	}
}

// NearestGroundItem finds the closest stack of the requested type relative to the actor.
// Returns nil when no matching stack is available.
func NearestGroundItem(
	items map[string]*GroundItemState,
	actor *Actor,
	itemType string,
) (*GroundItemState, float64) {
	if len(items) == 0 || actor == nil || itemType == "" {
		return nil, 0
	}

	var best *GroundItemState
	bestDistance := math.MaxFloat64

	for _, item := range items {
		if item == nil || item.Qty <= 0 || item.Type != itemType {
			continue
		}

		dx := item.X - actor.X
		dy := item.Y - actor.Y
		distance := math.Hypot(dx, dy)

		if distance < bestDistance {
			bestDistance = distance
			best = item
		}
	}

	if best == nil {
		return nil, 0
	}

	return best, bestDistance
}

// SetGroundItemPosition updates the ground item's coordinates when they change.
// Returns true when the mutation was applied.
func SetGroundItemPosition(x, y *float64, newX, newY float64) bool {
	if x == nil || y == nil {
		return false
	}

	if positionsEqual(*x, *y, newX, newY) {
		return false
	}

	*x = newX
	*y = newY
	return true
}

// SetGroundItemQuantity clamps the quantity to zero or greater and updates the
// stored value when it changes. Returns true when the mutation was applied.
func SetGroundItemQuantity(qty *int, newQty int) bool {
	if qty == nil {
		return false
	}

	if newQty < 0 {
		newQty = 0
	}

	if *qty == newQty {
		return false
	}

	*qty = newQty
	return true
}

func groundItemsSnapshot(items map[string]*GroundItemState) []GroundItem {
	if len(items) == 0 {
		return make([]GroundItem, 0)
	}

	snapshot := make([]GroundItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		snapshot = append(snapshot, item.GroundItem)
	}

	sort.Slice(snapshot, func(i, j int) bool {
		return snapshot[i].ID < snapshot[j].ID
	})

	return snapshot
}

// GroundItemsSnapshot returns a broadcast-friendly copy of the provided ground items.
func GroundItemsSnapshot(items map[string]*GroundItemState) []GroundItem {
	snapshot := groundItemsSnapshot(items)
	if snapshot == nil {
		return make([]GroundItem, 0)
	}
	return snapshot
}

const goldItemType = "gold"

const (
	// PickupFailureReasonNotFound indicates no matching stack could be collected.
	PickupFailureReasonNotFound = "not_found"
	// PickupFailureReasonOutOfRange indicates the nearest stack was beyond the allowed radius.
	PickupFailureReasonOutOfRange = "out_of_range"
	// PickupFailureReasonInventoryError indicates the inventory mutation failed.
	PickupFailureReasonInventoryError = "inventory_error"
)

const (
	// DropFailureReasonInvalidQuantity indicates the requested quantity could not be processed.
	DropFailureReasonInvalidQuantity = "invalid_quantity"
	// DropFailureReasonInsufficientGold indicates the actor did not have enough gold available.
	DropFailureReasonInsufficientGold = "insufficient_gold"
	// DropFailureReasonInventoryError indicates the inventory mutation failed.
	DropFailureReasonInventoryError = "inventory_error"
)

// PickupResult captures the outcome of a successful ground item pickup.
type PickupResult struct {
	StackID  string
	Quantity int
	Distance float64
}

// PickupFailure describes why a pickup attempt failed.
type PickupFailure struct {
	Reason   string
	StackID  string
	Distance float64
	Err      string
}

// DropResult captures the outcome of a successful gold drop.
type DropResult struct {
	StackID  string
	Quantity int
}

// DropFailure describes why a drop attempt failed.
type DropFailure struct {
	Reason string
	Err    string
}

// PickupNearestItem moves the nearest stack of the requested type into the inventory via the
// provided callback when it falls within the allowed radius. The remove callback is invoked once
// the transfer succeeds or when the stack quantity is already depleted. Returns a PickupResult on
// success or a PickupFailure when the attempt cannot be completed.
func PickupNearestItem(
	items map[string]*GroundItemState,
	itemsByTile map[GroundTileKey]map[string]*GroundItemState,
	actor *Actor,
	itemType string,
	maxDistance float64,
	addToInventory func(ItemStack) error,
	appendPatch func(journal.Patch),
) (*PickupResult, *PickupFailure) {
	if len(items) == 0 || actor == nil || itemType == "" || addToInventory == nil || appendPatch == nil {
		return nil, &PickupFailure{Reason: PickupFailureReasonNotFound}
	}

	var item *GroundItemState
	bestDistance := math.MaxFloat64
	var depleted *GroundItemState
	depletedDistance := math.MaxFloat64

	for _, candidate := range items {
		if candidate == nil || candidate.Type != itemType {
			continue
		}

		dx := candidate.X - actor.X
		dy := candidate.Y - actor.Y
		distance := math.Hypot(dx, dy)

		if candidate.Qty <= 0 {
			if distance < depletedDistance {
				depleted = candidate
				depletedDistance = distance
			}
			continue
		}

		if distance < bestDistance {
			item = candidate
			bestDistance = distance
		}
	}

	if item == nil {
		if depleted != nil {
			RemoveGroundItem(items, itemsByTile, depleted, appendPatch)
			return nil, &PickupFailure{Reason: PickupFailureReasonNotFound, StackID: depleted.ID}
		}
		return nil, &PickupFailure{Reason: PickupFailureReasonNotFound}
	}

	if maxDistance >= 0 && bestDistance > maxDistance {
		return nil, &PickupFailure{Reason: PickupFailureReasonOutOfRange, StackID: item.ID, Distance: bestDistance}
	}

	qty := item.Qty
	if qty <= 0 {
		RemoveGroundItem(items, itemsByTile, item, appendPatch)
		return nil, &PickupFailure{Reason: PickupFailureReasonNotFound, StackID: item.ID}
	}

	stack := ItemStack{Type: item.Type, FungibilityKey: item.FungibilityKey, Quantity: qty}
	if err := addToInventory(stack); err != nil {
		failure := &PickupFailure{
			Reason:   PickupFailureReasonInventoryError,
			StackID:  item.ID,
			Distance: bestDistance,
		}
		if err != nil {
			failure.Err = err.Error()
		}
		return nil, failure
	}

	RemoveGroundItem(items, itemsByTile, item, appendPatch)
	return &PickupResult{StackID: item.ID, Quantity: qty, Distance: bestDistance}, nil
}

type GroundDropConfig struct {
	Items          map[string]*GroundItemState
	ItemsByTile    map[GroundTileKey]map[string]*GroundItemState
	NextID         *uint64
	Actor          *Actor
	Scatter        ScatterConfig
	RandomAngle    func() float64
	RandomDistance func(min, max float64) float64
	EnsureKey      func(*ItemStack) bool
	AppendPatch    func(journal.Patch)
	LogDrop        func(*Actor, ItemStack, string, string)
}

type GroundDropDelegates struct {
	items       map[string]*GroundItemState
	itemsByTile map[GroundTileKey]map[string]*GroundItemState
	nextID      *uint64
	actor       *Actor
	cfg         ScatterConfig
	angleFn     func() float64
	distanceFn  func(min, max float64) float64
	ensureKey   func(*ItemStack) bool
	setQuantity func(*GroundItemState, int)
	setPosition func(*GroundItemState, float64, float64)
	logDrop     func(*Actor, ItemStack, string, string)
}

// GroundItemQuantityJournalSetter returns a setter that updates the stack quantity and
// records a journal patch when the value changes.
func GroundItemQuantityJournalSetter(appendPatch func(journal.Patch)) func(*GroundItemState, int) {
	return func(item *GroundItemState, qty int) {
		if item == nil {
			return
		}
		if !SetGroundItemQuantity(&item.Qty, qty) {
			return
		}
		item.Version++
		if appendPatch == nil {
			return
		}
		appendPatch(journal.Patch{
			Kind:     journal.PatchGroundItemQty,
			EntityID: item.ID,
			Payload:  journal.GroundItemQtyPayload{Qty: item.Qty},
		})
	}
}

// GroundItemPositionJournalSetter returns a setter that updates the stack coordinates and
// records a journal patch when the value changes.
func GroundItemPositionJournalSetter(appendPatch func(journal.Patch)) func(*GroundItemState, float64, float64) {
	return func(item *GroundItemState, x, y float64) {
		if item == nil {
			return
		}
		if !SetGroundItemPosition(&item.X, &item.Y, x, y) {
			return
		}
		item.Version++
		if appendPatch == nil {
			return
		}
		appendPatch(journal.Patch{
			Kind:     journal.PatchGroundItemPos,
			EntityID: item.ID,
			Payload:  journal.GroundItemPosPayload{X: item.X, Y: item.Y},
		})
	}
}

// GroundDropRemoveStacksProvider removes all stacks of the requested item type when the
// actor lookup succeeds. The boolean return value indicates whether the provider handled
// the request (allowing callers to fall back when lookups fail).
type GroundDropRemoveStacksProvider func(string) ([]ItemStack, bool)

// GroundDropDrainProvider drains stacks when the actor lookup succeeds. The boolean return
// value indicates whether the provider handled the request.
type GroundDropDrainProvider func() ([]ItemStack, bool)

// GroundDropActorConfig wires the closures required for inventory and equipment drains
// across the various actor ownership cases.
type GroundDropActorConfig struct {
	RemovePlayerStacks   GroundDropRemoveStacksProvider
	RemoveNPCStacks      GroundDropRemoveStacksProvider
	RemoveFallbackStacks GroundDropRemoveStacksProvider

	RemoveGoldQuantity func(int) (int, error)

	DrainPlayerInventory   GroundDropDrainProvider
	DrainNPCInventory      GroundDropDrainProvider
	DrainFallbackInventory GroundDropDrainProvider

	DrainPlayerEquipment   GroundDropDrainProvider
	DrainNPCEquipment      GroundDropDrainProvider
	DrainFallbackEquipment GroundDropDrainProvider
}

func BuildGroundDropDelegates(cfg GroundDropConfig) (GroundDropDelegates, bool) {
	if cfg.Items == nil || cfg.ItemsByTile == nil || cfg.NextID == nil || cfg.Actor == nil || cfg.AppendPatch == nil {
		return GroundDropDelegates{}, false
	}

	delegates := GroundDropDelegates{
		items:       cfg.Items,
		itemsByTile: cfg.ItemsByTile,
		nextID:      cfg.NextID,
		actor:       cfg.Actor,
		cfg:         cfg.Scatter,
		angleFn:     cfg.RandomAngle,
		distanceFn:  cfg.RandomDistance,
		ensureKey:   cfg.EnsureKey,
		setQuantity: GroundItemQuantityJournalSetter(cfg.AppendPatch),
		setPosition: GroundItemPositionJournalSetter(cfg.AppendPatch),
		logDrop:     cfg.LogDrop,
	}

	return delegates, true
}

func InvokeGroundDrop(cfg GroundDropConfig, call func(GroundDropDelegates) int) int {
	if call == nil {
		return 0
	}

	delegates, ok := BuildGroundDropDelegates(cfg)
	if !ok {
		return 0
	}

	return call(delegates)
}

func GroundDropRemoveStacksFunc(cfg GroundDropActorConfig) func(string) []ItemStack {
	providers := make([]GroundDropRemoveStacksProvider, 0, 3)
	if cfg.RemovePlayerStacks != nil {
		providers = append(providers, cfg.RemovePlayerStacks)
	}
	if cfg.RemoveNPCStacks != nil {
		providers = append(providers, cfg.RemoveNPCStacks)
	}
	if cfg.RemoveFallbackStacks != nil {
		providers = append(providers, cfg.RemoveFallbackStacks)
	}
	if len(providers) == 0 {
		return nil
	}

	return func(itemType string) []ItemStack {
		if itemType == "" {
			return nil
		}
		for _, provider := range providers {
			stacks, handled := provider(itemType)
			if handled {
				return stacks
			}
		}
		return nil
	}
}

func GroundDropInventoryDrainFunc(cfg GroundDropActorConfig) func() []ItemStack {
	providers := make([]GroundDropDrainProvider, 0, 3)
	if cfg.DrainPlayerInventory != nil {
		providers = append(providers, cfg.DrainPlayerInventory)
	}
	if cfg.DrainNPCInventory != nil {
		providers = append(providers, cfg.DrainNPCInventory)
	}
	if cfg.DrainFallbackInventory != nil {
		providers = append(providers, cfg.DrainFallbackInventory)
	}
	if len(providers) == 0 {
		return nil
	}

	return func() []ItemStack {
		for _, provider := range providers {
			stacks, handled := provider()
			if handled {
				return stacks
			}
		}
		return nil
	}
}

func GroundDropEquipmentDrainFunc(cfg GroundDropActorConfig) func() []ItemStack {
	providers := make([]GroundDropDrainProvider, 0, 3)
	if cfg.DrainPlayerEquipment != nil {
		providers = append(providers, cfg.DrainPlayerEquipment)
	}
	if cfg.DrainNPCEquipment != nil {
		providers = append(providers, cfg.DrainNPCEquipment)
	}
	if cfg.DrainFallbackEquipment != nil {
		providers = append(providers, cfg.DrainFallbackEquipment)
	}
	if len(providers) == 0 {
		return nil
	}

	return func() []ItemStack {
		for _, provider := range providers {
			stacks, handled := provider()
			if handled {
				return stacks
			}
		}
		return nil
	}
}

func GroundDropRemoveGoldQuantityFunc(cfg GroundDropActorConfig) func(int) (int, error) {
	return cfg.RemoveGoldQuantity
}

func (d GroundDropDelegates) valid() bool {
	return d.items != nil && d.itemsByTile != nil && d.nextID != nil && d.actor != nil
}

// DropGoldQuantity removes the requested quantity of gold from the actor via the provided
// callbacks and places it on the ground. Returns a DropResult on success or a DropFailure when
// the transfer cannot be completed.
func DropGoldQuantity(
	delegates GroundDropDelegates,
	quantity int,
	reason string,
	available func() int,
	removeQuantity func(int) (int, error),
) (*DropResult, *DropFailure) {
	if !delegates.valid() {
		return nil, &DropFailure{Reason: DropFailureReasonInventoryError}
	}
	if quantity <= 0 {
		return nil, &DropFailure{Reason: DropFailureReasonInvalidQuantity}
	}
	if available == nil || removeQuantity == nil {
		return nil, &DropFailure{Reason: DropFailureReasonInventoryError}
	}

	if available() < quantity {
		return nil, &DropFailure{Reason: DropFailureReasonInsufficientGold}
	}

	removed, err := removeQuantity(quantity)
	if err != nil || removed != quantity {
		failure := &DropFailure{Reason: DropFailureReasonInventoryError}
		if err != nil {
			failure.Err = err.Error()
		}
		return nil, failure
	}

	stack := ItemStack{Type: goldItemType, Quantity: removed}
	item := UpsertGroundItem(
		delegates.items,
		delegates.itemsByTile,
		delegates.nextID,
		delegates.actor,
		stack,
		reason,
		delegates.cfg,
		delegates.angleFn,
		delegates.distanceFn,
		delegates.ensureKey,
		delegates.setQuantity,
		delegates.setPosition,
		delegates.logDrop,
	)

	result := &DropResult{Quantity: removed}
	if item != nil {
		result.StackID = item.ID
	}
	return result, nil
}

// DropAllGold removes all gold stacks from the actor and places them on the ground.
// Returns the total quantity dropped.
func DropAllGold(
	delegates GroundDropDelegates,
	reason string,
	removeStacks func(string) []ItemStack,
) int {
	return DropAllItemsOfType(
		delegates,
		goldItemType,
		reason,
		removeStacks,
	)
}

// DropAllItemsOfType removes all stacks of the requested item type from the actor and
// places them on the ground. Returns the total quantity dropped.
func DropAllItemsOfType(
	delegates GroundDropDelegates,
	itemType string,
	reason string,
	removeStacks func(string) []ItemStack,
) int {
	if !delegates.valid() || itemType == "" || removeStacks == nil {
		return 0
	}

	stacks := removeStacks(itemType)
	return dropStacks(
		delegates,
		stacks,
		reason,
	)
}

// DropAllInventory drains the actor's inventory and equipment, placing the collected
// stacks on the ground. Returns the total quantity dropped.
func DropAllInventory(
	delegates GroundDropDelegates,
	reason string,
	drainInventory func() []ItemStack,
	drainEquipment func() []ItemStack,
) int {
	if !delegates.valid() {
		return 0
	}

	var stacks []ItemStack
	if drainInventory != nil {
		stacks = append(stacks, drainInventory()...)
	}
	if drainEquipment != nil {
		stacks = append(stacks, drainEquipment()...)
	}

	return dropStacks(
		delegates,
		stacks,
		reason,
	)
}

func dropStacks(
	delegates GroundDropDelegates,
	stacks []ItemStack,
	reason string,
) int {
	if !delegates.valid() {
		return 0
	}
	if len(stacks) == 0 {
		return 0
	}

	total := 0
	for _, stack := range stacks {
		if stack.Type == "" || stack.Quantity <= 0 {
			continue
		}
		UpsertGroundItem(
			delegates.items,
			delegates.itemsByTile,
			delegates.nextID,
			delegates.actor,
			stack,
			reason,
			delegates.cfg,
			delegates.angleFn,
			delegates.distanceFn,
			delegates.ensureKey,
			delegates.setQuantity,
			delegates.setPosition,
			delegates.logDrop,
		)
		total += stack.Quantity
	}
	return total
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
