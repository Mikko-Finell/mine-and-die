package simulation

import (
	"context"

	"mine-and-die/server/logging"
)

const (
	// EventTickBudgetOverrun is emitted when the simulation loop exceeds the allotted tick budget.
	EventTickBudgetOverrun logging.EventType = "simulation.tick_budget_overrun"
	// EventTickBudgetAlarm is emitted when the server schedules recovery due to a severe tick budget breach.
	EventTickBudgetAlarm logging.EventType = "simulation.tick_budget_alarm"
)

// TickBudgetOverrunPayload captures timing details for a tick budget breach.
type TickBudgetOverrunPayload struct {
	DurationMillis int64   `json:"durationMillis"`
	BudgetMillis   int64   `json:"budgetMillis"`
	Ratio          float64 `json:"ratio"`
	Streak         uint64  `json:"streak"`
}

// TickBudgetOverrun publishes a warning when the simulation exceeds the configured tick budget.
func TickBudgetOverrun(ctx context.Context, pub logging.Publisher, tick uint64, payload TickBudgetOverrunPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventTickBudgetOverrun,
		Tick:     tick,
		Severity: logging.SeverityWarn,
		Category: "simulation",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}

// TickBudgetAlarmPayload captures details when the server escalates an overrun into a resynchronisation alarm.
type TickBudgetAlarmPayload struct {
	DurationMillis  int64   `json:"durationMillis"`
	BudgetMillis    int64   `json:"budgetMillis"`
	Ratio           float64 `json:"ratio"`
	Streak          uint64  `json:"streak"`
	ResyncScheduled bool    `json:"resyncScheduled"`
	ThresholdRatio  float64 `json:"thresholdRatio"`
	ThresholdStreak uint64  `json:"thresholdStreak"`
}

// TickBudgetAlarm publishes an error event when the server forces a resync due to sustained tick budget overruns.
func TickBudgetAlarm(ctx context.Context, pub logging.Publisher, tick uint64, payload TickBudgetAlarmPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventTickBudgetAlarm,
		Tick:     tick,
		Severity: logging.SeverityError,
		Category: "simulation",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}
