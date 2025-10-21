package effects

import (
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestSpawnContractBloodDecalFromInstance(t *testing.T) {
	now := time.UnixMilli(1_650)
	tileSize := 40.0
	width := 36.0
	height := 44.0
	centerX := 120.0
	centerY := 200.0
	lifetimeTicks := 45
	tickRate := 15

	instance := &effectcontract.EffectInstance{
		ID:           "effect-blood",
		DefinitionID: "custom-blood",
		OwnerActorID: "npc-goblin",
		StartTick:    32,
		DeliveryState: effectcontract.EffectDeliveryState{
			Geometry: effectcontract.EffectGeometry{
				Width:  QuantizeWorldCoord(width, tileSize),
				Height: QuantizeWorldCoord(height, tileSize),
			},
		},
		BehaviorState: effectcontract.EffectBehaviorState{
			TicksRemaining: lifetimeTicks,
			Extra: map[string]int{
				"centerX": QuantizeWorldCoord(centerX, tileSize),
				"centerY": QuantizeWorldCoord(centerY, tileSize),
			},
		},
	}

	params := map[string]float64{"drag": 0.9, "speed": 3}
	colors := []string{"#111", "#222"}

	effect := SpawnContractBloodDecalFromInstance(BloodDecalSpawnConfig{
		Instance:        instance,
		Now:             now,
		TileSize:        tileSize,
		TickRate:        tickRate,
		DefaultSize:     2 * 20,
		DefaultDuration: 1200 * time.Millisecond,
		Params:          params,
		Colors:          colors,
	})

	if effect == nil {
		t.Fatal("expected blood decal to spawn")
	}
	if effect.ID != instance.ID {
		t.Fatalf("unexpected effect id: got %q want %q", effect.ID, instance.ID)
	}
	if effect.Type != instance.DefinitionID {
		t.Fatalf("unexpected effect type: got %q want %q", effect.Type, instance.DefinitionID)
	}
	if effect.Owner != instance.OwnerActorID {
		t.Fatalf("unexpected owner: got %q want %q", effect.Owner, instance.OwnerActorID)
	}
	if effect.Start != now.UnixMilli() {
		t.Fatalf("unexpected start: got %d want %d", effect.Start, now.UnixMilli())
	}
	expectedWidth := DequantizeWorldCoord(QuantizeWorldCoord(width, tileSize), tileSize)
	expectedHeight := DequantizeWorldCoord(QuantizeWorldCoord(height, tileSize), tileSize)
	expectedLifetime := TicksToDuration(lifetimeTicks, tickRate)
	if effect.Duration != expectedLifetime.Milliseconds() {
		t.Fatalf("unexpected duration: got %d want %d", effect.Duration, expectedLifetime.Milliseconds())
	}
	if effect.Width != expectedWidth {
		t.Fatalf("unexpected width: got %f want %f", effect.Width, expectedWidth)
	}
	if effect.Height != expectedHeight {
		t.Fatalf("unexpected height: got %f want %f", effect.Height, expectedHeight)
	}
	if effect.X != centerX-expectedWidth/2 {
		t.Fatalf("unexpected x: got %f want %f", effect.X, centerX-expectedWidth/2)
	}
	if effect.Y != centerY-expectedHeight/2 {
		t.Fatalf("unexpected y: got %f want %f", effect.Y, centerY-expectedHeight/2)
	}
	if effect.ExpiresAt != now.Add(expectedLifetime) {
		t.Fatalf("unexpected expiry: got %v want %v", effect.ExpiresAt, now.Add(expectedLifetime))
	}
	if !effect.ContractManaged {
		t.Fatalf("expected contract managed flag to be true")
	}
	if effect.TelemetrySpawnTick != instance.StartTick {
		t.Fatalf("unexpected spawn tick: got %d want %d", effect.TelemetrySpawnTick, instance.StartTick)
	}
	if effect.Params["drag"] != params["drag"] {
		t.Fatalf("unexpected params drag: got %f want %f", effect.Params["drag"], params["drag"])
	}
	if effect.Params["speed"] != params["speed"] {
		t.Fatalf("unexpected params speed: got %f want %f", effect.Params["speed"], params["speed"])
	}
	if len(effect.Params) != len(params) {
		t.Fatalf("unexpected param count: got %d want %d", len(effect.Params), len(params))
	}
	if len(effect.Colors) != len(colors) {
		t.Fatalf("unexpected color count: got %d want %d", len(effect.Colors), len(colors))
	}
	for i := range colors {
		if effect.Colors[i] != colors[i] {
			t.Fatalf("unexpected color at %d: got %q want %q", i, effect.Colors[i], colors[i])
		}
	}

	params["drag"] = 1.5
	colors[0] = "#fff"
	if effect.Params["drag"] == params["drag"] {
		t.Fatal("expected params to be cloned")
	}
	if effect.Colors[0] == colors[0] {
		t.Fatal("expected colors to be cloned")
	}
}

func TestSpawnContractBloodDecalFromInstanceDefaults(t *testing.T) {
	now := time.UnixMilli(99)
	tileSize := 40.0
	instance := &effectcontract.EffectInstance{
		ID: "missing-geometry",
		BehaviorState: effectcontract.EffectBehaviorState{
			Extra: map[string]int{
				"centerX": QuantizeWorldCoord(40, tileSize),
				"centerY": QuantizeWorldCoord(80, tileSize),
			},
		},
	}

	effect := SpawnContractBloodDecalFromInstance(BloodDecalSpawnConfig{
		Instance:        instance,
		Now:             now,
		TileSize:        tileSize,
		TickRate:        0,
		DefaultSize:     30,
		DefaultDuration: 250 * time.Millisecond,
	})
	if effect == nil {
		t.Fatal("expected blood decal to spawn with defaults")
	}
	if effect.Type != effectcontract.EffectIDBloodSplatter {
		t.Fatalf("expected default effect type, got %q", effect.Type)
	}
	if effect.Width != 30 {
		t.Fatalf("expected default width, got %f", effect.Width)
	}
	if effect.Height != 30 {
		t.Fatalf("expected default height, got %f", effect.Height)
	}
	expectedDuration := (250 * time.Millisecond).Milliseconds()
	if effect.Duration != expectedDuration {
		t.Fatalf("unexpected duration: got %d want %d", effect.Duration, expectedDuration)
	}
	if effect.ExpiresAt != now.Add(250*time.Millisecond) {
		t.Fatalf("unexpected expiry: got %v want %v", effect.ExpiresAt, now.Add(250*time.Millisecond))
	}
}

func TestSpawnContractBloodDecalFromInstanceMissingCenter(t *testing.T) {
	instance := &effectcontract.EffectInstance{}
	effect := SpawnContractBloodDecalFromInstance(BloodDecalSpawnConfig{
		Instance: instance,
	})
	if effect != nil {
		t.Fatal("expected nil effect when center coordinates missing")
	}
}

func TestSpawnContractBloodDecalFromInstanceMinimumDuration(t *testing.T) {
	now := time.UnixMilli(7)
	instance := &effectcontract.EffectInstance{
		BehaviorState: effectcontract.EffectBehaviorState{
			Extra: map[string]int{
				"centerX": 0,
				"centerY": 0,
			},
		},
	}

	effect := SpawnContractBloodDecalFromInstance(BloodDecalSpawnConfig{
		Instance:        instance,
		Now:             now,
		TileSize:        40,
		TickRate:        0,
		DefaultSize:     10,
		DefaultDuration: 0,
	})
	if effect == nil {
		t.Fatal("expected effect when defaults require minimum duration")
	}
	if effect.Duration != time.Millisecond.Milliseconds() {
		t.Fatalf("expected minimum duration of 1ms, got %d", effect.Duration)
	}
	if effect.ExpiresAt != now.Add(time.Millisecond) {
		t.Fatalf("unexpected expiresAt: got %v want %v", effect.ExpiresAt, now.Add(time.Millisecond))
	}
}

func TestSyncContractBloodDecalInstance(t *testing.T) {
	tileSize := 40.0
	instance := &effectcontract.EffectInstance{
		BehaviorState: effectcontract.EffectBehaviorState{
			Extra: map[string]int{"other": 7},
		},
		Params: map[string]int{"speed": 3},
	}
	effect := &State{
		X:      120,
		Y:      200,
		Width:  36,
		Height: 44,
	}
	colors := []string{"#111", "#222"}

	SyncContractBloodDecalInstance(BloodDecalSyncConfig{
		Instance:    instance,
		Effect:      effect,
		TileSize:    tileSize,
		DefaultSize: 2 * 20,
		Colors:      colors,
	})

	expectedWidth := QuantizeWorldCoord(effect.Width, tileSize)
	expectedHeight := QuantizeWorldCoord(effect.Height, tileSize)
	if instance.DeliveryState.Geometry.Width != expectedWidth {
		t.Fatalf("unexpected geometry width: got %d want %d", instance.DeliveryState.Geometry.Width, expectedWidth)
	}
	if instance.DeliveryState.Geometry.Height != expectedHeight {
		t.Fatalf("unexpected geometry height: got %d want %d", instance.DeliveryState.Geometry.Height, expectedHeight)
	}
	expectedCenterX := QuantizeWorldCoord(effect.X+effect.Width/2, tileSize)
	expectedCenterY := QuantizeWorldCoord(effect.Y+effect.Height/2, tileSize)
	if instance.BehaviorState.Extra["centerX"] != expectedCenterX {
		t.Fatalf("unexpected centerX extra: got %d want %d", instance.BehaviorState.Extra["centerX"], expectedCenterX)
	}
	if instance.BehaviorState.Extra["centerY"] != expectedCenterY {
		t.Fatalf("unexpected centerY extra: got %d want %d", instance.BehaviorState.Extra["centerY"], expectedCenterY)
	}
	if instance.Params["centerX"] != expectedCenterX {
		t.Fatalf("unexpected params centerX: got %d want %d", instance.Params["centerX"], expectedCenterX)
	}
	if instance.Params["centerY"] != expectedCenterY {
		t.Fatalf("unexpected params centerY: got %d want %d", instance.Params["centerY"], expectedCenterY)
	}
	if instance.BehaviorState.Extra["other"] != 7 {
		t.Fatalf("expected existing extra key to be preserved, got %d", instance.BehaviorState.Extra["other"])
	}
	if instance.Params["speed"] != 3 {
		t.Fatalf("expected existing param key to be preserved, got %d", instance.Params["speed"])
	}
	if len(instance.Colors) != len(colors) {
		t.Fatalf("unexpected colors length: got %d want %d", len(instance.Colors), len(colors))
	}
	colors[0] = "#999"
	if instance.Colors[0] == colors[0] {
		t.Fatal("expected colors to be cloned")
	}
}

func TestSyncContractBloodDecalInstanceDefaults(t *testing.T) {
	instance := &effectcontract.EffectInstance{}
	effect := &State{X: 10, Y: -4}
	colors := []string{"#abc"}

	SyncContractBloodDecalInstance(BloodDecalSyncConfig{
		Instance:    instance,
		Effect:      effect,
		TileSize:    0,
		DefaultSize: 30,
		Colors:      colors,
	})

	expectedWidth := QuantizeWorldCoord(30, 0)
	expectedHeight := QuantizeWorldCoord(30, 0)
	if instance.DeliveryState.Geometry.Width != expectedWidth {
		t.Fatalf("expected default width quantized to %d, got %d", expectedWidth, instance.DeliveryState.Geometry.Width)
	}
	if instance.DeliveryState.Geometry.Height != expectedHeight {
		t.Fatalf("expected default height quantized to %d, got %d", expectedHeight, instance.DeliveryState.Geometry.Height)
	}
	expectedCenterX := QuantizeWorldCoord(effect.X, 0)
	expectedCenterY := QuantizeWorldCoord(effect.Y, 0)
	if instance.BehaviorState.Extra["centerX"] != expectedCenterX {
		t.Fatalf("expected centerX %d, got %d", expectedCenterX, instance.BehaviorState.Extra["centerX"])
	}
	if instance.BehaviorState.Extra["centerY"] != expectedCenterY {
		t.Fatalf("expected centerY %d, got %d", expectedCenterY, instance.BehaviorState.Extra["centerY"])
	}
	if instance.Params["centerX"] != expectedCenterX {
		t.Fatalf("expected params centerX %d, got %d", expectedCenterX, instance.Params["centerX"])
	}
	if instance.Params["centerY"] != expectedCenterY {
		t.Fatalf("expected params centerY %d, got %d", expectedCenterY, instance.Params["centerY"])
	}
	if len(instance.Colors) != len(colors) {
		t.Fatalf("expected colors length %d, got %d", len(colors), len(instance.Colors))
	}
	if instance.BehaviorState.Extra == nil {
		t.Fatal("expected behavior extra map to be initialized")
	}
	if instance.Params == nil {
		t.Fatal("expected params map to be initialized")
	}
	colors[0] = "#def"
	if instance.Colors[0] == colors[0] {
		t.Fatal("expected colors slice to be cloned")
	}
}
