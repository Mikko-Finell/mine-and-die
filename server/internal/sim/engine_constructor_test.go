package sim_test

import (
	"testing"

	"mine-and-die/server"
)

func TestEngineConstructorMatchesHubHarness(t *testing.T) {
	hubRecord, engineRecord := server.RunDeterminismHarness(t, server.DeterminismHarnessOptions{})

	if engineRecord != hubRecord {
		t.Fatalf("determinism harness mismatch: legacy=%+v new=%+v", hubRecord, engineRecord)
	}
	if hubRecord.PatchChecksum == "" {
		t.Fatalf("determinism harness returned empty patch checksum")
	}
}
