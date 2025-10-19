package main

import "testing"

func TestFacingConversionRoundTrip(t *testing.T) {
	cases := []FacingDirection{FacingUp, FacingDown, FacingLeft, FacingRight}
	for _, legacy := range cases {
		t.Run(string(legacy), func(t *testing.T) {
			simValue := toSimFacing(legacy)
			if simValue == "" {
				t.Fatalf("expected non-empty sim value for %q", legacy)
			}
			roundTrip := legacyFacingFromSim(simValue)
			if roundTrip != legacy {
				t.Fatalf("expected round trip to return %q, got %q", legacy, roundTrip)
			}
		})
	}

	if toSimFacing("invalid") != "" {
		t.Fatalf("expected invalid facing to map to empty sim value")
	}
	if legacyFacingFromSim("invalid") != "" {
		t.Fatalf("expected invalid sim facing to map to empty legacy value")
	}
}

func TestCommandTypeConversionRoundTrip(t *testing.T) {
	cases := []CommandType{CommandMove, CommandAction, CommandHeartbeat, CommandSetPath, CommandClearPath}
	for _, legacy := range cases {
		t.Run(string(legacy), func(t *testing.T) {
			simValue := toSimCommandType(legacy)
			if simValue == "" {
				t.Fatalf("expected non-empty sim value for %q", legacy)
			}
			roundTrip := legacyCommandTypeFromSim(simValue)
			if roundTrip != legacy {
				t.Fatalf("expected round trip to return %q, got %q", legacy, roundTrip)
			}
		})
	}

	if toSimCommandType("invalid") != "" {
		t.Fatalf("expected invalid command type to map to empty sim value")
	}
	if legacyCommandTypeFromSim("invalid") != "" {
		t.Fatalf("expected invalid sim command type to map to empty legacy value")
	}
}
