package world

import (
	"errors"
	"slices"
	"testing"
)

type testInventory struct {
	slots []int
}

func cloneTestInventory(inv testInventory) testInventory {
	return testInventory{slots: slices.Clone(inv.slots)}
}

func equalTestInventory(a, b testInventory) bool {
	return slices.Equal(a.slots, b.slots)
}

func TestMutateActorInventory_EmitsOnChange(t *testing.T) {
	inv := testInventory{slots: []int{1}}
	version := uint64(7)
	var emitted []testInventory

	err := MutateActorInventory(
		&inv,
		&version,
		func(current *testInventory) error {
			current.slots = append(current.slots, 2)
			return nil
		},
		cloneTestInventory,
		equalTestInventory,
		func(updated testInventory) {
			emitted = append(emitted, cloneTestInventory(updated))
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if version != 8 {
		t.Fatalf("expected version 8, got %d", version)
	}
	if !equalTestInventory(inv, testInventory{slots: []int{1, 2}}) {
		t.Fatalf("mutation did not persist: %+v", inv.slots)
	}
	if len(emitted) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(emitted))
	}
	if !equalTestInventory(emitted[0], inv) {
		t.Fatalf("emitted snapshot mismatch: %+v vs %+v", emitted[0].slots, inv.slots)
	}
}

func TestMutateActorInventory_RollsBackOnError(t *testing.T) {
	inv := testInventory{slots: []int{3}}
	version := uint64(11)
	emitted := false

	err := MutateActorInventory(
		&inv,
		&version,
		func(current *testInventory) error {
			current.slots = append(current.slots, 4)
			return errors.New("boom")
		},
		cloneTestInventory,
		equalTestInventory,
		func(testInventory) {
			emitted = true
		},
	)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if version != 11 {
		t.Fatalf("expected version to remain 11, got %d", version)
	}
	if !equalTestInventory(inv, testInventory{slots: []int{3}}) {
		t.Fatalf("expected inventory to roll back, got %+v", inv.slots)
	}
	if emitted {
		t.Fatalf("unexpected emission on error")
	}
}

func TestMutateActorInventory_IgnoresNoop(t *testing.T) {
	inv := testInventory{slots: []int{5, 6}}
	version := uint64(2)
	emitted := false

	err := MutateActorInventory(
		&inv,
		&version,
		func(current *testInventory) error {
			return nil
		},
		cloneTestInventory,
		equalTestInventory,
		func(testInventory) {
			emitted = true
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != 2 {
		t.Fatalf("expected version to remain 2, got %d", version)
	}
	if emitted {
		t.Fatalf("unexpected emission on noop mutation")
	}
}
