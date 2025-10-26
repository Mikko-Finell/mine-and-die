package server

import (
	"context"
	"time"

	effectcatalog "mine-and-die/server/effects/catalog"
	effectcontract "mine-and-die/server/effects/contract"
	combat "mine-and-die/server/internal/combat"
	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
	abilitiespkg "mine-and-die/server/internal/world/abilities"
	loggingeconomy "mine-and-die/server/logging/economy"
)

type EffectManager struct {
	core  *internaleffects.Manager
	world *World
}

func newEffectManager(world *World) *EffectManager {
	definitions := effectcontract.BuiltInDefinitions()
	var resolver *effectcatalog.Resolver
	if r, err := effectcatalog.Load(effectcontract.BuiltInRegistry, effectcatalog.DefaultPaths()...); err == nil {
		if loaded := r.DefinitionsByContractID(); len(loaded) > 0 {
			for id, def := range loaded {
				if _, exists := definitions[id]; exists {
					continue
				}
				definitions[id] = def
			}
		}
		resolver = r
	}

	var registryProvider func() internaleffects.Registry
	if world != nil {
		registryProvider = world.effectRegistry
	}

	hooks := defaultEffectHookRegistry(world)
	manager := internaleffects.NewManager(internaleffects.ManagerConfig{
		Definitions: definitions,
		Catalog:     resolver,
		Hooks:       hooks,
		OwnerMissing: func(actorID string) bool {
			if actorID == "" || world == nil {
				return false
			}
			if _, ok := world.players[actorID]; ok {
				return false
			}
			if _, ok := world.npcs[actorID]; ok {
				return false
			}
			return true
		},
		Registry: registryProvider,
	})

	return &EffectManager{core: manager, world: world}
}

func (m *EffectManager) Definitions() map[string]*effectcontract.EffectDefinition {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.Definitions()
}

func (m *EffectManager) Hooks() map[string]internaleffects.HookSet {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.Hooks()
}

func (m *EffectManager) Instances() map[string]*effectcontract.EffectInstance {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.Instances()
}

func (m *EffectManager) Catalog() *effectcatalog.Resolver {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.Catalog()
}

func (m *EffectManager) TotalEnqueued() int {
	if m == nil || m.core == nil {
		return 0
	}
	return m.core.TotalEnqueued()
}

func (m *EffectManager) TotalDrained() int {
	if m == nil || m.core == nil {
		return 0
	}
	return m.core.TotalDrained()
}

func (m *EffectManager) LastTickProcessed() effectcontract.Tick {
	if m == nil || m.core == nil {
		return 0
	}
	return m.core.LastTickProcessed()
}

func (m *EffectManager) PendingIntentCount() int {
	if m == nil || m.core == nil {
		return 0
	}
	return m.core.PendingIntentCount()
}

func (m *EffectManager) WorldEffect(id string) *effectState {
	if m == nil || m.core == nil {
		return nil
	}
	return internaleffects.LoadRuntimeEffect(m.core, id)
}

func (m *EffectManager) PendingIntents() []effectcontract.EffectIntent {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.PendingIntents()
}

func (m *EffectManager) ResetPendingIntents() {
	if m == nil || m.core == nil {
		return
	}
	m.core.ResetPendingIntents()
}

func (m *EffectManager) EnqueueIntent(intent effectcontract.EffectIntent) {
	if m == nil || m.core == nil {
		return
	}
	m.core.EnqueueIntent(intent)
}

func (m *EffectManager) RunTick(tick effectcontract.Tick, now time.Time, emit func(effectcontract.EffectLifecycleEvent)) {
	if m == nil || m.core == nil {
		return
	}
	m.core.RunTick(tick, now, emit)
}

type projectileOwnerAdapter struct {
	x      float64
	y      float64
	facing string
}

func (a projectileOwnerAdapter) Facing() string {
	if a.facing == "" {
		return string(defaultFacing)
	}
	return a.facing
}

func (a projectileOwnerAdapter) FacingVector() (float64, float64) {
	return facingToVector(FacingDirection(a.Facing()))
}

func (a projectileOwnerAdapter) Position() (float64, float64) {
	return a.x, a.y
}

