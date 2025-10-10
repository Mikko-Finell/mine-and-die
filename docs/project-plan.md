# Mine & Die — Product Management Plan (V5)

_Last updated: 2025-10-10_

---

## 1. Executive Summary

*Mine & Die* is a real-time PvP mining prototype built on an authoritative Go backend that simulates the world at ~15 Hz and streams state snapshots to a lightweight JavaScript client over WebSockets.【F:README.md†L3-L54】【F:docs/server.md†L3-L52】【F:docs/client.md†L3-41】

The foundation is strong: deterministic simulation, functional combat and mining loops, and integrated diagnostics. However, marketed features such as the **gold economy**, **safe-zone trading**, and **faction governance** remain aspirational.  
Our short-term mission is to **stabilize the core loop**, **align messaging with current reality**, and **lay the groundwork for scalable economy and governance systems**.

---

## 2. Current Product Snapshot

| Layer | State Summary | Key References |
|-------|----------------|----------------|
| **Server** | Hub architecture manages sessions, ticks world state, and enforces deterministic updates via write barriers. Handles movement, combat, hazards, loot, and diagnostics endpoints. | 【F:docs/server.md†L3-114】 |
| **Client** | Modular ES-module stack for input, rendering, networking, and telemetry. Integrates `js-effects` for visuals; diagnostics overlay assists QA. | 【F:docs/client.md†L3-73】 |
| **Gameplay** | Players mine ore, fight with melee and fireballs, face hazards, and drop inventories on death. PvP and resource risk loops are functional but shallow. | 【F:README.md†L43-71】 |
| **Tooling & Tests** | Go test coverage on hubs; Vitest suite for client modules. HTTP endpoints and integration flows remain untested. | 【F:docs/testing.md†L3-24】【F:technical_debt.md†L14-16】 |
| **Docs & Messaging** | Documentation oversells planned systems (economy, factions), creating mismatch between shipped state and roadmap narrative. | 【F:README.md†L95-129】【F:technical_debt.md†L19-23】 |

---

## 3. Key Risks

| Risk | Impact | Mitigation |
|------|---------|-------------|
| **Operational instability** – no backpressure on command queue; broadcast fan-out spawns uncontrolled goroutines. | Tick lag, server churn under load. | Implement per-player queue limits; centralize broadcaster; add latency telemetry.【F:technical_debt.md†L5-15】 |
| **Unsafe resets** – `/world/reset` wipes state without confirmation. | Irreversible data loss during testing. | Add confirmation UI + role gating; restrict to debug mode.【F:technical_debt.md†L8-10】【F:technical_debt.md†L22-23】 |
| **Testing blind spots** – HTTP handlers untested. | Regression risk in core endpoints. | Build `httptest.Server` coverage for `/join`, `/ws`, `/reset`. |
| **Expectation mismatch** – roadmap claims not yet built. | Stakeholder confusion, player distrust. | Publish accurate status update; keep roadmap separate from shipped features.【F:technical_debt.md†L19-23】 |

---

## 4. Strategic Objectives (Next Two Quarters)

1. **Stabilize Core Gameplay Loop (P0)**  
   Deliver a reliable mining/combat experience suitable for internal and external playtests.

2. **Establish the Gold Economy (P1)**  
   Implement finite deposits, depletion/respawn cadence, and inventory/tax routing to complete the extraction fantasy.【F:README.md†L99-108】

3. **Enable Safe Trade & Progression (P1)**  
   Introduce safe zones, escrowed market trades, and UI feedback supporting gatherer vs. fighter diversity.【F:README.md†L109-118】

4. **Introduce Faction Governance (P2)**  
   Implement rank hierarchy, tax flow, and succession to anchor long-term player identity.【F:README.md†L120-128】

5. **Improve Player Lifecycle (P0/P1 overlay)**  
   Add onboarding, in-client diagnostics, and telemetry for retention and economy health tracking.【F:docs/server.md†L103-114】

