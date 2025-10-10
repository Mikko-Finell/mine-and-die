package stats

import "testing"

func TestComponentLayerOrder(t *testing.T) {
	base := ValueSet{}
	base[StatMight] = 10
	comp := NewComponent(base)

	permanent := NewStatDelta()
	permanent.Add[StatMight] = 5
	comp.Apply(CommandStatChange{
		Layer:  LayerPermanent,
		Source: SourceKey{Kind: SourceKindProgression, ID: "training"},
		Delta:  permanent,
	})

	equipment := NewStatDelta()
	equipment.Add[StatMight] = 5
	equipment.Mul[StatMight] = 1.1
	comp.Apply(CommandStatChange{
		Layer:  LayerEquipment,
		Source: SourceKey{Kind: SourceKindEquipment, ID: "sword"},
		Delta:  equipment,
	})

	temp := NewStatDelta()
	temp.Override[StatMight] = OverrideValue{Active: true, Value: 30}
	comp.Apply(CommandStatChange{
		Layer:         LayerTemporary,
		Source:        SourceKey{Kind: SourceKindTemporary, ID: "buff"},
		Delta:         temp,
		ExpiresAtTick: 5,
	})

	comp.Resolve(1)

	if got := comp.GetTotal(StatMight); got != 30 {
		t.Fatalf("expected might total 30, got %.2f", got)
	}
	if got := comp.GetDerived(DerivedMaxHealth); got != 150 {
		t.Fatalf("expected max health 150, got %.2f", got)
	}

	comp.Resolve(6)
	if got := comp.GetTotal(StatMight); got == 30 {
		t.Fatalf("expected temporary override to expire; still have %.2f", got)
	}
}

func TestDerivedScaling(t *testing.T) {
	comp := DefaultComponent(ArchetypePlayer)
	if got := comp.GetDerived(DerivedMaxHealth); mathAbsDiff(got, 100) > 1e-6 {
		t.Fatalf("expected default player max health 100, got %.2f", got)
	}
	if got := comp.GetDerived(DerivedMaxMana); mathAbsDiff(got, 101) > 1e-6 {
		t.Fatalf("expected default player max mana 101, got %.2f", got)
	}

	boost := NewStatDelta()
	boost.Add[StatResonance] = 10
	comp.Apply(CommandStatChange{
		Layer:  LayerPermanent,
		Source: SourceKey{Kind: SourceKindProgression, ID: "focus-crystal"},
		Delta:  boost,
	})

	comp.Resolve(2)
	expectedMana := computeMaxMana(26)
	if got := comp.GetDerived(DerivedMaxMana); mathAbsDiff(got, expectedMana) > 1e-6 {
		t.Fatalf("expected mana %.2f, got %.2f", expectedMana, got)
	}
}

func TestDeterministicRecomputation(t *testing.T) {
	base := DefaultBase(ArchetypeGoblin)
	compA := NewComponent(base)
	compB := NewComponent(base)

	perm := NewStatDelta()
	perm.Add[StatMight] = 3
	equip := NewStatDelta()
	equip.Mul[StatMight] = 1.25

	compA.Apply(CommandStatChange{Layer: LayerPermanent, Source: SourceKey{Kind: SourceKindProgression, ID: "milestone"}, Delta: perm})
	compA.Apply(CommandStatChange{Layer: LayerEquipment, Source: SourceKey{Kind: SourceKindEquipment, ID: "axe"}, Delta: equip})

	compB.Apply(CommandStatChange{Layer: LayerEquipment, Source: SourceKey{Kind: SourceKindEquipment, ID: "axe"}, Delta: equip})
	compB.Apply(CommandStatChange{Layer: LayerPermanent, Source: SourceKey{Kind: SourceKindProgression, ID: "milestone"}, Delta: perm})

	compA.Resolve(10)
	compB.Resolve(10)

	for i := StatID(0); i < StatCount; i++ {
		if mathAbsDiff(compA.GetTotal(i), compB.GetTotal(i)) > 1e-6 {
			t.Fatalf("totals diverged for stat %d: %.4f vs %.4f", i, compA.GetTotal(i), compB.GetTotal(i))
		}
	}
	for i := DerivedID(0); i < DerivedCount; i++ {
		if mathAbsDiff(compA.GetDerived(i), compB.GetDerived(i)) > 1e-6 {
			t.Fatalf("derived diverged for stat %d: %.4f vs %.4f", i, compA.GetDerived(i), compB.GetDerived(i))
		}
	}
}

func mathAbsDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