func defaultEffectHookRegistry(world *World) map[string]internaleffects.HookSet {
	hooks := make(map[string]internaleffects.HookSet)
	var ownerLookup abilitiespkg.AbilityOwnerLookup[*actorState, combat.AbilityActor]
	var stateLookup abilitiespkg.AbilityOwnerStateLookup[*actorState]
	if world != nil {
		world.configureAbilityOwnerAdapters()
		ownerLookup = world.abilityOwnerLookup
		stateLookup = world.abilityOwnerStateLookup
	}
	hooks[effectcontract.HookMeleeSpawn] = internaleffects.MeleeSpawnHook(internaleffects.MeleeSpawnHookConfig{
		TileSize:        tileSize,
		DefaultWidth:    meleeAttackWidth,
		DefaultReach:    meleeAttackReach,
		DefaultDamage:   meleeAttackDamage,
		DefaultDuration: meleeAttackDuration,
		LookupOwner: func(actorID string) *internaleffects.MeleeOwner {
			if actorID == "" {
				return nil
			}
			if ownerLookup == nil {
				return nil
			}
			owner, _, ok := ownerLookup(actorID)
			if !ok || owner == nil {
				return nil
			}
			var reference any
			if stateLookup != nil {
				if state, _, ok := stateLookup(actorID); ok && state != nil {
					reference = state
				}
			}
			return &internaleffects.MeleeOwner{
				X:         owner.X,
				Y:         owner.Y,
				Reference: reference,
			}
		},
		ResolveImpact: func(effect *internaleffects.State, owner *internaleffects.MeleeOwner, actorID string, tick effectcontract.Tick, now time.Time, area internaleffects.MeleeImpactArea) {
			if world == nil || effect == nil {
				return
			}

			var ownerRef any
			if owner != nil {
				ownerRef = owner.Reference
			}

			tick64 := uint64(tick)

			cfg := worldpkg.ResolveMeleeImpactConfig{
				EffectType: effect.Type,
				Effect:     effect,
				Owner:      ownerRef,
				ActorID:    actorID,
				Tick:       tick64,
				Now:        now,
				Area: worldpkg.Obstacle{
					X:      area.X,
					Y:      area.Y,
					Width:  area.Width,
					Height: area.Height,
				},
				Obstacles: world.obstacles,
				ForEachPlayer: func(visit func(id string, x, y float64, reference any)) {
					for id, player := range world.players {
						if player == nil {
							continue
						}
						visit(id, player.X, player.Y, player)
					}
				},
				ForEachNPC: func(visit func(id string, x, y float64, reference any)) {
					for id, npc := range world.npcs {
						if npc == nil {
							continue
						}
						visit(id, npc.X, npc.Y, npc)
					}
				},
				GivePlayerGold: func(id string) (bool, error) {
					if _, ok := world.players[id]; !ok {
						return false, nil
					}
					err := world.MutateInventory(id, func(inv *Inventory) error {
						if inv == nil {
							return nil
						}
						_, addErr := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1})
						return addErr
					})
					return true, err
				},
				GiveNPCGold: func(id string) (bool, error) {
					if _, ok := world.npcs[id]; !ok {
						return false, nil
					}
					err := world.MutateNPCInventory(id, func(inv *Inventory) error {
						if inv == nil {
							return nil
						}
						_, addErr := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1})
						return addErr
					})
					return true, err
				},
				GiveOwnerGold: func(ref any) error {
					actor, ok := ref.(*actorState)
					if !ok || actor == nil {
						return nil
					}
					_, err := actor.Inventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1})
					return err
				},
				ApplyPlayerHit: func(effectRef any, target any, now time.Time) {
					if world == nil || world.playerHitCallback == nil {
						return
					}
					world.playerHitCallback(effectRef, target, now)
				},
				ApplyNPCHit: func(effectRef any, target any, now time.Time) {
					if world == nil || world.npcHitCallback == nil {
						return
					}
					world.npcHitCallback(effectRef, target, now)
				},
				RecordGoldGrantFailure: func(actorID string, obstacleID string, err error) {
					if err == nil {
						return
					}
					loggingeconomy.ItemGrantFailed(
						context.Background(),
						world.publisher,
						tick64,
						world.entityRef(actorID),
						loggingeconomy.ItemGrantFailedPayload{ItemType: string(ItemTypeGold), Quantity: 1, Reason: "mine_gold"},
						map[string]any{"error": err.Error(), "obstacle": obstacleID},
					)
				},
				RecordAttackOverlap: func(actorID string, tick uint64, effectType string, hitPlayers []string, hitNPCs []string) {
					if world == nil || world.recordAttackOverlap == nil {
						return
					}
					world.recordAttackOverlap(actorID, tick, effectType, hitPlayers, hitNPCs, nil)
				},
			}

			worldpkg.ResolveMeleeImpact(cfg)
		},
	})
	hooks[effectcontract.HookProjectileLifecycle] = internaleffects.ContractProjectileLifecycleHook(internaleffects.ContractProjectileLifecycleHookConfig{
		TileSize: tileSize,
		TickRate: tickRate,
		LookupTemplate: func(definitionID string) *internaleffects.ProjectileTemplate {
			if world == nil {
				return nil
			}
			return world.projectileTemplates[definitionID]
		},
		LookupOwner: func(actorID string) internaleffects.ProjectileOwner {
			if actorID == "" {
				return nil
			}
			if ownerLookup == nil {
				return nil
			}
			owner, _, ok := ownerLookup(actorID)
			if !ok || owner == nil {
				return nil
			}
			return projectileOwnerAdapter{x: owner.X, y: owner.Y, facing: owner.Facing}
		},
		PruneExpired: func(at time.Time) {
			if world == nil {
				return
			}
			world.pruneEffects(at)
		},
		RecordEffectSpawn: func(effectType, category string) {
			if world == nil {
				return
			}
			world.recordEffectSpawn(effectType, category)
		},
		AdvanceProjectile: func(effect *internaleffects.State, now time.Time, dt float64) bool {
			if world == nil {
				return false
			}
			return world.advanceProjectile(effect, now, dt)
		},
	})
	lookupContractActor := func(actorID string) *internaleffects.ContractStatusActor {
		if world == nil || actorID == "" {
			return nil
		}
		actor := world.actorByID(actorID)
		if actor == nil {
			return nil
		}
		contractActor := &internaleffects.ContractStatusActor{
			ID: actor.ID,
			X:  actor.X,
			Y:  actor.Y,
			ApplyBurningDamage: func(ownerID string, status internaleffects.StatusEffectType, delta float64, now time.Time) {
				world.applyBurningDamage(ownerID, actor, StatusEffectType(status), delta, now)
			},
		}
		if actor.StatusEffects != nil {
			if inst := actor.StatusEffects[StatusEffectBurning]; inst != nil {
				contractActor.StatusInstance = &internaleffects.ContractStatusInstance{
					Instance:  inst,
					ExpiresAt: func() time.Time { return inst.ExpiresAt },
				}
			}
		}
		return contractActor
	}
	hooks[effectcontract.HookStatusBurningVisual] = internaleffects.ContractBurningVisualHook(internaleffects.ContractBurningVisualHookConfig{
		StatusEffect:     internaleffects.StatusEffectType(StatusEffectBurning),
		DefaultLifetime:  burningStatusEffectDuration,
		FallbackLifetime: burningTickInterval,
		TileSize:         tileSize,
		DefaultFootprint: playerHalf * 2,
		TickRate:         tickRate,
		LookupActor:      lookupContractActor,
		ExtendLifetime: func(fields worldpkg.StatusEffectLifetimeFields, expiresAt time.Time) {
			worldpkg.ExtendStatusEffectLifetime(fields, expiresAt)
		},
		ExpireLifetime: func(fields worldpkg.StatusEffectLifetimeFields, now time.Time) {
			worldpkg.ExpireStatusEffectLifetime(fields, now)
		},
		RecordEffectSpawn: func(effectType, category string) {
			if world == nil {
				return
			}
			world.recordEffectSpawn(effectType, category)
		},
	})
	hooks[effectcontract.HookStatusBurningDamage] = internaleffects.ContractBurningDamageHook(internaleffects.ContractBurningDamageHookConfig{
		StatusEffect:    internaleffects.StatusEffectType(StatusEffectBurning),
		DamagePerSecond: lavaDamagePerSecond,
		TickInterval:    burningTickInterval,
		LookupActor:     lookupContractActor,
	})
	ensureBloodDecal := func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, _ effectcontract.Tick, now time.Time) {
		internaleffects.EnsureBloodDecalInstance(internaleffects.BloodDecalInstanceConfig{
			Runtime:         rt,
			Instance:        instance,
			Now:             now,
			TileSize:        tileSize,
			TickRate:        tickRate,
			DefaultSize:     playerHalf * 2,
			DefaultDuration: bloodSplatterDuration,
			Params:          newBloodSplatterParams(),
			Colors:          bloodSplatterColors(),
			PruneExpired: func(at time.Time) {
				if world == nil {
					return
				}
				world.pruneEffects(at)
			},
			RecordSpawn: func(effectType, producer string) {
				if world == nil {
					return
				}
				world.recordEffectSpawn(effectType, producer)
			},
		})
	}
	hooks[effectcontract.HookVisualBloodSplatter] = internaleffects.HookSet{
		OnSpawn: ensureBloodDecal,
		OnTick:  ensureBloodDecal,
	}
	return hooks
}
