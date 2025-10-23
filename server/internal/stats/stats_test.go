package stats

import (
	"testing"

	serverstats "mine-and-die/server/stats"
)

func TestResolveInvokesSyncForActors(t *testing.T) {
	component := serverstats.DefaultComponent(serverstats.ArchetypeGoblin)
	component.Resolve(0)

	var invoked int
	Resolve(1, []Actor{
		{
			Component: &component,
			SyncMaxHealth: func(max float64) {
				invoked++
				if max <= 0 {
					t.Fatalf("expected positive max health, got %f", max)
				}
			},
		},
	})

	if invoked != 1 {
		t.Fatalf("expected sync callback to run once, got %d", invoked)
	}
}

func TestResolveSkipsNilComponents(t *testing.T) {
	Resolve(1, []Actor{
		{
			Component: nil,
			SyncMaxHealth: func(float64) {
				t.Fatalf("expected nil component to be skipped")
			},
		},
	})
}

func TestSyncMaxHealthIgnoresMissingCallback(t *testing.T) {
	component := serverstats.DefaultComponent(serverstats.ArchetypeGoblin)
	component.Resolve(0)
	SyncMaxHealth(&component, nil)
}