---

## 5. Execution Framework

| Workstream | Engineering Scope | Design / UX Scope | Product Goals |
|-------------|-------------------|-------------------|----------------|
| **Reliability Hardening** | Command queue limits, broadcast refactor, HTTP test suite, reset safeguards. | Confirmation UX, diagnostics copy polish. | Server stability under 20 concurrent players; zero unintended resets. |
| **Gold Economy** | Deposit lifecycle (spawn → deplete → respawn), tax pipeline integration. | HUD cues for deposit states and scarcity. | Measure loop engagement (contested deposits/hour). |
| **Safe Zones & Market** | PvP suppression, escrowed trades, new endpoints. | Safe-zone boundaries, market UI. | Diversify player motivations; validate market retention. |
| **Faction Governance** | Data model for ranks/taxation, persistence hooks. | Hierarchy management UI, coup notifications. | Establish social competition loop. |
| **Player Lifecycle** | Gated lobbies, telemetry ingestion, analytics dashboard. | Tutorial sandbox, accessibility review. | Reduce churn and inform feature iteration. |

---

## 6. Milestone Roadmap

| Milestone | Target | Deliverables |
|------------|---------|--------------|
| **A – Stability Release** | Month 1-2 | Queue limits, broadcast centralization, reset safeguards, full HTTP coverage, doc alignment. |
| **B – Gold Economy Loop** | Month 3-4 | Deposit system, mining→inventory flow, economy telemetry MVP. |
| **C – Safe Trade / Onboarding** | Month 5-6 | Safe-zone framework, market UI prototype, improved tutorial experience. |
| **D – Faction Governance Alpha** | Month 7+ | Persistence layer, management UI, taxation pipeline integration. |

---

## 7. Metrics for Success

| Category | KPI | Target |
|-----------|-----|--------|
| **Stability** | Tick latency variance | ≤ 5 % under 20 simulated players |
| **Reliability** | Unintended resets | 0 occurrences across playtests |
| **Accuracy** | Docs vs. reality | 100 % alignment post-update |
| **Engagement** | Avg. deposits contested/hr | +30 % after Gold Loop release |
| **Readiness** | Roadmap spec approvals | Economy + Safe Zone specs approved before Milestone C |

---

## 8. Cross-Functional Coordination

- **Engineering:** Twice-weekly syncs on reliability and telemetry progress; maintain shared Grafana-style dashboard for tick and queue metrics.  
- **Design / UX:** Co-own onboarding, reset confirmation flows, and diagnostic overlays.  
- **QA:** Build regression checklist and automated smoke tests; schedule playtest runs after Milestone A completion.  
- **Comms / Marketing:** Publish “Prototype Status” article updating feature claims while preserving aspirational roadmap context.【F:technical_debt.md†L19-23】  
- **Product:** Maintain milestone tracking board, integrate telemetry KPIs into sprint reviews, and capture learnings in `docs/gameplay-design/`.

---

## 9. Immediate Actions (Next 2 Weeks)

1. **Start reliability hardening** — implement queue limits and broadcast loop refactor (owners: Server / Infra).  
2. **Finalize README refresh** — align docs and marketing copy with current capabilities (owners: PM + Comms).  
3. **Design reset confirmation UX** — deliver wireframes for review (owner: Design).  
4. **Draft economy data contracts** — deposits, taxes, and safe-zone placeholders for early client integration (owner: Engineering).  
5. **Define telemetry schema** — events for heartbeat, gold flow, and faction promotions (owner: PM + Backend).  

---

## 10. Long-Term Vision

The end-state for *Mine & Die* is a **player-driven extraction world** where risk, economy, and governance intertwine: miners fuel markets, markets fund factions, and factions contest territory for lasting influence.  
This plan focuses on the infrastructural groundwork that enables that emergent future—stable simulation, truthful messaging, and data-driven iteration.

---
