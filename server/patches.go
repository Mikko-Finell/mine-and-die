package server

import (
	"os"
	"strconv"
	"time"

	"mine-and-die/server/internal/journal"
)

const (
	defaultJournalKeyframeCapacity = 8
	defaultJournalKeyframeMaxAge   = 5 * time.Second
)

const (
	envJournalCapacity = "KEYFRAME_JOURNAL_CAPACITY"
	envJournalMaxAgeMS = "KEYFRAME_JOURNAL_MAX_AGE_MS"
)

type PatchKind = journal.PatchKind

const (
	PatchPlayerPos       = journal.PatchPlayerPos
	PatchPlayerFacing    = journal.PatchPlayerFacing
	PatchPlayerIntent    = journal.PatchPlayerIntent
	PatchPlayerHealth    = journal.PatchPlayerHealth
	PatchPlayerInventory = journal.PatchPlayerInventory
	PatchPlayerEquipment = journal.PatchPlayerEquipment
	PatchPlayerRemoved   = journal.PatchPlayerRemoved

	PatchNPCPos       = journal.PatchNPCPos
	PatchNPCFacing    = journal.PatchNPCFacing
	PatchNPCHealth    = journal.PatchNPCHealth
	PatchNPCInventory = journal.PatchNPCInventory
	PatchNPCEquipment = journal.PatchNPCEquipment

	PatchEffectPos    = journal.PatchEffectPos
	PatchEffectParams = journal.PatchEffectParams

	PatchGroundItemPos = journal.PatchGroundItemPos
	PatchGroundItemQty = journal.PatchGroundItemQty
)

type Patch = journal.Patch

type PositionPayload = journal.PositionPayload

type PlayerPosPayload = journal.PlayerPosPayload

type NPCPosPayload = journal.NPCPosPayload

type EffectPosPayload = journal.EffectPosPayload

type GroundItemPosPayload = journal.GroundItemPosPayload

type FacingPayload = journal.FacingPayload

type PlayerFacingPayload = journal.PlayerFacingPayload

type NPCFacingPayload = journal.NPCFacingPayload

type PlayerIntentPayload = journal.PlayerIntentPayload

type HealthPayload = journal.HealthPayload

type PlayerHealthPayload = journal.PlayerHealthPayload

type NPCHealthPayload = journal.NPCHealthPayload

type InventoryPayload = journal.InventoryPayload

type PlayerInventoryPayload = journal.PlayerInventoryPayload

type NPCInventoryPayload = journal.NPCInventoryPayload

type EquipmentPayload = journal.EquipmentPayload

type PlayerEquipmentPayload = journal.PlayerEquipmentPayload

type NPCEquipmentPayload = journal.NPCEquipmentPayload

type EffectParamsPayload = journal.EffectParamsPayload

type GroundItemQtyPayload = journal.GroundItemQtyPayload

type EffectEventBatch = journal.EffectEventBatch

type Journal = journal.Journal

type keyframe = journal.Keyframe

type journalEviction = journal.KeyframeEviction

type keyframeRecordResult = journal.KeyframeRecordResult

type resyncPolicy = journal.Policy

type resyncSignal = journal.ResyncSignal

type resyncReason = journal.ResyncReason

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
