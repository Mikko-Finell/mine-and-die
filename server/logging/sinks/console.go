package sinks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"mine-and-die/server/logging"
)

type ConsoleSink struct {
	logger *log.Logger
}

func NewConsoleSink(w io.Writer, cfg logging.ConsoleConfig) *ConsoleSink {
	prefix := ""
	flags := log.LstdFlags
	return &ConsoleSink{logger: log.New(w, prefix, flags)}
}

func (s *ConsoleSink) Write(event logging.Event) error {
	if s.logger == nil {
		return nil
	}
	payload := formatPayload(event.Payload)
	targets := formatTargets(event.Targets)
	s.logger.Printf("[%s] tick=%d actor=%s severity=%s%s%s", event.Type, event.Tick, formatEntity(event.Actor), formatSeverity(event.Severity), targets, payload)
	return nil
}

func (s *ConsoleSink) Close(context.Context) error {
	return nil
}

func formatSeverity(sev logging.Severity) string {
	switch sev {
	case logging.SeverityDebug:
		return "debug"
	case logging.SeverityInfo:
		return "info"
	case logging.SeverityWarn:
		return "warn"
	case logging.SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

func formatEntity(ref logging.EntityRef) string {
	if ref.ID == "" {
		return string(ref.Kind)
	}
	if ref.Kind == "" {
		return ref.ID
	}
	return fmt.Sprintf("%s:%s", ref.Kind, ref.ID)
}

func formatTargets(targets []logging.EntityRef) string {
	if len(targets) == 0 {
		return ""
	}
	parts := make([]string, 0, len(targets))
	for _, target := range targets {
		parts = append(parts, formatEntity(target))
	}
	return fmt.Sprintf(" targets=%s", strings.Join(parts, ","))
}

func formatPayload(payload any) string {
	if payload == nil {
		return ""
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(" payload=%v", payload)
	}
	return fmt.Sprintf(" payload=%s", data)
}
