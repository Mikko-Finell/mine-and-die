package world

import (
	"errors"
	"slices"
	"testing"
)

type testEquipment struct {
	slots []int
}

func cloneTestEquipment(eq testEquipment) testEquipment {
	return testEquipment{slots: slices.Clone(eq.slots)}
}

func equalTestEquipment(a, b testEquipment) bool {
	return slices.Equal(a.slots, b.slots)
}

func TestMutateActorEquipment_EmitsOnChange(t *testing.T) {
	eq := testEquipment{slots: []int{1}}
	version := uint64(3)
	var emitted []testEquipment

	err := MutateActorEquipment(
		&eq,
		&version,
		func(current *testEquipment) error {
			current.slots = append(current.slots, 2)
			return nil
		},
		cloneTestEquipment,
		equalTestEquipment,
		func(updated testEquipment) {
			emitted = append(emitted, cloneTestEquipment(updated))
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if version != 4 {
		t.Fatalf("expected version 4, got %d", version)
	}
	if !equalTestEquipment(eq, testEquipment{slots: []int{1, 2}}) {
		t.Fatalf("mutation did not persist: %+v", eq.slots)
	}
	if len(emitted) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(emitted))
	}
	if !equalTestEquipment(emitted[0], eq) {
		t.Fatalf("emitted snapshot mismatch: %+v vs %+v", emitted[0].slots, eq.slots)
	}
}

func TestMutateActorEquipment_RollsBackOnError(t *testing.T) {
	eq := testEquipment{slots: []int{5}}
	version := uint64(9)
	emitted := false

	err := MutateActorEquipment(
		&eq,
		&version,
		func(current *testEquipment) error {
			current.slots = append(current.slots, 6)
			return errors.New("boom")
		},
		cloneTestEquipment,
		equalTestEquipment,
		func(testEquipment) {
			emitted = true
		},
	)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if version != 9 {
		t.Fatalf("expected version to remain 9, got %d", version)
	}
	if !equalTestEquipment(eq, testEquipment{slots: []int{5}}) {
		t.Fatalf("expected equipment to roll back, got %+v", eq.slots)
	}
	if emitted {
		t.Fatalf("unexpected emission on error")
	}
}

func TestMutateActorEquipment_IgnoresNoop(t *testing.T) {
	eq := testEquipment{slots: []int{7, 8}}
	version := uint64(1)
	emitted := false

	err := MutateActorEquipment(
		&eq,
		&version,
		func(current *testEquipment) error {
			return nil
		},
		cloneTestEquipment,
		equalTestEquipment,
		func(testEquipment) {
			emitted = true
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected version to remain 1, got %d", version)
	}
	if emitted {
		t.Fatalf("unexpected emission on noop mutation")
	}
}
