package server

import (
	"os"
	"strconv"
	"time"

	"mine-and-die/server/internal/journal"
	simpatches "mine-and-die/server/internal/sim/patches/typed"
)

const (
	defaultJournalKeyframeCapacity = 8
	defaultJournalKeyframeMaxAge   = 5 * time.Second
)

const (
	envJournalCapacity = "KEYFRAME_JOURNAL_CAPACITY"
	envJournalMaxAgeMS = "KEYFRAME_JOURNAL_MAX_AGE_MS"
)

type PatchKind = simpatches.PatchKind

const (
	PatchPlayerPos       = simpatches.PatchPlayerPos
	PatchPlayerFacing    = simpatches.PatchPlayerFacing
	PatchPlayerIntent    = simpatches.PatchPlayerIntent
	PatchPlayerHealth    = simpatches.PatchPlayerHealth
	PatchPlayerInventory = simpatches.PatchPlayerInventory
	PatchPlayerEquipment = simpatches.PatchPlayerEquipment
	PatchPlayerRemoved   = simpatches.PatchPlayerRemoved

	PatchNPCPos       = simpatches.PatchNPCPos
	PatchNPCFacing    = simpatches.PatchNPCFacing
	PatchNPCHealth    = simpatches.PatchNPCHealth
	PatchNPCInventory = simpatches.PatchNPCInventory
	PatchNPCEquipment = simpatches.PatchNPCEquipment

	PatchEffectPos    = simpatches.PatchEffectPos
	PatchEffectParams = simpatches.PatchEffectParams

	PatchGroundItemPos = simpatches.PatchGroundItemPos
	PatchGroundItemQty = simpatches.PatchGroundItemQty
)

type Patch = simpatches.Patch

type PositionPayload = simpatches.PositionPayload

type PlayerPosPayload = simpatches.PlayerPosPayload

type NPCPosPayload = simpatches.NPCPosPayload

type EffectPosPayload = simpatches.EffectPosPayload

type GroundItemPosPayload = simpatches.GroundItemPosPayload

type FacingPayload = simpatches.FacingPayload

type PlayerFacingPayload = simpatches.PlayerFacingPayload

type NPCFacingPayload = simpatches.NPCFacingPayload

type PlayerIntentPayload = simpatches.PlayerIntentPayload

type HealthPayload = simpatches.HealthPayload

type PlayerHealthPayload = simpatches.PlayerHealthPayload

type NPCHealthPayload = simpatches.NPCHealthPayload

type InventoryPayload = simpatches.InventoryPayload

type PlayerInventoryPayload = simpatches.PlayerInventoryPayload

type NPCInventoryPayload = simpatches.NPCInventoryPayload

type EquipmentPayload = simpatches.EquipmentPayload

type PlayerEquipmentPayload = simpatches.PlayerEquipmentPayload

type NPCEquipmentPayload = simpatches.NPCEquipmentPayload

type EffectParamsPayload = simpatches.EffectParamsPayload

type GroundItemQtyPayload = simpatches.GroundItemQtyPayload

type EffectEventBatch = simpatches.EffectEventBatch

type Journal = journal.Journal

type keyframe = journal.Keyframe

type journalEviction = journal.KeyframeEviction

type keyframeRecordResult = journal.KeyframeRecordResult

type resyncPolicy = journal.Policy

type resyncSignal = simpatches.EffectResyncSignal

type resyncReason = simpatches.EffectResyncReason

const (
	metricJournalNonMonotonicSeq = "journal_nonmonotonic_seq"
	metricJournalUnknownIDUpdate = "journal_unknown_id_update"
	metricJournalUpdateAfterEnd  = "journal_update_after_end"
)

func newJournal(keyframeCapacity int, maxAge time.Duration) Journal {
	return journal.New(keyframeCapacity, maxAge)
}

// journalConfig loads retention settings from the environment falling back to
// defaults when unset or invalid.
func journalConfig() (int, time.Duration) {
	capacity := defaultJournalKeyframeCapacity
	if raw := os.Getenv(envJournalCapacity); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			capacity = parsed
		}
	}

	maxAge := defaultJournalKeyframeMaxAge
	if raw := os.Getenv(envJournalMaxAgeMS); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			maxAge = time.Duration(parsed) * time.Millisecond
		}
	}

	if capacity < 0 {
		capacity = 0
	}
	if maxAge < 0 {
		maxAge = 0
	}

	return capacity, maxAge
}
