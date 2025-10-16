package contract

// BuiltInDefinitions materialises the gameplay definitions associated with the
// built-in effect contracts. Callers receive a fresh map and struct instances so
// they can customise behaviour without mutating the package-level templates.
func BuiltInDefinitions() map[string]*EffectDefinition {
	return map[string]*EffectDefinition{
		EffectIDAttack: {
			TypeID:        EffectIDAttack,
			Delivery:      DeliveryKindArea,
			Shape:         GeometryShapeRect,
			Motion:        MotionKindInstant,
			Impact:        ImpactPolicyAllInPath,
			LifetimeTicks: 1,
			Hooks: EffectHooks{
				OnSpawn: HookMeleeSpawn,
			},
			Client: ReplicationSpec{
				SendSpawn:       true,
				SendUpdates:     true,
				SendEnd:         true,
				ManagedByClient: true,
			},
			End: EndPolicy{Kind: EndInstant},
		},
		EffectIDFireball: {
			TypeID:        EffectIDFireball,
			Delivery:      DeliveryKindArea,
			Shape:         GeometryShapeCircle,
			Motion:        MotionKindLinear,
			Impact:        ImpactPolicyFirstHit,
			LifetimeTicks: 45,
			Hooks: EffectHooks{
				OnSpawn: HookProjectileLifecycle,
				OnTick:  HookProjectileLifecycle,
			},
			Client: ReplicationSpec{
				SendSpawn:   true,
				SendUpdates: true,
				SendEnd:     true,
			},
			End: EndPolicy{Kind: EndDuration},
		},
		EffectIDBurningTick: {
			TypeID:        EffectIDBurningTick,
			Delivery:      DeliveryKindTarget,
			Shape:         GeometryShapeRect,
			Motion:        MotionKindInstant,
			Impact:        ImpactPolicyFirstHit,
			LifetimeTicks: 1,
			Hooks: EffectHooks{
				OnSpawn: HookStatusBurningDamage,
			},
			Client: ReplicationSpec{
				SendSpawn:   true,
				SendUpdates: false,
				SendEnd:     true,
			},
			End: EndPolicy{Kind: EndInstant},
		},
		EffectIDBurningVisual: {
			TypeID:        EffectIDBurningVisual,
			Delivery:      DeliveryKindTarget,
			Shape:         GeometryShapeRect,
			Motion:        MotionKindFollow,
			Impact:        ImpactPolicyFirstHit,
			LifetimeTicks: 45,
			Hooks: EffectHooks{
				OnSpawn: HookStatusBurningVisual,
				OnTick:  HookStatusBurningVisual,
			},
			Client: ReplicationSpec{
				SendSpawn:   true,
				SendUpdates: true,
				SendEnd:     true,
			},
			End: EndPolicy{Kind: EndDuration},
		},
		EffectIDBloodSplatter: {
			TypeID:        EffectIDBloodSplatter,
			Delivery:      DeliveryKindVisual,
			Shape:         GeometryShapeRect,
			Motion:        MotionKindNone,
			Impact:        ImpactPolicyNone,
			LifetimeTicks: 18,
			Hooks: EffectHooks{
				OnSpawn: HookVisualBloodSplatter,
				OnTick:  HookVisualBloodSplatter,
			},
			Client: ReplicationSpec{
				SendSpawn:       true,
				SendUpdates:     false,
				SendEnd:         true,
				ManagedByClient: true,
			},
			End: EndPolicy{Kind: EndDuration},
		},
	}
}
